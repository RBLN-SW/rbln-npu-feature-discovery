# RBLN NPU Feature Discovery

**Make NPU hardware information available as Kubernetes node labels.**

[Get started](#get-started) · [Guides](#guides) · [Documentation](https://docs.rbln.ai/latest/software/system_management/kubernetes/npu_feature_discovery.html) · [Releases](https://github.com/RBLN-SW/rbln-npu-feature-discovery/releases)

NPU Feature Discovery collects device information and writes a feature file.
Kubernetes Node Feature Discovery (NFD) reads it through its `local` source and
applies labels that help you find NPU nodes and place workloads on suitable hardware.

## Get started

1. [Install the NPU Operator](https://docs.rbln.ai/latest/software/system_management/kubernetes/install_npu_operator.html) with NFD and NPU Feature Discovery.
2. [Check the generated labels](docs/guides/check-node-labels.md).

For a separate deployment with an existing NFD installation, follow the
[standalone guide](docs/guides/deploy-standalone.md).

## Guides

| I want to… | Guide |
| --- | --- |
| Find NPU nodes and inspect their labels | [Check node labels](docs/guides/check-node-labels.md) |
| Run a workload on a specific NPU family | [Schedule by family](https://docs.rbln.ai/latest/software/system_management/kubernetes/tutorials/find_and_schedule.html) |
| Understand the available labels | [Read the label reference](docs/reference/labels.md) |
| Use an existing NFD deployment | [Deploy the component separately](docs/guides/deploy-standalone.md) |
| Change collection or logging settings | [Configuration reference](docs/reference/configuration.md) |

## Labels at a glance

| Label | Purpose |
| --- | --- |
| `rebellions.ai/npu.present` | Find nodes with NPUs |
| `rebellions.ai/npu.count` | Inspect the discovered device count |
| `rebellions.ai/npu.product` | Identify the NPU product |
| `rebellions.ai/driver-version.full` | Inspect the driver version |

The `rebellions.ai/npu.family` label is provided by the operator's NFD rule.
It is separate from the labels produced by this component.

## Development

See [Contributing](CONTRIBUTING.md) to build and test NPU Feature Discovery.

## Support

Report bugs and feature requests in [GitHub issues](https://github.com/RBLN-SW/rbln-npu-feature-discovery/issues).
Include the NFD version, expected labels, and component logs.

[License](LICENSE)
