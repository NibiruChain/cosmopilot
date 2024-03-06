# nibiru-operator
Kubernetes controllers to manage nibiru (and other cosmos nodes) on a Kubernetes cluster.

## Description
// TODO(user): An in-depth paragraph about your project and overview of use

## Requirements
* Kubernetes cluster v1.29

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster

Before deploying, you need to have a `ghcr` secret in the `nibiru-system` namespace

```sh
kubectl create secret docker-registry ghcr --docker-server='ghcr.io' --docker-email='devops@nibiru.org' --docker-username='nibibot' --docker-password='<password>' --namespace='nibiru-system'
```

You can obtain the `<password>` from a `ghcr` secret from another cluster. For example,

```sh
kubectl get secret ghcr -n nibiru-system -o jsonpath='{.data.\.dockerconfigjson}' | base64 --decode
```

1. Install Instances of Custom Resources:

```sh
make install
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push VERSION=<your-tag>
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy VERSION=<your-tag>
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023 Nibiru.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

