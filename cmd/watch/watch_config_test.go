package watch_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ubccr/grendel/cmd/watch"
)

func TestConfigReloader(t *testing.T) {
	// Create a new parent context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary directory to store the config file
	tempDir, err := os.MkdirTemp("", "test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := watch.Config{
		Hosts: []watch.Host{
			{
				Name:      "tux01",
				Provision: true,
				BootImage: "centOS",
				Interfaces: []struct {
					IP  string `json:"ip"`
					Mac string `json:"mac"`
					Bmc bool   `json:"bmc"`
				}{
					{
						IP:  "192.168.10.12/24",
						Mac: "DE:AD:BE:EF:12:8C",
						Bmc: true,
					},
					{
						IP:  "192.168.10.13/24",
						Mac: "DE:AD:BE:EF:12:8D",
						Bmc: false,
					},
				},
			},
		},
		Images: []watch.Image{
			{
				Name:    "centos",
				Kernel:  "centos.vmlinuz",
				Initrd:  []string{"centos.initramfs"},
				Cmdline: "centos.autologin",
			},
		},
	}

	// Create temporary config files and write data to it
	hostJson, err := json.Marshal(config.Images)
	imageJson, err := json.Marshal(config.Hosts)

	hostFile := filepath.Join(tempDir, "hosts.json")
	os.Setenv("HOSTS_FILE", hostFile)
	err = os.WriteFile(hostFile, hostJson, 0644)
	require.NoError(t, err)

	imageFile := filepath.Join(tempDir, "images.json")
	os.Setenv("IMAGES_FILE", imageFile)
	err = os.WriteFile(imageFile, imageJson, 0644)
	require.NoError(t, err)

	// Create a config channel and start observing the config file
	configChan := make(chan *watch.Config)
	go watch.WatchConfig(ctx, configChan)

	// Create a mock handleConfig function that just sleeps for 1 second
	handleConfigCallCount := 0
	handleConfigCalls := make([]*watch.Config, 2)
	doneChan := make(chan struct{})
	handleConfigMock := func(ctx context.Context, cfg *watch.Config) {
		handleConfigCalls[handleConfigCallCount] = cfg
		handleConfigCallCount++
		select {
		case doneChan <- struct{}{}:
			return
		case <-ctx.Done():
			return
		}
	}

	// Launch the configReloader function in a separate goroutine
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err := watch.ConfigReloader(ctx, configChan, handleConfigMock)
		require.Equal(t, context.Canceled, err)
		wg.Done()
	}()

	// Wait for the handleConfig call to complete
	<-doneChan

	newHost := watch.Host{
		Name:      "web-server-2",
		Provision: true,
		BootImage: "centos-20.04",
		Interfaces: []struct {
			IP  string `json:"ip"`
			Mac string `json:"mac"`
			Bmc bool   `json:"bmc"`
		}{
			{IP: "10.0.0.2", Mac: "00:11:22:33:44:55", Bmc: true},
			{IP: "192.168.1.2", Mac: "66:77:88:99:aa:bb", Bmc: false},
		},
	}
	newJson, err := json.Marshal(newHost)

	// Write a new config file with different data
	time.Sleep(time.Second)
	err = os.WriteFile(hostFile, newJson, 0644)
	require.NoError(t, err)

	// Wait for the second handleConfig call to complete
	<-doneChan

	// Check that handleConfig was called twice with the correct configs
	require.Equal(t, 2, handleConfigCallCount)
	require.Equal(t, 1, len(handleConfigCalls[0].Hosts))
	require.Equal(t, 2, len(handleConfigCalls[1].Hosts))

	// Cancel the parent context to stop the configReloader function
	cancel()

	// Wait for the configReloader function to exit
	wg.Wait()
}
