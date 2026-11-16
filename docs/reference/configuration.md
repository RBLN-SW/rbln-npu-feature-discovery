# NPU Feature Discovery settings

This component reads device information from the host and writes a feature file
for NFD. Use settings supported by your installed component and operator releases.

## Collection settings

Flags override environment variables. The environment suffixes below use the
prefix `RBLN_NPU_FEATURE_DISCOVERY_`.

| Flag | Environment suffix | Default / purpose |
| --- | --- | --- |
| `--sleep-interval` | `SLEEP_INTERVAL` | `60`; seconds between collections, from `10` to `3600` |
| `--output-file` / `-o` | `OUTPUT_FILE` | `/etc/kubernetes/node-feature-discovery/features.d/rbln-features` |
| `--oneshot` | `ONESHOT` | `false`; collect once and exit |
| `--no-timestamp` | `NO_TIMESTAMP` | `false`; omit the expiry timestamp |

For a standalone deployment, set these in the copied manifest described in the
[deployment guide](../guides/deploy-standalone.md). Keep the output file in the
directory shared with NFD's local source. Changing its path also requires matching
volume mounts and NFD configuration.

The old `--rbln-daemon-url` flag is accepted but ignored: this component no longer
collects through the daemon. Operator-managed collection settings depend on the
operator release's supported configuration fields.

## Logging

| Environment variable | Default | Accepted values |
| --- | --- | --- |
| `RBLN_NPU_FEATURE_DISCOVERY_LOG_LEVEL` | `info` | `error`, `warning`/`warn`, `info`, `debug` |
| `RBLN_NPU_FEATURE_DISCOVERY_LOG_FORMAT` | `json` | `json`, `text` |

For an operator deployment, set `npuFeatureDiscovery.logging.level: debug`
and `npuFeatureDiscovery.logging.format: json` in your existing Helm values,
then apply the updated values to your Helm release.
For standalone deployment, set the environment variables in the manifest.

[Read the updated Pod's logs](../guides/check-node-labels.md#3-investigate-missing-labels)
and [check the generated labels](../guides/check-node-labels.md).
Invalid logging settings fall back to defaults with a warning. Restore `info`
after diagnosis.

[Back to the guides](../../README.md#guides)
