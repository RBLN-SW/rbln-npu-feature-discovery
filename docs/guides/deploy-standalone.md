# Deploy with an existing NFD installation

Install NPU Feature Discovery separately when the NPU Operator does not already
manage it on the target nodes.

## Before you start

- Prepare NPU nodes with a compatible RBLN driver.
- Enable NFD's local feature source.
- Mount `/etc/kubernetes/node-feature-discovery/features.d` into the NFD worker.
- Check out the component release you intend to deploy.

## 1. Prepare the manifest

From the repository root, copy the reference manifest:

```bash
cp deployments/static/npu-feature-discovery-daemonset.yaml /tmp/rbln-npu-feature-discovery.yaml
```

Edit the copy to select your release's image and configure the namespace, node
affinity, and tolerations. The checked-in manifest has its own image pin; confirm
that it matches your intended release.

The default namespace is `node-feature-discovery`. The target nodes must have
the PCI labels selected by the manifest, and the host feature directory must exist.

## 2. Deploy

For the default namespace:

```bash
kubectl create namespace node-feature-discovery --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f /tmp/rbln-npu-feature-discovery.yaml
kubectl rollout status daemonset/rbln-npu-feature-discovery -n node-feature-discovery --timeout=5m
```

## 3. Verify

[Check node labels](check-node-labels.md). Expect hardware labels to appear after
NPU Feature Discovery collects information and NFD processes it.

To adjust the collection interval, set `RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL`
in the container environment. Its default is 60 seconds.

## Remove this deployment

```bash
kubectl delete -f /tmp/rbln-npu-feature-discovery.yaml
```

Stop all instances using the feature file before following the
[label cleanup guide](https://docs.rbln.ai/latest/software/system_management/kubernetes/npu_feature_discovery.html).
Keep the existing NFD installation for its other feature sources.
