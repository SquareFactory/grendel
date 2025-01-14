// Copyright 2019 Grendel Authors. All rights reserved.
//
// This file is part of Grendel.
//
// Grendel is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Grendel is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Grendel. If not, see <https://www.gnu.org/licenses/>.

package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ubccr/grendel/cmd"
	"github.com/ubccr/grendel/cmd/watch"
	"github.com/ubccr/grendel/model"
	"gopkg.in/tomb.v2"
)

var (
	DB            model.DataStore
	hostsFile     string
	imagesFile    string
	listenAddress string
	serveCmd      = &cobra.Command{
		Use:   "serve",
		Short: "Run services",
		Long:  `Run grendel services`,
		RunE: func(command *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(command.Context())

			// Trap cleanup
			cleanChan := make(chan os.Signal, 1)
			signal.Notify(cleanChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-cleanChan
				cancel()
			}()

			errChan := make(chan error)

			go watch.WatchConfig(ctx, []string{hostsFile, imagesFile}, func() error {
				if err := loadHostJSON(); err != nil {
					return err
				}
				if err := loadImageJSON(); err != nil {
					return err
				}
				return nil
			}, errChan)

			return watch.ConfigReloader(ctx, errChan, runServices)
		},
	}
)

func init() {
	serveCmd.PersistentFlags().String("dbpath", ":memory:", "path to database file")
	viper.BindPFlag("dbpath", serveCmd.PersistentFlags().Lookup("dbpath"))
	serveCmd.PersistentFlags().StringVar(&hostsFile, "hosts", "", "path to hosts file")
	serveCmd.MarkPersistentFlagRequired("hosts")
	serveCmd.PersistentFlags().StringVar(&imagesFile, "images", "", "path to boot images file")
	serveCmd.MarkPersistentFlagRequired("images")
	serveCmd.PersistentFlags().StringSlice("services", []string{}, "enabled services")
	serveCmd.PersistentFlags().StringVar(&listenAddress, "listen", "", "listen address")
	viper.BindPFlag("services", serveCmd.PersistentFlags().Lookup("services"))

	serveCmd.PersistentPreRunE = func(command *cobra.Command, args []string) error {
		err := cmd.SetupLogging()
		if err != nil {
			return err
		}

		DB, err = model.NewDataStore(viper.GetString("dbpath"))
		if err != nil {
			return err
		}

		cmd.Log.Infof("Using database path: %s", viper.GetString("dbpath"))

		return nil
	}

	serveCmd.PersistentPostRunE = func(command *cobra.Command, args []string) error {
		if DB != nil {
			cmd.Log.Info("Closing Database")
			err := DB.Close()
			if err != nil {
				return err
			}
		}

		return nil
	}

	cmd.Root.AddCommand(serveCmd)
}

func loadHostJSON() error {
	file, err := os.Open(hostsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonBlob, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	var hostList model.HostList
	err = json.Unmarshal(jsonBlob, &hostList)
	if err != nil {
		return err
	}

	err = DB.StoreHosts(hostList)
	if err != nil {
		return err
	}

	cmd.Log.WithField("hosts", string(jsonBlob)).Infof("Successfully loaded hosts")
	return nil
}

func loadImageJSON() error {
	file, err := os.Open(imagesFile)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonBlob, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	var imageList model.BootImageList
	err = json.Unmarshal(jsonBlob, &imageList)
	if err != nil {
		return err
	}

	err = DB.StoreBootImages(imageList)
	if err != nil {
		return err
	}

	cmd.Log.WithField("images", string(jsonBlob)).Infof("Successfully loaded boot images")
	return nil
}

func runServices(ctx context.Context) error {
	doneChan := make(chan error)
	t := NewInterruptTomb()
	t.Go(func() error {
		t.Go(func() error { return serveTFTP(t) })
		t.Go(func() error { return serveDNS(t) })
		t.Go(func() error { return serveDHCP(t) })
		t.Go(func() error { return servePXE(t) })
		t.Go(func() error { return serveAPI(t) })
		t.Go(func() error { return serveProvision(t) })
		return nil
	})
	go func() {
		doneChan <- t.Wait()
	}()
	for {
		select {
		case err := <-doneChan:
			return err
		case <-ctx.Done():
			t.Kill(ctx.Err())
			cmd.Log.Infof("cancelling runServices")
			return <-doneChan
		}
	}
}

func NewInterruptTomb() *tomb.Tomb {
	t := &tomb.Tomb{}
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		select {
		case <-t.Dying():
		case <-sigint:
			cmd.Log.Debug("Caught interrupt signal")
			t.Kill(nil)
		}
	}()

	return t
}

func GetListenAddress(address string) (string, error) {
	if listenAddress == "" {
		return address, nil
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s", listenAddress, port), nil
}

func NewInterruptContext() (context.Context, context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		oscall := <-c
		cmd.Log.Debugf("Signal interrupt system call: %+v", oscall)
		cancel()
	}()

	return ctx, cancel
}
