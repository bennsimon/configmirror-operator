# configmirror

A kubernetes operator that replicates configmaps based on label selector and target namespaces given.

## Description

The operator introduces a custom resources to handle its logic:

### ConfigMirror

This resource takes in the following properties;

1. SourceNamespace (**MUST** be a valid Namespace, **Required**)
2. TargetNamespaces (String array of valid Namespace, **Required**)
3. LabelSelector (Map of labels, **Should have at least one property**).

Sample

````yaml
apiVersion: bennsimon.github.io/v1alpha1
kind: ConfigMirror
metadata:
  labels:
    app.kubernetes.io/name: configmirror
    app.kubernetes.io/managed-by: kustomize
  name: configmirror-sample
spec:
  sourceNamespace: default
  targetNamespaces:
    - config-mirror
  selector:
    matchLabels:
      app: mySvc
````

In the above sample configmaps, in the `default` namespace with labels `{app: mySvc}` will be replicated to the
`config-mirror` namespace and then saved to the database.

> When a replicated configmap is deleted it will be recreated automatically.

### Configuration

#### Container Environment Configuration

| Configuration             | Description                                      | Default                    |
|---------------------------|--------------------------------------------------|----------------------------|
| `CM_DATABASE_HOST`        | Specifies the postgresql database host.          | ``                         |
| `CM_DATABASE_PASSWORD`    | Specifies the postgresql database user password. | ``                         |
| `CM_DATABASE_USERNAME`    | Specifies the postgresql database username.      | ``                         |
| `CM_DATABASE_PORT`        | Specifies the postgresql database port.          | ``                         |
| `CM_DATABASE_NAME`        | Specifies the postgresql database name.          | ``                         |
| `SAVE_REPLICATION_ACTION` | Specifies whether replication action is saved.   | "false" (same as omission) |

#### GitHub Actions

Update the following GitHub action workflow with proper AWS configurations i.e. `AWS_ACCOUNT_ID` and `AWS_REGION`.
[docker-publish.yml](.github/workflows/docker-publish.yml)
[helm-publish.yml](.github/workflows/helm-publish.yml)

> Both the docker image and helm chart are published to a container repository.

## Deployment

### Creating release tag

This creates a docker image by github action workflow.

````shell
git tag -a v0.0.1 -m "initial release" && git push origin v0.0.1
````

### On existing cluster

#### Helm Chart

Find the helm chart in the following [directory](charts).

## Getting Started

### Prerequisites

- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<container-repository-domain>/configmirror-operator:tag
```

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<container-repository-domain>/configmirror-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
> privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

> **NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

### How it works

This project aims to follow the
Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the
cluster.

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

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.


[homepage]: https://bennsimon.me

[github]: https://github.com/bennsimon
