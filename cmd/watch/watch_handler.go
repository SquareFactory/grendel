package watch

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/ubccr/grendel/cmd"
)

func restartGrendel(ctx context.Context, config *Config) error {
	// json.marshal new config
	newHostConfig, err := json.Marshal(config.Host)
	if err != nil {
		return err
	}
	newImageConfig, err := json.Marshal(config.Image)
	if err != nil {
		return err
	}

	// writing new configuration to .json files
	if err := os.WriteFile(os.Getenv("HOSTS_FILE"), newHostConfig, 0666); err != nil {
		return err
	}
	if err := os.WriteFile(os.Getenv("IMAGES_FILE"), newImageConfig, 0666); err != nil {
		return err
	}

	// restarting grendel
	command := exec.Command("/app/grendel", "serve", "--debug", "--verbose", "-c", "/secret/grendel.toml", "--hosts", os.Getenv("HOSTS_FILE"), "--images", os.Getenv("IMAGES_FILE"), "--listen", "0.0.0.0")

	if err := command.Start(); err != nil {
		cmd.Log.Errorf("error restarting grendel: %w", err)
		return err
	}

	cmd.Log.Infof("restarting grendel")
	command.Wait()

	return nil
}
