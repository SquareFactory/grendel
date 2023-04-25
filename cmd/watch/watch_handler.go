package watch

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/ubccr/grendel/cmd"
	"github.com/ubccr/grendel/model"
)

var DB model.DataStore

func restartGrendel(ctx context.Context, config *Config) error {
	hostFile := os.Getenv("HOSTS_FILE")
	imageFile := os.Getenv("IMAGES_FILE")

	// load new config
	if err := loadJson(hostFile, imageFile, DB); err != nil {
		return err
	}

	return nil

}

func loadJson(hostFile, imageFile string, DB model.DataStore) error {
	host, err := os.Open(hostFile)
	if err != nil {
		return err
	}
	defer host.Close()

	jsonBlob, err := ioutil.ReadAll(host)
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

	image, err := os.Open(imageFile)
	if err != nil {
		return err
	}
	defer image.Close()

	jsonBlob, err = ioutil.ReadAll(image)
	if err != nil {
		return err
	}

	var imageList model.HostList
	err = json.Unmarshal(jsonBlob, &hostList)
	if err != nil {
		return err
	}

	err = DB.StoreHosts(imageList)
	if err != nil {
		return err
	}

	cmd.Log.Infof("Successfully loaded %d hosts", len(hostList))

	return nil
}
