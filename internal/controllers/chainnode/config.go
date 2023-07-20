package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jellydator/ttlcache/v3"
	"github.com/mitchellh/hashstructure/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/NibiruChain/nibiru-operator/api/v1"
	"github.com/NibiruChain/nibiru-operator/internal/chainutils"
	"github.com/NibiruChain/nibiru-operator/internal/utils"
)

var (
	defaultConfigToml = map[string]interface{}{
		"rpc": map[string]interface{}{
			"laddr":                "tcp://0.0.0.0:26657",
			"cors_allowed_origins": []string{"*"},
		},
		"p2p": map[string]interface{}{
			"addr_book_file":     "/home/app/data/addrbook.json",
			"addr_book_strict":   false,
			"allow_duplicate_ip": true,
		},
		"instrumentation": map[string]interface{}{
			"prometheus": true,
		},
	}

	validatorConfigToml = map[string]interface{}{
		"p2p": map[string]interface{}{
			"pex": false,
		},
	}

	defaultAppToml = map[string]interface{}{
		"api": map[string]interface{}{
			"enable":  true,
			"address": "tcp://0.0.0.0:1317",
		},
		"grpc": map[string]interface{}{
			"address": "0.0.0.0:9090",
		},
	}
)

func (r *Reconciler) ensureConfig(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) (string, error) {
	logger := log.FromContext(ctx)

	configs, err := r.getGeneratedConfigs(ctx, app, chainNode)
	if err != nil {
		return "", err
	}

	// Apply app.toml and config.toml defaults
	configs[appTomlFilename], err = utils.Merge(configs[appTomlFilename], defaultAppToml)
	if err != nil {
		return "", err
	}

	configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], defaultConfigToml)
	if err != nil {
		return "", err
	}

	// Apply state-sync config
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.StateSync.Enabled() {
		configs[appTomlFilename], err = utils.Merge(configs[appTomlFilename], map[string]interface{}{
			"state-sync": map[string]interface{}{
				"snapshot-interval":    chainNode.Spec.Config.StateSync.SnapshotInterval,
				"snapshot-keep-recent": chainNode.Spec.Config.StateSync.GetKeepRecent(),
			},
		})
		if err != nil {
			return "", err
		}
	}

	// Apply seed-mode if enabled
	if chainNode.Spec.Config.SeedModeEnabled() {
		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
			"p2p": map[string]interface{}{
				"seed_mode": true,
			},
		})
		if err != nil {
			return "", err
		}
	}

	if chainNode.StateSyncRestoreEnabled() {
		peers, stateSyncAnnotations, err := r.getChainPeers(ctx, chainNode, AnnotationStateSyncTrustHeight, AnnotationStateSyncTrustHash)
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
			return "", fmt.Errorf("could not find other peers for this chain")
		}

		trustHeight, trustHash := getMostRecentHeightFromServicesAnnotations(stateSyncAnnotations)
		if trustHeight == 0 {
			r.recorder.Event(chainNode,
				corev1.EventTypeWarning,
				appsv1.ReasonNoTrustHeight,
				"no chainnode with valid trust height config is available",
			)
			return "", fmt.Errorf("could not find ChainNode with valid state-sync configs")
		}

		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
			"statesync": map[string]interface{}{
				"enable":       true,
				"rpc_servers":  strings.Join(rpcServers, ","),
				"trust_height": trustHeight,
				"trust_hash":   trustHash,
				"trust_period": defaultStateSyncTrustPeriod,
			},
		})
		if err != nil {
			return "", err
		}
	}

	// Apply validator configs
	if chainNode.IsValidator() {
		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], validatorConfigToml)
		if err != nil {
			return "", err
		}
		if chainNode.UsesTmKms() {
			configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], map[string]interface{}{
				"priv_validator_laddr": privValidatorListenAddress,
			})
			if err != nil {
				return "", err
			}
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

	// Apply peer configuration
	peerConfig, err := r.getPeerConfiguration(ctx, chainNode)
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
			logger.Info("creating configs configmap")
			cm = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.GetName(),
					Namespace: chainNode.GetNamespace(),
					Labels:    chainNode.Labels,
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
		logger.Info("updating configs configmap")
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

	configs, err := r.getConfigsFromCache(chainNode.Spec.App.GetImage())
	if err != nil {
		return nil, err
	}

	if configs != nil {
		return configs, nil
	}

	logger.Info("generating new config files")
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

	r.storeConfigsInCache(chainNode.Spec.App.GetImage(), decodedConfigs)
	return r.getConfigsFromCache(chainNode.Spec.App.GetImage())
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

func (r *Reconciler) getPeerConfiguration(ctx context.Context, chainNode *appsv1.ChainNode) (map[string]interface{}, error) {
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
		"p2p": map[string]interface{}{
			"persistent_peers":       strings.Join(peers, ","),
			"unconditional_peer_ids": strings.Join(unconditional, ","),
			"private_peer_ids":       strings.Join(private, ","),
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

		if svc.Labels[LabelValidator] == "true" {
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
