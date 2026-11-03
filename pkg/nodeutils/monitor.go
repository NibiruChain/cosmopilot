package nodeutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	log "github.com/sirupsen/logrus"
)

const (
	abciPollInterval       = time.Second
	abciRequestTimeout     = time.Second
	sdkUpgradeInfoMaxBytes = 1024 * 1024
)

type RequiredUpgrade struct {
	Height int64         `json:"height"`
	Source UpgradeSource `json:"source"`
}

type UpgradeStatus struct {
	LatestHeight    *int64           `json:"latestHeight,omitempty"`
	RequiredUpgrade *RequiredUpgrade `json:"requiredUpgrade"`
}

type TerminationEvidence struct {
	UpgradeStatus
	HaltHeight     int64 `json:"haltHeight,omitempty"`
	ForcedShutdown bool  `json:"forcedShutdown"`
}

type abciInfoClient interface {
	GetAbciInfo(context.Context) (abci.ResponseInfo, error)
}

type upgradeMonitor struct {
	reconcileMu sync.Mutex
	mu          sync.RWMutex

	client          abciInfoClient
	checker         *UpgradeChecker
	upgradeInfoPath string
	stopNode        func() error
	status          UpgradeStatus
	stopSucceeded   bool
}

func newUpgradeMonitor(client abciInfoClient, checker *UpgradeChecker, upgradeInfoPath string, stopNode func() error) *upgradeMonitor {
	return &upgradeMonitor{
		client:          client,
		checker:         checker,
		upgradeInfoPath: upgradeInfoPath,
		stopNode:        stopNode,
	}
}

func (m *upgradeMonitor) Status() UpgradeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := m.status
	if status.LatestHeight != nil {
		height := *status.LatestHeight
		status.LatestHeight = &height
	}
	if status.RequiredUpgrade != nil {
		required := *status.RequiredUpgrade
		status.RequiredUpgrade = &required
	}
	return status
}

func (m *upgradeMonitor) RequiresUpgrade() bool {
	return m.Status().RequiredUpgrade != nil
}

func (m *upgradeMonitor) Run(ctx context.Context) {
	if err := m.Reconcile(ctx); err != nil {
		log.WithError(err).Warn("initial upgrade reconciliation did not complete")
	}
	ticker := time.NewTicker(abciPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-m.checker.Changed():
		}
		if err := m.Reconcile(ctx); err != nil {
			log.WithError(err).Warn("upgrade reconciliation did not complete")
		}
	}
}

func (m *upgradeMonitor) Reconcile(ctx context.Context) error {
	return m.reconcile(ctx, true)
}

func (m *upgradeMonitor) reconcile(ctx context.Context, stopManualUpgrade bool) error {
	m.reconcileMu.Lock()
	defer m.reconcileMu.Unlock()

	freshHeight := m.observeHeight(ctx)
	m.mu.Lock()
	required, shouldStop, err := m.requiredUpgradeLocked(freshHeight)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	changed := !sameRequiredUpgrade(m.status.RequiredUpgrade, required)
	m.status.RequiredUpgrade = required
	if changed || !shouldStop {
		m.stopSucceeded = false
	}
	stopSucceeded := m.stopSucceeded
	m.mu.Unlock()

	if !stopManualUpgrade || required == nil || required.Source != ManualUpgrade || !shouldStop || stopSucceeded {
		return nil
	}
	if err := m.stopNode(); err != nil {
		return fmt.Errorf("stop node for upgrade at height %d: %w", required.Height, err)
	}
	m.mu.Lock()
	m.stopSucceeded = true
	m.mu.Unlock()
	return nil
}

func sameRequiredUpgrade(a, b *RequiredUpgrade) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m *upgradeMonitor) observeHeight(ctx context.Context) bool {
	requestCtx, cancel := context.WithTimeout(ctx, abciRequestTimeout)
	defer cancel()
	info, err := m.client.GetAbciInfo(requestCtx)
	if err != nil {
		return false
	}
	height := info.LastBlockHeight
	m.mu.Lock()
	m.status.LatestHeight = &height
	m.mu.Unlock()
	return true
}

func (m *upgradeMonitor) requiredUpgradeLocked(freshHeight bool) (*RequiredUpgrade, bool, error) {
	config := m.checker.Snapshot()
	if required, shouldStop := m.manualRequiredUpgradeLocked(config); required != nil {
		return required, shouldStop, nil
	}

	info, err := readSDKUpgradeInfo(m.upgradeInfoPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read SDK upgrade info %q: %w", m.upgradeInfoPath, err)
	}
	if !matchesGovernanceUpgrade(config, info.Height) || !m.governanceHeightEligibleLocked(info.Height, freshHeight) {
		return nil, false, nil
	}
	return &RequiredUpgrade{Height: info.Height, Source: OnChainUpgrade}, false, nil
}

func (m *upgradeMonitor) manualRequiredUpgradeLocked(config UpgradesConfig) (*RequiredUpgrade, bool) {
	for _, upgrade := range config.Upgrades {
		if upgrade.Source == ManualUpgrade && upgrade.Status == UpgradeOnGoing && upgrade.Height > 0 {
			return &RequiredUpgrade{Height: upgrade.Height, Source: upgrade.Source}, false
		}
	}
	for _, upgrade := range config.Upgrades {
		if upgrade.Source != ManualUpgrade || upgrade.Status != UpgradeScheduled || upgrade.Height <= 0 {
			continue
		}
		if m.status.LatestHeight != nil && *m.status.LatestHeight >= upgrade.Height-1 {
			return &RequiredUpgrade{Height: upgrade.Height, Source: upgrade.Source}, true
		}
	}
	return nil, false
}

func (m *upgradeMonitor) governanceHeightEligibleLocked(target int64, freshHeight bool) bool {
	if m.status.LatestHeight == nil {
		return true
	}
	height := *m.status.LatestHeight
	if height >= target {
		return false
	}
	return !freshHeight || height >= target-1
}

func matchesGovernanceUpgrade(config UpgradesConfig, height int64) bool {
	for _, upgrade := range config.Upgrades {
		if upgrade.Source == OnChainUpgrade && upgrade.Height == height &&
			(upgrade.Status == UpgradeScheduled || upgrade.Status == UpgradeOnGoing) {
			return true
		}
	}
	return false
}

type sdkUpgradeInfo struct {
	Name   string `json:"name"`
	Height int64  `json:"height"`
}

func readSDKUpgradeInfo(path string) (sdkUpgradeInfo, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return sdkUpgradeInfo{}, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return sdkUpgradeInfo{}, err
	}
	if !fileInfo.Mode().IsRegular() {
		return sdkUpgradeInfo{}, fmt.Errorf("SDK upgrade info is not a regular file")
	}
	if fileInfo.Size() > sdkUpgradeInfoMaxBytes {
		return sdkUpgradeInfo{}, fmt.Errorf("SDK upgrade info is too large")
	}
	body, err := io.ReadAll(io.LimitReader(file, sdkUpgradeInfoMaxBytes+1))
	if err != nil {
		return sdkUpgradeInfo{}, err
	}
	if len(body) > sdkUpgradeInfoMaxBytes {
		return sdkUpgradeInfo{}, fmt.Errorf("SDK upgrade info is too large")
	}
	var upgradeInfo sdkUpgradeInfo
	if err := json.Unmarshal(body, &upgradeInfo); err != nil {
		return sdkUpgradeInfo{}, err
	}
	if upgradeInfo.Height <= 0 || upgradeInfo.Name == "" {
		return sdkUpgradeInfo{}, fmt.Errorf("invalid SDK upgrade info")
	}
	return upgradeInfo, nil
}
