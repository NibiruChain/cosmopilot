package nodeutils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

const (
	UpgradeCompleted = "completed"
	UpgradeOnGoing   = "ongoing"
)

type UpgradeChecker struct {
	configFile string
	config     UpgradesConfig
}

type UpgradesConfig struct {
	Upgrades []Upgrade `json:"upgrades"`
}

type Upgrade struct {
	Height int64  `json:"height"`
	Status string `json:"status"`
}

func NewUpgradeChecker(configFile string) (*UpgradeChecker, error) {
	if _, err := os.Stat(configFile); err != nil {
		return nil, fmt.Errorf("configuration file does not exist: %v", err)
	}
	uc := &UpgradeChecker{
		configFile: configFile,
	}
	return uc, uc.loadConfig()
}

func (u *UpgradeChecker) WatchConfigFile() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(filepath.Dir(u.configFile)); err != nil {
		return err
	}
	for {
		select {
		case _, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("could not retrieve event")
			}
			log.Info("reloading upgrades config file")
			if err := u.loadConfig(); err != nil {
				return err
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("could not retrieve error")
			}
			return err
		}
	}
}

func (u *UpgradeChecker) loadConfig() error {
	f, err := os.Open(u.configFile)
	if err != nil {
		return err
	}
	defer f.Close()

	body, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &u.config)
}

func (u *UpgradeChecker) ShouldUpgrade(height int64) bool {
	for _, upgrade := range u.config.Upgrades {
		if upgrade.Height == height &&
			upgrade.Status != UpgradeCompleted &&
			upgrade.Status != UpgradeOnGoing {
			return true
		}
	}
	return false
}
