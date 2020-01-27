package serve

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ubccr/grendel/cmd"
	"github.com/ubccr/grendel/model"
)

var (
	DB         model.DataStore
	hostsFile  string
	imagesFile string
	serveCmd   = &cobra.Command{
		Use:   "serve",
		Short: "Run services",
		Long:  `Run grendel services`,
		RunE: func(command *cobra.Command, args []string) error {
			if hostsFile != "" {
				err := loadHostJSON()
				if err != nil {
					return err
				}
			}
			if imagesFile != "" {
				err := loadImageJSON()
				if err != nil {
					return err
				}
			}

			return runServices()
		},
	}
)

func init() {
	serveCmd.PersistentFlags().String("dbpath", ":memory:", "path to database file")
	viper.BindPFlag("dbpath", serveCmd.PersistentFlags().Lookup("dbpath"))
	serveCmd.PersistentFlags().StringVar(&hostsFile, "hosts", "", "path to hosts file")
	serveCmd.PersistentFlags().StringVar(&imagesFile, "images", "", "path to boot images file")
	serveCmd.PersistentFlags().StringSlice("services", []string{}, "enabled services")
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

	cmd.Log.Infof("Successfully loaded %d hosts", len(hostList))
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

	cmd.Log.Infof("Successfully loaded %d boot images", len(imageList))
	return nil
}

func runServices() error {
	ctx, cancel := NewInterruptContext()

	var wg sync.WaitGroup
	wg.Add(6)
	errs := make(chan error, 6)

	go func() {
		errs <- serveAPI(ctx)
		wg.Done()
	}()
	go func() {
		errs <- serveTFTP(ctx)
		wg.Done()
	}()
	go func() {
		errs <- serveDNS(ctx)
		wg.Done()
	}()
	go func() {
		errs <- serveProvision(ctx)
		wg.Done()
	}()
	go func() {
		errs <- serveDHCP(ctx)
		wg.Done()
	}()
	go func() {
		errs <- servePXE(ctx)
		wg.Done()
	}()

	// Fail if any servers error out
	err := <-errs

	cmd.Log.Infof("Waiting for all services to shutdown...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		cmd.Log.Info("All services shutdown")
	case <-ctxShutdown.Done():
		cmd.Log.Warning("Timeout reached")
	}

	return err
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
