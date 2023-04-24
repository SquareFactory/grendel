package watch

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ubccr/grendel/cmd"
)

type Config struct {
	Hosts  []Host
	Images []Image
}

func loadConfig() (*Config, error) {

	hostsFile, err := os.Open(os.Getenv("HOSTS_FILE"))
	if err != nil {
		return nil, err
	}
	defer hostsFile.Close()

	imagesFile, err := os.Open(os.Getenv("IMAGES_FILE"))
	if err != nil {
		return nil, err
	}
	defer imagesFile.Close()

	config := &Config{}
	if err := json.NewDecoder(hostsFile).Decode(&config.Hosts); err != nil {
		return nil, err
	}

	if err := json.NewDecoder(imagesFile).Decode(&config.Images); err != nil {
		return nil, err
	}

	return config, err
}

func WatchConfig(ctx context.Context, configChan chan<- *Config) {

	var lastModTime [2]time.Time
	hostFile := os.Getenv("HOSTS_FILE")
	imageFile := os.Getenv("IMAGES_FILE")

	// Initial load
	func() {
		hostStat, err := os.Stat(hostFile)
		if err != nil {
			cmd.Log.Infof("failed to stat %s : %w ", hostFile, err)
			return
		}
		lastModTime[0] = hostStat.ModTime()

		imageStat, err := os.Stat(imageFile)
		if err != nil {
			cmd.Log.Infof("failed to stat %s : %w ", imageFile, err)
			return
		}
		lastModTime[1] = imageStat.ModTime()

		cmd.Log.Infof("initial config detected")
		config, err := loadConfig()
		if err != nil {
			cmd.Log.Infof("failed to load config : %w", err)
			return
		}

		configChan <- config
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		cmd.Log.Infof("failed to watch config")
	}
	defer watcher.Close()

	if err = watcher.Add(hostFile); err != nil {
		cmd.Log.Infof("failed to add host file to config reloader")
	}

	if err = watcher.Add(imageFile); err != nil {
		cmd.Log.Infof("failed to add image file to config reloader")
	}

	for {

		select {

		case <-ctx.Done():
			return

		case _, ok := <-watcher.Events:
			if !ok {
				return
			}

			hostStat, err := os.Stat(hostFile)
			if err != nil {
				cmd.Log.Infof("failed to stat file %s: %w", hostFile, err)
			}

			imageStat, err := os.Stat(imageFile)
			if err != nil {
				cmd.Log.Infof("failed to stat file %s: %w", hostFile, err)
			}

			if hostStat.ModTime() != lastModTime[0] || imageStat.ModTime() != lastModTime[1] {
				lastModTime[0] = hostStat.ModTime()
				lastModTime[1] = imageStat.ModTime()
				cmd.Log.Infof("new config detected")

				config, err := loadConfig()
				if err != nil {
					cmd.Log.Infof("failed to load config : %w", err)
					continue
				}
				select {
				case configChan <- config:

				case <-ctx.Done():

					return
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			cmd.Log.Infof("config reloader thrown an error : %w", err)
		}
	}
}

func ConfigReloader(ctx context.Context, configChan <-chan *Config, restartGrendel func(ctx context.Context, config *Config)) error {
	var configContext context.Context
	var configCancel context.CancelFunc
	// Channel used to assure only one restartGrendel can be launched
	doneChan := make(chan struct{})

	for {
		select {
		case newConfig := <-configChan:
			if configContext != nil && configCancel != nil {
				configCancel()
				select {
				case <-doneChan:
					cmd.Log.Info("loading new config")
				case <-time.After(30 * time.Second):
					cmd.Log.Fatal("couldn't load a new config because of a deadlock")
				}
			}
			configContext, configCancel = context.WithCancel(ctx)
			go func() {
				cmd.Log.Info("loaded new config")
				restartGrendel(configContext, newConfig)
				doneChan <- struct{}{}
			}()
		case <-ctx.Done():
			if configContext != nil && configCancel != nil {
				configCancel()
				configContext = nil
			}

			// This assure that the `restartGrendel` ends gracefully
			select {
			case <-doneChan:
				cmd.Log.Info("config reloader graceful exit")
			case <-time.After(30 * time.Second):
				cmd.Log.Fatal("config reloader force fatal exit")
			}

			// The context was canceled, exit the loop
			return ctx.Err()
		}
	}
}
