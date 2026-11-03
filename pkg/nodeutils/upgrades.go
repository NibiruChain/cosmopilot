package nodeutils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

type UpgradeSource string

const (
	UpgradeScheduled = "scheduled"
	UpgradeOnGoing   = "ongoing"
	UpgradeCompleted = "completed"
	UpgradeSkipped   = "skipped"

	ManualUpgrade  UpgradeSource = "manual"
	OnChainUpgrade UpgradeSource = "on-chain"
)

type UpgradeChecker struct {
	configFile string

	mu        sync.RWMutex
	config    UpgradesConfig
	changed   chan struct{}
	watching  chan struct{}
	watchOnce sync.Once
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
		changed:    make(chan struct{}, 1),
		watching:   make(chan struct{}),
	}
	return uc, uc.reload()
}

func (u *UpgradeChecker) Changed() <-chan struct{} {
	return u.changed
}

func (u *UpgradeChecker) WatchConfigFile(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(filepath.Dir(u.configFile)); err != nil {
		return err
	}
	if err := u.reload(); err != nil {
		log.WithError(err).Warn("could not reload upgrades config after starting watcher; keeping last valid version")
	}
	u.watchOnce.Do(func() { close(u.watching) })
	resync := time.NewTicker(time.Second)
	defer resync.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("could not retrieve event")
			}
			if err := u.reload(); err != nil {
				log.WithError(err).Warn("could not reload upgrades config; keeping last valid version")
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("could not retrieve error")
			}
			log.WithError(err).Warn("upgrades config watcher error")
		case <-resync.C:
			_ = u.reload()
		}
	}
}

func (u *UpgradeChecker) reload() error {
	f, err := os.Open(u.configFile)
	if err != nil {
		return err
	}
	defer f.Close()

	body, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	var config UpgradesConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return err
	}

	u.mu.Lock()
	changed := !reflect.DeepEqual(u.config, config)
	u.config = config
	u.mu.Unlock()
	if changed {
		select {
		case u.changed <- struct{}{}:
		default:
		}
	}
	return nil
}

func (u *UpgradeChecker) Snapshot() UpgradesConfig {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return UpgradesConfig{Upgrades: append([]Upgrade(nil), u.config.Upgrades...)}
}

func (u *UpgradeChecker) ShouldUpgrade(height int64) bool {
	_, err := u.GetUpgrade(height)
	return err == nil
}

func (u *UpgradeChecker) GetUpgrade(height int64) (*Upgrade, error) {
	for _, upgrade := range u.Snapshot().Upgrades {
		if height >= upgrade.Height && (upgrade.Status == UpgradeScheduled || upgrade.Status == UpgradeOnGoing) {
			return &upgrade, nil
		}
	}
	return nil, fmt.Errorf("upgrade not found")
}
