# NPU node labels

NPU Feature Discovery writes the labels below to the feature file read by NFD.
All keys use the `rebellions.ai/` prefix, and Kubernetes stores their values as strings.

| Label | Meaning | Example |
| --- | --- | --- |
| `npu.present` | Whether any NPU devices were discovered | `true`, `false` |
| `npu.count` | Discovered device count, excluding PFs hosting SR-IOV VFs | `2` |
| `npu.product` | Resolved NPU product name | `RBLN-CR13` |
| `driver-version.full` | Driver version before the first `-`, `+`, or `~` separator | `1.2.92` |
| `driver-version.major` | Major version component | `1` |
| `driver-version.minor` | Minor version component | `2` |
| `driver-version.patch` | Patch version component | `92` |
| `driver-version.revision` | Suffix after the separator; omitted when absent or empty | `abc` |

For example, driver version `1.2.92-abc` produces `driver-version.full=1.2.92`
and `driver-version.revision=abc`.

If no devices are discovered, only `npu.present=false` is written. When devices
are present, an unresolved product name omits only `npu.product`; missing or
blank driver-version metadata omits the `driver-version.*` labels.

The operator's NFD rule provides `rebellions.ai/npu.family` separately.
NPU Feature Discovery does not produce this label.

[Check node labels](../guides/check-node-labels.md) · [Back to the guides](../../README.md#guides)
