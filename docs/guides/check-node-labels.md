# Check NPU node labels

Confirm that NPU Feature Discovery and NFD have published usable node labels.

## 1. List NPU nodes

```bash
kubectl get nodes -L rebellions.ai/npu.present,rebellions.ai/npu.product,rebellions.ai/npu.count
```

For a particular node:

```bash
kubectl get node <node-name> -o yaml
```

Look under `metadata.labels` for `rebellions.ai/npu.*` and
`rebellions.ai/driver-version.*` keys.

## 2. Check the expected values

| Label | Expected result |
| --- | --- |
| `npu.present` | Whether the node has discovered NPUs |
| `npu.count` | Discovered device count; PFs hosting VFs are excluded |
| `npu.product` | Product name, when it can be resolved |
| `driver-version.full` | Driver version, when available |

All keys above use the `rebellions.ai/` prefix. Kubernetes label values are
strings. Product or driver-version labels may be absent when that information
is unavailable. See the [label reference](../reference/labels.md) for the complete
list and driver-version fields.

The `rebellions.ai/npu.family` label is created by the operator's NFD rule.
Check that rule separately if only the family label is missing.

## 3. Investigate missing labels

- Confirm that the NPU Feature Discovery Pod is running on the target node.
- Check its logs for driver, sysfs, or output-directory errors.
- Confirm that NFD's local source is enabled and reads the shared feature directory.
- For Pending Pods, check events and the deployment's node affinity.

For an operator deployment, list the Pods with:

```bash
kubectl get pods -n rbln-system -o wide
kubectl logs -n rbln-system <feature-discovery-pod> --all-containers --tail=100
```

Use your component namespace for a standalone installation.
If needed, [enable debug logs](../reference/configuration.md#logging) and confirm
the [feature output path](../reference/configuration.md#collection-settings) matches NFD's mount.

Next: [schedule a Pod by NPU family](https://docs.rbln.ai/latest/software/system_management/kubernetes/tutorials/find_and_schedule.html).
