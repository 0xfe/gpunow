# gpunow Design

## Overview
`gpunow` is a Go-based CLI for provisioning multi-node GPU clusters on GCP. It replaces the legacy shell script with a structured, testable architecture and adds cluster-aware networking, SSH/SCP helpers, and configuration profiles.

## Goals
- Simple, consistent CLI for cluster lifecycle.
- Configuration profiles under `profiles/<name>` with a default profile.
- Per-cluster networking with internal reachability for all nodes.
- All nodes have ephemeral public IPs.
- SSH/SCP convenience with direct connections to each node.
- Strong validation, clear errors, and reliable, idempotent GCP operations.

## Implementation Packages
- `internal/config`: config loading and validation.
- `internal/gcp`: Compute Engine API wrapper interfaces.
- `internal/instance`: shared instance builder for VM/cluster creation.
- `internal/cluster`: cluster orchestration and networking.
- `internal/ssh`: SSH/SCP argument construction and resolution.
- `internal/ui`: terminal output styling and progress.

## CLI Surface
- `gpunow install`
- `gpunow create <cluster> -n/--num-instances N [--start]`
- `gpunow start <cluster>`
- `gpunow stop <cluster> [--delete] [--keep-disks]`
- `gpunow status [cluster]`
- `gpunow update <cluster> --max-hours N`
- `gpunow ssh <cluster/idx> [-u user] [-- cmd]`
- `gpunow scp <src> <dst> [-u user]`

## Configuration
- Profile directory: `profiles/<name>`.
- Required files: `config.toml`, `cloud-init.yaml`, `setup.sh`.
- `config.toml` is parsed and validated; default values are explicit.
- Profile discovery order: `GPUNOW_HOME` → `~/.config/gpunow`.

## State
- Cluster state is stored under `<home>/state/state.json` with profile, timestamps, and last action.

Key schema highlights:
- `project.id`, `project.zone`
- `cluster.network_name_prefix`, `cluster.subnet_cidr_base`, `cluster.subnet_prefix`
- `instance.machine_type`, `instance.max_run_hours`, `instance.provisioning_model`
- `network.default_network`, `network.ports`, `network.tags_base`
- `gpu.type`, `gpu.count`
- `disk.image`, `disk.size_gb`, `disk.type`
- `service_account.email`, `service_account.scopes`
- `ssh.default_user`

## Networking
- Each cluster gets its own VPC and subnet.
- VPC name: `<network_name_prefix>-<cluster>`.
- Subnet CIDR: derived deterministically from the cluster name using the configured base CIDR and prefix length.
- Firewall rules allow internal traffic within the subnet.
- Firewall rules allow SSH to all cluster nodes from the public internet.
- Firewall rules allow optional ports (from config) to all cluster nodes.

## Instance Naming
- Instance names: `<cluster>-<index>`.
- Optional hostname: if `instance.hostname_domain` is set, hostname becomes `<name>.<domain>`.
- Instance index starts at 0.
- All cluster nodes receive ephemeral public IPs.
- Cluster instances are labeled with `cluster`, `cluster_index`, and `cluster_role`.

## SSH/SCP
- `gpunow ssh` and `gpunow scp` construct OpenSSH commands.
- SSH user defaults to `ssh.default_user` unless `-u` is provided.
- `gpunow ssh` uses agent forwarding for convenience.

## Logging and Output
- Logging uses `zap` with default level WARN.
- User-visible output is styled via a small `ui` package.
- Progress for long operations uses `mpb` with consistent styling.

## Test Strategy
- Unit tests for config parsing and validation.
- Unit tests for naming and parsing logic (`cluster/idx`).
- Unit tests for SSH/SCP command construction.
- GCP layer tests use interface mocks and fake responses.

## Versioning
- Version stored in `VERSION` at repo root.
- Injected into binaries via `-ldflags` at build time.
