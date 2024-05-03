package nodeutils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	upgradeTypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

type UpgradeSource string

const (
	UpgradeScheduled = "scheduled"

	ManualUpgrade  UpgradeSource = "manual"
	OnChainUpgrade UpgradeSource = "on-chain"

	UpgradeInfoFile = "upgrade-info.json"
)

type UpgradeChecker struct {
	configFile string
	config     UpgradesConfig
}

type UpgradesConfig struct {
	Upgrades []Upgrade `json:"upgrades"`
}

type Upgrade struct {
	Height int64         `json:"height"`
	Status string        `json:"status"`
	Image  string        `json:"image"`
	Source UpgradeSource `json:"source"`
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
	_, err := u.GetUpgrade(height)
	return err == nil
}

func (u *UpgradeChecker) GetUpgrade(height int64) (*Upgrade, error) {
	for _, upgrade := range u.config.Upgrades {
		if height >= upgrade.Height && upgrade.Status == UpgradeScheduled {
			return &upgrade, nil
		}
	}
	return nil, fmt.Errorf("upgrade not found")
}

func (u *UpgradeChecker) LoadPlan(upgradeInfoPath string) (*upgradeTypes.Plan, error) {
	file, err := os.Open(upgradeInfoPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var plan upgradeTypes.Plan
	if err = json.Unmarshal(bytes, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func (u *UpgradeChecker) HasUpgradeInfo(height int64, path string) (bool, error) {
	if _, err := os.Stat(filepath.Join(path, UpgradeInfoFile)); err == nil {
		plan, err := u.LoadPlan(filepath.Join(path, UpgradeInfoFile))
		if err != nil {
			return false, err
		}
		return plan.Height == height, nil
	}
	return false, nil
}
