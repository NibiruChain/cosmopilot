package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jellydator/ttlcache/v3"
	"github.com/mitchellh/hashstructure/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/cosmopilot/api/v1"
	"github.com/NibiruChain/cosmopilot/internal/chainutils"
	"github.com/NibiruChain/cosmopilot/pkg/utils"
)

var (
	configGenerationLocks      = make(map[string]*sync.Mutex)
	configGenerationLocksMutex sync.Mutex
)

func getConfigsLockForAppVersion(version string) *sync.Mutex {
	configGenerationLocksMutex.Lock()
	defer configGenerationLocksMutex.Unlock()

	if lock, exists := configGenerationLocks[version]; exists {
		return lock
	}

	newLock := &sync.Mutex{}
	configGenerationLocks[version] = newLock
	return newLock
}

func (r *Reconciler) ensureConfigMap(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode, nodePodRunning bool) (string, error) {
	logger := log.FromContext(ctx)

	configs, err := r.getGeneratedConfigs(ctx, app, chainNode)
	if err != nil {
		return "", err
	}

	kf := GetKeyFormatter(chainNode)

	// Apply app.toml and config.toml defaults
	configs[appTomlFilename], err = utils.Merge(configs[appTomlFilename], kf.GetBaseAppToml())
	if err != nil {
		return "", err
	}

	configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], kf.GetBaseConfigToml())
	if err != nil {
		return "", err
	}

	// Apply moniker
	configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
		kf.Moniker(): chainNode.GetMoniker(),
	})
	if err != nil {
		return "", err
	}

	// Persist address book file
	if chainNode.Spec.Config.ShouldPersistAddressBook() {
		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
			kf.P2P(): map[string]interface{}{
				kf.AddrBookFile(): defaultAddrBookFile,
			},
		})
		if err != nil {
			return "", err
		}
	}

	// Set external address if there is one
	if addr, ok := getExternalAddress(chainNode); ok {
		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
			kf.P2P(): map[string]interface{}{
				kf.ExternalAddress(): addr,
			},
		})
		if err != nil {
			return "", err
		}
	}

	// Apply state-sync config
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.StateSync.Enabled() {
		configs[appTomlFilename], err = utils.Merge(configs[appTomlFilename], map[string]interface{}{
			kf.StateDashSync(): map[string]interface{}{
				kf.SnapshotInterval():   chainNode.Spec.Config.StateSync.SnapshotInterval,
				kf.SnapshotKeepRecent(): chainNode.Spec.Config.StateSync.GetKeepRecent(),
			},
		})
		if err != nil {
			return "", err
		}
	}

	// Apply seed-mode if enabled
	if chainNode.Spec.Config.SeedModeEnabled() {
		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
			kf.P2P(): map[string]interface{}{
				kf.SeedMode(): true,
			},
		})
		if err != nil {
			return "", err
		}
	}

	// Use genesis from data dir
	if chainNode.Spec.Genesis.ShouldUseDataVolume() {
		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
			kf.GenesisFile(): genesisLocation,
		})
		if err != nil {
			return "", err
		}
	}

	// Apply user specified configs
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.Override != nil {
		for filename, b := range *chainNode.Spec.Config.Override {
			var data map[string]interface{}
			if err := json.Unmarshal(b.Raw, &data); err != nil {
				return "", err
			}
			if _, ok := configs[filename]; ok {
				configs[filename], err = utils.Merge(configs[filename], data)
			} else {
				configs[filename] = data
			}
		}
	}

	// Get hash before adding peer configuration
	hash, err := getConfigHash(configs)
	if err != nil {
		return "", err
	}

	// Apply state-sync restore config if enabled and node is not running. Also ignore this if this node is restoring
	// from a volume snapshot.
	if chainNode.StateSyncRestoreEnabled() && !nodePodRunning && !chainNode.ShouldRestoreFromSnapshot() {
		peers, stateSyncAnnotations, err := r.getChainPeers(ctx, chainNode, AnnotationStateSyncTrustHeight, AnnotationStateSyncTrustHash)
		if err != nil {
			return "", err
		}

		peers, err = r.filterNonReadyPeers(ctx, chainNode, peers)
		if err != nil {
			return "", err
		}

		rpcServers := make([]string, 0)

		switch {
		case len(peers) > 1:
			for _, peer := range peers {
				rpcServers = append(rpcServers, fmt.Sprintf("http://%s:%d", peer.Address, chainutils.RpcPort))
			}

		case len(peers) == 1:
			for i := 0; i < 2; i++ {
				rpcServers = append(rpcServers, fmt.Sprintf("http://%s:%d", peers[0].Address, chainutils.RpcPort))
			}

		default:
			logger.Info("not restoring from state-sync: could not find other peers for this chain")
			r.recorder.Event(chainNode,
				corev1.EventTypeWarning,
				appsv1.ReasonNoPeers,
				"not restoring from state-sync: could not find other peers for this chain",
			)
		}

		if len(rpcServers) >= 2 {
			trustHeight, trustHash := getMostRecentHeightFromServicesAnnotations(stateSyncAnnotations)
			if trustHeight == 0 {
				logger.Info("not restoring from state-sync: no chainnode with valid trust height config is available")
				r.recorder.Event(chainNode,
					corev1.EventTypeWarning,
					appsv1.ReasonNoTrustHeight,
					"not restoring from state-sync: no chainnode with valid trust height config is available",
				)
			} else {
				logger.Info("configuring state-sync",
					"rpc_servers", strings.Join(rpcServers, ","),
					"trust_height", trustHeight,
					"trust_hash", trustHash,
				)
				configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
					kf.StateSync(): map[string]interface{}{
						kf.Enable():      true,
						kf.RPCServers():  strings.Join(rpcServers, ","),
						kf.TrustHeight(): trustHeight,
						kf.TrustHash():   trustHash,
						kf.TrustPeriod(): defaultStateSyncTrustPeriod,
					},
				})
				if err != nil {
					return "", err
				}

				// Set latest height to trust height so that old upgrades are marked as skipped
				chainNode.Status.LatestHeight = trustHeight
			}
		}
	}

	// Apply peer configuration
	peerConfig, err := r.getPeerConfiguration(ctx, chainNode, kf)
	if err != nil {
		return "", err
	}
	configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], peerConfig)
	if err != nil {
		return "", err
	}

	// Encode back to toml
	cmData := make(map[string]string, len(configs))
	for filename, data := range configs {
		cmData[filename], err = utils.TomlEncode(data)
		if err != nil {
			return "", err
		}
	}

	cm := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKeyFromObject(chainNode), cm)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating configs configmap", "configmap", cm.GetName(), "hash", hash)
			cm = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.GetName(),
					Namespace: chainNode.GetNamespace(),
					Labels:    WithChainNodeLabels(chainNode),
					Annotations: map[string]string{
						annotationConfigHash: hash,
					},
				},
				Data: cmData,
			}
			if err = controllerutil.SetControllerReference(chainNode, cm, r.Scheme); err != nil {
				return "", err
			}
			if err = r.Create(ctx, cm); err != nil {
				return "", err
			}
			r.recorder.Eventf(chainNode,
				corev1.EventTypeNormal,
				appsv1.ReasonConfigsCreated,
				"Configuration files successfully created",
			)
			return hash, nil
		}
		return "", err
	}

	var shouldUpdate bool
	for file, data := range cmData {
		if oldData, ok := cm.Data[file]; !ok || data != oldData {
			shouldUpdate = true
			break
		}
	}

	if shouldUpdate {
		logger.Info("updating configs configmap", "configmap", cm.GetName(), "hash", hash)
		cm.Annotations[annotationConfigHash] = hash
		cm.Data = cmData
		if err := r.Update(ctx, cm); err != nil {
			return "", err
		}
		r.recorder.Eventf(chainNode,
			corev1.EventTypeNormal,
			appsv1.ReasonConfigsUpdated,
			"Configuration files updated",
		)
		return hash, nil
	}
	return hash, nil
}

func (r *Reconciler) getGeneratedConfigs(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) (map[string]interface{}, error) {
	logger := log.FromContext(ctx)

	lock := getConfigsLockForAppVersion(chainNode.GetAppImage())
	lock.Lock()
	defer lock.Unlock()

	configs, err := r.getConfigsFromCache(chainNode.GetAppImage())
	if err != nil {
		return nil, err
	}

	if configs != nil {
		logger.Info("loaded configs from cache", "version", chainNode.GetAppVersion())
		return configs, nil
	}

	logger.Info("generating new config files", "version", chainNode.GetAppVersion())
	configFiles, err := app.GenerateConfigFiles(ctx)
	if err != nil {
		return nil, err
	}

	decodedConfigs := make(map[string]interface{}, len(configFiles))
	for name, content := range configFiles {
		decodedConfigs[name], err = utils.TomlDecode(content)
		if err != nil {
			return nil, err
		}
	}

	r.storeConfigsInCache(chainNode.GetAppImage(), decodedConfigs)
	return r.getConfigsFromCache(chainNode.GetAppImage())
}

func (r *Reconciler) storeConfigsInCache(key string, configs map[string]interface{}) {
	r.configCache.Set(key, configs, ttlcache.DefaultTTL)
}

func (r *Reconciler) getConfigsFromCache(key string) (map[string]interface{}, error) {
	data := r.configCache.Get(key)
	if data == nil {
		return nil, nil
	}

	// Make a copy of the configs in cache
	cfgCopy := make(map[string]interface{}, len(data.Value()))
	for item, content := range data.Value() {
		cfgCopy[item] = content
	}
	return cfgCopy, nil
}

func getConfigHash(configs map[string]interface{}) (string, error) {
	hash, err := hashstructure.Hash(configs, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets: true,
		ZeroNil:      true,
	})
	return strconv.FormatUint(hash, 10), err
}

func (r *Reconciler) getPeerConfiguration(ctx context.Context, chainNode *appsv1.ChainNode, kf *KeyFormatter) (map[string]interface{}, error) {
	peers := make([]string, 0)
	unconditional := make([]string, 0)
	private := make([]string, 0)

	var peersList []appsv1.Peer
	if chainNode.AutoDiscoverPeersEnabled() {
		chainPeers, _, err := r.getChainPeers(ctx, chainNode)
		if err != nil {
			return nil, err
		}
		peersList = append(chainPeers, chainNode.Spec.Peers...)
	} else {
		peersList = chainNode.Spec.Peers
	}

	for _, peer := range peersList {
		peers = append(peers, fmt.Sprintf("%s@%s:%d", peer.ID, peer.Address, peer.GetPort()))
		if peer.IsUnconditional() {
			unconditional = append(unconditional, peer.ID)
		}
		if peer.IsPrivate() {
			private = append(private, peer.ID)
		}
	}

	return map[string]interface{}{
		kf.P2P(): map[string]interface{}{
			kf.PersistentPeers():      strings.Join(peers, ","),
			kf.UnconditionalPeerIDs(): strings.Join(unconditional, ","),
			kf.PrivatePeerIDs():       strings.Join(private, ","),
		},
	}, nil
}

func (r *Reconciler) getChainPeers(ctx context.Context, chainNode *appsv1.ChainNode, getAnnotations ...string) ([]appsv1.Peer, []map[string]string, error) {
	// List all services with the same chain ID label
	listOption := client.MatchingLabels{LabelChainID: chainNode.Status.ChainID}
	svcList := &corev1.ServiceList{}
	if err := r.List(ctx, svcList, listOption); err != nil {
		return nil, nil, err
	}

	peers := make([]appsv1.Peer, 0)
	annotationsList := make([]map[string]string, 0)

	for _, svc := range svcList.Items {
		if svc.Labels[LabelNodeID] == chainNode.Status.NodeID {
			continue
		}

		peer := appsv1.Peer{
			ID:            svc.Labels[LabelNodeID],
			Address:       svc.Name,
			Port:          pointer.Int(chainutils.P2pPort),
			Unconditional: pointer.Bool(true),
		}

		if svc.Labels[LabelValidator] == StringValueTrue {
			peer.Private = pointer.Bool(true)
		}

		peers = append(peers, peer)
		annotations := make(map[string]string)
		for _, annotation := range getAnnotations {
			annotations[annotation] = svc.Annotations[annotation]
		}
		annotationsList = append(annotationsList, annotations)
	}

	sort.Slice(annotationsList, func(i, j int) bool {
		return peers[i].ID < peers[j].ID
	})

	sort.Slice(peers, func(i, j int) bool {
		return peers[i].ID < peers[j].ID
	})

	return peers, annotationsList, nil
}

func getMostRecentHeightFromServicesAnnotations(annotationsList []map[string]string) (int64, string) {
	var trustHeight int64
	var trustHash string

	for _, annotations := range annotationsList {
		heightStr, ok := annotations[AnnotationStateSyncTrustHeight]
		if !ok {
			continue
		}

		height, err := strconv.ParseInt(heightStr, 10, 64)
		if err == nil && height > trustHeight {
			trustHeight = height
			trustHash = annotations[AnnotationStateSyncTrustHash]
		}
	}

	return trustHeight, trustHash
}

func getExternalAddress(chainNode *appsv1.ChainNode) (string, bool) {
	if chainNode.Status.PublicAddress != "" {
		parts := strings.Split(chainNode.Status.PublicAddress, "@")
		if len(parts) == 2 {
			return parts[1], true
		}
	}
	return "", false
}
