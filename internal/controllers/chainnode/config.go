package chainnode

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jellydator/ttlcache/v3"
	"github.com/mitchellh/hashstructure/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Apply validator configs
	if chainNode.IsValidator() {
		configs[configTomlFilename], err = utils.Merge(configs[configTomlFilename], validatorConfigToml)
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

	var currentHash string
	if cm.Annotations == nil {
		cm.ObjectMeta.Annotations = make(map[string]string)
	} else {
		currentHash = cm.Annotations[annotationConfigHash]
	}

	if currentHash != hash {
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

	if chainNode.AutoDiscoverPeersEnabled() {
		// List all services with the same chain ID label
		listOption := client.MatchingLabels{labelChainID: chainNode.Status.ChainID}
		svcList := &corev1.ServiceList{}
		if err := r.List(ctx, svcList, listOption); err != nil {
			return nil, err
		}

		for _, svc := range svcList.Items {
			if svc.Labels[labelNodeID] == chainNode.Status.NodeID {
				continue
			}
			peers = append(peers, fmt.Sprintf("%s@%s:26656", svc.Labels[labelNodeID], svc.Spec.ClusterIP))
			unconditional = append(unconditional, svc.Labels[labelNodeID])
			if svc.Labels[labelValidator] == "true" {
				private = append(private, svc.Labels[labelNodeID])
			}
		}
	}

	for _, peer := range chainNode.Spec.Peers {
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
