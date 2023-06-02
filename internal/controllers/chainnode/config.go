package chainnode

import (
	"context"
	"encoding/json"

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
		"node_key_file":           "/secret/" + nodeKeyFilename,
		"priv_validator_key_file": "/secret/" + privKeyFilename,
		"genesis_file":            "/genesis/" + genesisFilename,
		"rpc": map[string]interface{}{
			"laddr": "tcp://0.0.0.0:26657",
		},
		"p2p": map[string]interface{}{
			"addr_book_file":     "/home/app/data/addrbook.json",
			"addr_book_strict":   false,
			"allow_duplicate_ip": true,
		},
	}

	defaultAppToml = map[string]interface{}{
		"api": map[string]interface{}{
			"enable": true,
		},
	}
)

func (r *Reconciler) ensureConfig(ctx context.Context, app *chainutils.App, chainNode *appsv1.ChainNode) error {
	logger := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(chainNode), cm)
	mustCreate := false
	if err != nil {
		if errors.IsNotFound(err) {
			mustCreate = true
			cm = &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      chainNode.Name,
					Namespace: chainNode.Namespace,
				},
				Data: make(map[string]string),
			}
			if err := controllerutil.SetControllerReference(chainNode, cm, r.Scheme); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if len(cm.Data) == 0 {
		logger.Info("generating new config files")
		configFiles, err := app.GenerateConfigFiles(ctx)
		if err != nil {
			return err
		}
		for name, content := range configFiles {
			cm.Data[name] = content
		}
	}

	decodedConfigs := make(map[string]interface{}, len(cm.Data))
	for filename, data := range cm.Data {
		decodedConfigs[filename], err = utils.TomlDecode(data)
		if err != nil {
			return err
		}
	}

	// Apply app.toml and config.toml defaults
	logger.Info("applying default configs")
	decodedConfigs[appTomlFilename], err = utils.Merge(decodedConfigs[appTomlFilename], defaultAppToml)
	if err != nil {
		return err
	}

	decodedConfigs[configTomlFilename], err = utils.Merge(decodedConfigs[configTomlFilename], defaultConfigToml)
	if err != nil {
		return err
	}

	// Apply used specified configs
	logger.Info("applying user specified configs")
	if chainNode.Spec.Config != nil && chainNode.Spec.Config.Override != nil {
		for filename, b := range *chainNode.Spec.Config.Override {
			var data map[string]interface{}
			if err := json.Unmarshal(b.Raw, &data); err != nil {
				return err
			}
			if _, ok := decodedConfigs[filename]; ok {
				decodedConfigs[filename], err = utils.Merge(decodedConfigs[filename], data)
			} else {
				decodedConfigs[filename] = data
			}
		}
	}

	// Encode back to configmap
	for filename, data := range decodedConfigs {
		cm.Data[filename], err = utils.TomlEncode(data)
		if err != nil {
			return err
		}
	}

	if mustCreate {
		logger.Info("creating configs configmap")
		return r.Create(ctx, cm)
	}
	logger.Info("updating configs configmap")
	return r.Update(ctx, cm)
}
