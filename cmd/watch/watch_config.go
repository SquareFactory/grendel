package watch

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ubccr/grendel/cmd"
	"github.com/ubccr/grendel/util/channel"
)

func WatchConfig(ctx context.Context, files []string, configLoader func() error, errChan chan<- error) {
	var lastModTimes = make([]time.Time, 0, len(files))

	// Initial load
	cmd.Log.Info("loading initial config")
	func() {
		for _, file := range files {
			fileStat, err := os.Stat(file)
			if err != nil {
				cmd.Log.Errorf("failed to stat %s: %w", file, err)
				errChan <- err
				return
			}
			lastModTimes = append(lastModTimes, fileStat.ModTime())
		}

		cmd.Log.Infof("initial config detected")
		if err := configLoader(); err != nil {
			cmd.Log.Errorf("failed to load config : %w", err)
			errChan <- err
			return
		}
		errChan <- nil
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		cmd.Log.Panicf("failed to watch config")
	}
	defer watcher.Close()

	for _, file := range files {
		if err = watcher.Add(file); err != nil {
			cmd.Log.Panicf("failed to add host file to config reloader")
		}
	}

	debouncedEvents := channel.Debounce(watcher.Events, time.Second)

	for {
		select {

		case <-ctx.Done():
			return

		case _, ok := <-debouncedEvents:
			if !ok {
				return
			}

			var hasChanged bool
			for i, file := range files {
				fileStat, err := os.Stat(file)
				if err != nil {
					cmd.Log.Errorf("failed to stat %s: %w", file, err)
					errChan <- err
					return
				}

				hasChanged = hasChanged || fileStat.ModTime() != lastModTimes[i]
				lastModTimes[i] = fileStat.ModTime()
			}

			if hasChanged {
				cmd.Log.Infof("new config detected")

				if err := configLoader(); err != nil {
					cmd.Log.Errorf("failed to load config : %w", err)
					continue
				}
				select {
				case errChan <- nil:

				case <-ctx.Done():

					return
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			cmd.Log.Errorf("config reloader thrown an error : %w", err)
		}
	}
}

func ConfigReloader(ctx context.Context, restartChan <-chan error, restartGrendel func(ctx context.Context) error) error {
	var configContext context.Context
	var configCancel context.CancelFunc
	// Channel used to assure only one restartGrendel can be launched
	doneChan := make(chan error)

	for {
		select {
		case err := <-restartChan:
			if err != nil {
				if configContext != nil && configCancel != nil {
					configCancel()
				}
				return err
			}

			if configContext != nil && configCancel != nil {
				configCancel()
				select {
				case err := <-doneChan:
					if err != nil && !errors.Is(err, context.Canceled) {
						return err
					}
					cmd.Log.Info("loading new config")
				case <-time.After(30 * time.Second):
					cmd.Log.Fatal("couldn't load a new config because of a deadlock")
				}
			}
			configContext, configCancel = context.WithCancel(ctx)
			go func() {
				cmd.Log.Info("loaded new config")
				doneChan <- restartGrendel(configContext)
			}()
		case <-ctx.Done():
			if configContext != nil && configCancel != nil {
				configCancel()
				configContext = nil
			}

			// This assure that the `restartGrendel` ends gracefully
			select {
			case err := <-doneChan:
				if err != nil {
					return err
				}
				cmd.Log.Info("config reloader graceful exit")
			case <-time.After(30 * time.Second):
				cmd.Log.Fatal("config reloader force fatal exit")
			}

			// The context was canceled, exit the loop
			return ctx.Err()
		}
	}
}
