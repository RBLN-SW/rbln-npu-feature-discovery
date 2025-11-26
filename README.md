# RBLN NPU Feature Discovery

RBLN NPU Feature Discovery automatically publishes Kubernetes node labels describing the Rebellions NPUs installed on a node. It extends the Node Feature Discovery (NFD) workflow by writing a local feature file that NFD consumes to stamp labels onto the node object.

## Overview

The binary queries the `rbln-daemon` gRPC endpoint whenever it is available and falls back to sysfs inspection under `/sys/class/rebellions`. All collected facts are stored in `/etc/kubernetes/node-feature-discovery/features.d/rbln-features`, which NFD reads through the `local` feature source.

| Component | Purpose |
|-----------|---------|
| RBLN driver | Exposes kernel version and PCI details for Rebellions devices. |
| rbln-daemon | Provides device inventory and driver metadata over gRPC. |
| RBLN NPU Feature Discovery | Collects hardware information periodically and writes local feature entries. |
| Node Feature Discovery | Converts the local feature file into Kubernetes node labels. |

## Prerequisites

- Kubernetes 1.19+ cluster
- Nodes equipped with Rebellions NPUs (ATOM or REBEL families) and RBLN driver 1.2.79 or newer.
- `rbln-daemon` reachable from the pod (default `127.0.0.1:50051`).
- Node Feature Discovery v0.17.x deployed on target nodes with the [local feature source](https://kubernetes-sigs.github.io/node-feature-discovery/v0.17/usage/customization-guide.html#local-feature-source) enabled and `/etc/kubernetes/node-feature-discovery/features.d/` mounted.

## Deployment

### 1. Prepare NPU nodes

Install the RBLN driver by following the [official installation guide](https://docs.rbln.ai/getting_started/installation_guide.html#step-1-rbln-driver) and confirm that the Rebellions PCI devices appear inside `/sys/bus/pci/devices`. The feature discovery pod requires read access to `/sys` and to the NFD feature directory on the host.

### 2. Install NPU Feature Discovery

**DaemonSet (recommended).** Apply `https://raw.githubusercontent.com/rebellions-sw/rbln-npu-feature-discovery/main/deployments/static/npu-feature-discovery-daemonset.yaml`. The manifest:
- Runs in the `node-feature-discovery` namespace with `privileged: true` to read sysfs.
- Mounts `/etc/kubernetes/node-feature-discovery/features.d` and `/sys` from the host.
- Includes node affinity that matches the default PCI labels emitted by NFD for Rebellions devices (`feature.node.kubernetes.io/pci-1200_1eff.present` or `feature.node.kubernetes.io/pci-1eff.present`).
Customize namespace, tolerations, image registry, or resource requests as needed for your cluster.

### 3. Verify node labels

Once both NFD and RBLN NPU Feature Discovery are running, inspect a node with `kubectl get node <npu-node> -o yaml`. The `metadata.labels` section should now include keys such as:
- `rebellions.ai/npu.present=true`
- `rebellions.ai/npu.count=2`
- `rebellions.ai/npu.family=ATOM`
- `rebellions.ai/npu.product=RBLN-CA12`
- `rebellions.ai/driver-version.full=1.2.92-6d00b56`
- `rebellions.ai/driver-version.major=1`
- `rebellions.ai/driver-version.minor=2`
- `rebellions.ai/driver-version.patch=92`

## Generated labels

Label values are stored as strings in Kubernetes. The “Value type” column describes the logical type represented in the string.

| Label | Value type | Description | Examples |
|-------|------------|-------------|----------|
| `rebellions.ai/npu.present` | Boolean | Indicates if any RBLN NPU was detected on the node. | `true`, `false` |
| `rebellions.ai/npu.count` | Integer | Number of NPUs after filtering out PFs that own SR-IOV VFs. | `1`, `2` |
| `rebellions.ai/npu.family` | String | Architecture family derived from the PCI product code. | `ATOM`, `REBEL` |
| `rebellions.ai/npu.product` | String | Product name (e.g., `RBLN-CA22`, `RBLN-CR22`). | `RBLN-CA22`, `RBLN-CR22` |
| `rebellions.ai/driver-version.full` | String | Full semantic version reported by the driver, including optional revision suffix. | `1.2.92-6d00b56` |
| `rebellions.ai/driver-version.major` | Integer | Major component of the driver version. | `1` |
| `rebellions.ai/driver-version.minor` | Integer | Minor component of the driver version. | `2` |
| `rebellions.ai/driver-version.patch` | Integer | Patch component of the driver version. | `92` |

## Configuration

RBLN NPU Feature Discovery accepts both flags and environment variables. Defaults reflect the static manifests under `deployments/static`.

| Flag | Environment variable | Default | Description |
|------|----------------------|---------|-------------|
| `--rbln-daemon-url` | `RBLN_NPU_FEATURE_DISCOVERY_RBLN_DAEMON_URL` | `127.0.0.1:50051` | Endpoint for the `rbln-daemon` gRPC service. |
| `--output-file`, `-o` | `RBLN_NPU_FEATURE_DISCOVERY_OUTPUT_FILE` | `/etc/kubernetes/node-feature-discovery/features.d/rbln-features` | Destination file consumed by the NFD local source. |
| `--sleep-interval` | `RBLN_NPU_FEATURE_DISCOVERY_SLEEP_INTERVAL` | `60` seconds (min 10s, max 3600s) | Time between collections when running continuously. |
| `--oneshot` | `RBLN_NPU_FEATURE_DISCOVERY_ONESHOT` | `false` | Collect features once and exit. Used by the Job template. |
| `--no-timestamp` | `RBLN_NPU_FEATURE_DISCOVERY_NO_TIMESTAMP` | `false` | Skip writing the hourly expiry comment required by NFD. |

Example usage: `rbln-npu-feature-discovery --rbln-daemon-url 10.0.0.20:50051 --sleep-interval 120`.

## Troubleshooting

| Symptom | Suggested action |
|---------|------------------|
| Pod logs `failed to collect features from daemon` repeatedly | Confirm `rbln-daemon` is running on the host and that the pod uses `hostNetwork`. |
| Pod logs `output path validation failed` | Ensure `/etc/kubernetes/node-feature-discovery/features.d/` exists on the node before starting the DaemonSet. |
| Labels do not appear on the node | Verify that NFD is running with the local source enabled and that the feature directory is mounted read-only into the `nfd-worker` pod. |
| DaemonSet remains Pending | Confirm that NFD has applied `feature.node.kubernetes.io/pci-1200_1eff.present` or update the affinity to match your labeling scheme. |
