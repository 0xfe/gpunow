# GPUNow

`gpunow` lets you quickly spin up ephemeral GPU VMs or cluster in GCP. It's great for running one-off expermentation, training,
or other GPU-heavy workloads.

## Quickstart

```bash
# Make sure you have just installed
$ just build

# Install into ~/.local/bin and configs into ~/.config/gpunow
./bin/gpunow install

# Start a new shell, or exec bash/zsh so PATH is refreshed.

# Authenticate with GCP
gcloud auth application-default login

# Spin up a quick 3-node cluster
$ gpunow create my-cluster -n 3 --start
✓ Created cluster my-cluster (3 instances) in local state

# Preview estimated cost before creating resources
$ gpunow create my-cluster -n 3 --estimate-cost

# See status
➜ gpunow status

# SSH to a specific instance
$ gpunow ssh my-cluster/0
my-cluster-0$

# By defaul, all instances terminate in 12 hours, to terminate manually
# and optionally delete all resources
gpunow stop my-cluster [--delete] [--keep-disks]
```

## Prerequisites

- GCP credentials with Compute Engine permissions.
- Compute Engine API enabled and quota for the selected GPU type.
- Go 1.25.6 (to build from source)

## Authentication (ADC)

Recommended (user credentials):

```bash
gcloud auth application-default login
gcloud auth application-default set-quota-project <your-project-id>
```

## Build
```bash
just build
```
Binary output: `./bin/gpunow`

## Quickstart
Install gpunow and initialize a home profile directory:
```bash
./bin/gpunow install
```

Cluster:
```bash
./bin/gpunow create my-cluster -n 3
./bin/gpunow start my-cluster
./bin/gpunow create my-cluster -n 3 --start
./bin/gpunow create my-cluster -n 3 --estimate-cost
./bin/gpunow create my-cluster -n 3 --estimate-cost --refresh
./bin/gpunow status my-cluster
./bin/gpunow update my-cluster --max-hours 24
./bin/gpunow stop my-cluster --delete
```

Reference a node using `<cluster>/<index>` or `<cluster>-<index>`:
```bash
./bin/gpunow ssh my-cluster/0
./bin/gpunow ssh my-cluster/2
./bin/gpunow ssh my-cluster-2
```

SSH and SCP:
```bash
./bin/gpunow ssh my-cluster/0
./bin/gpunow ssh my-cluster/2 -u mo -- nvidia-smi
./bin/gpunow scp ./local.txt my-cluster/2:/home/mo/
./bin/gpunow scp my-cluster/0:/home/mo/logs.txt ./
./bin/gpunow scp -r -P 2222 ./dir my-cluster/2:/home/mo/
./bin/gpunow scp -- -weird ./local.txt   # use -- to separate flags from paths
```

State:
```bash
./bin/gpunow state
./bin/gpunow state raw
```

Status:
```bash
./bin/gpunow status
./bin/gpunow status sync
```

## Configuration
Configuration profiles live under `profiles/<name>` and contain:
- `config.toml`
- `cloud-init.yaml`
- `setup.sh`
- `zshrc`

The default profile is `profiles/default`. Use `-p/--profile` to select another profile:
```bash
./bin/gpunow create my-cluster -n 3 -p gpu-l4
./bin/gpunow start my-cluster -p gpu-l4
```

## GPUNOW_HOME, Profiles, and State
`gpunow` resolves its home directory in this order:
1. `GPUNOW_HOME` if set.
2. `~/.config/gpunow`.

Profiles are read from `<home>/profiles`, and state is written to `<home>/state/state.json`.
Use `gpunow install` to create `~/.config/gpunow/profiles/default`.

Key settings in `config.toml`:
- Schema version (`version`)
- Project and zone
- Instance defaults (machine type, GPU type/count, max run hours)
- Network defaults and exposed ports
- Service account and scopes
- Disk image and size
- Optional hostname domain for FQDN hostnames (`instance.hostname_domain`)
 - Optional SSH identity file (`ssh.identity_file`)

## Behavior Notes
- Each cluster gets its own VPC and subnet.
- All cluster nodes get ephemeral public IPv4 addresses (destroyed with the VM).
- `gpunow ssh` and `gpunow scp` connect directly to each node.
- Firewall rules apply to all cluster nodes.
- Host-level `ufw` is enabled and allows SSH (`22/tcp`) by default.
- Network defaults control additional allowed ports when configured.
- Hostnames: GCE requires a fully qualified domain name (FQDN) if you set `instance.hostname_domain`.
  Leave it empty to use the default internal DNS hostname derived from the instance name.
- `gpunow create --estimate-cost` estimates VM core/RAM, GPU, and boot disk pricing using the Cloud Billing Catalog API.
- Pricing data is cached at `<home>/state/pricing-cache.json` and reused automatically.
- Use `--refresh` with `--estimate-cost` to force re-download of pricing data.
- Estimates intentionally exclude egress, discounts/credits, taxes, and OS/license premiums.

## Repository Layout
- `go/`: Go source code
- `profiles/`: profile templates
- `VERSION`: version string used at build time
- `justfile`: build/test helpers

## Development
```bash
just test
just fmt
just vet
```

## Release
Cut a release tag and trigger GitHub Actions:
```bash
just release [version]
```

If no version is provided, `just release` bumps the patch version from `VERSION` (strips `-dev`), commits the change, creates an annotated `vX.Y.Z` tag, and pushes the commit and tag to `origin`. GitHub Actions runs tests and publishes release artifacts for darwin/linux and amd64/arm64.

Local release troubleshooting (same steps as GitHub Actions) can be run with:
```bash
just release-local [version]
```
This runs tests and builds release artifacts into `dist/` for darwin/linux and amd64/arm64.

## Service Accounts

Service account alternative:
```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
```

Required permissions (at minimum):
- `roles/compute.admin` (instances, disks, networks, firewall rules)
- `roles/iam.serviceAccountUser` (attach the configured service account)

Troubleshooting `invalid_grant`:
- Re-run `gcloud auth application-default login`.
- If it persists, revoke and re-login:
```bash
gcloud auth application-default revoke
gcloud auth application-default login
```
