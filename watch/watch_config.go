package watch

import (
	"encoding/json"
	"os"
)

type Config struct {
	host  grendelHost  `json:"grendelHost"`
	image grendelImage `json:"grendelImage"`
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
	if err := json.NewDecoder(hostsFile).Decode(&config.host); err != nil {
		return nil, err
	}

	if err := json.NewDecoder(imagesFile).Decode(&config.image); err != nil {
		return nil, err
	}

	return config, err
}

/* func watchConfig(ctx context.Context, configChan chan<- *Config) {

	var lastModTime time.Time

	// Initial load
	func() {
		stat, err := os.Stat(os.Getenv("HOSTS_FILE"))
		if err != nil {
			cmd.Log.Infof("failed to stat %s : %w ", filename, err)
			return
		}
		lastModTime = stat.ModTime()

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

	if err = watcher.Add(os.Getenv("HOSTS_FILE")); err != nil {
		cmd.Log.Infof("failed to add host file to config reloader")
	}

	if err = watcher.Add(os.Getenv("IMAGES_FILE")); err != nil {
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

			stat, err := os.Stat(filename)
			if err != nil {
				cmd.Log.Infof("failed to stat file %s: %w", filename, err)
			}

			if !stat.ModTime().Equal(lastModTime) {
				lastModTime = stat.ModTime()
				cmd.Log.Infof("new config detected")

				config, err := loadConfig(filename)
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
	// Channel used to assure only one handleConfig can be launched
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
*/
