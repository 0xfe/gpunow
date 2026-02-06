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

# Start a VM
$ gpunow vm start
✓ Created gpu0

# See VM status
➜ gpunow vm status
gpu0 (default) RUNNING
• Auto-terminating in 11h 7m (at 04:26am)
• Zone: us-east1-d
• Machine: g2-standard-16
  -> api: compute.machineTypes.get projects/symbolic-axe-717/zones/us-east1-d/machineTypes/g2-standard-16
  • CPUs: 16 (X86_64)
  • RAM: 64 GB
  • GPU: nvidia-l4 x1
• External IP: 34.26.181.230

# SSH to VM
$ gpunow ssh
gpu0$ 
```

## Prerequisites
- Go 1.25.6 (for building).
- GCP credentials with Compute Engine permissions.
- Compute Engine API enabled and quota for the selected GPU type.

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

Single VM (defaults to `vm.default_name` from the config):
```bash
./bin/gpunow vm start
./bin/gpunow vm status
./bin/gpunow vm stop
```

Cluster:
```bash
./bin/gpunow cluster start my-cluster -n 3
./bin/gpunow cluster status my-cluster
./bin/gpunow cluster update my-cluster --max-hours 24
./bin/gpunow cluster stop my-cluster --delete
```

Reference a VM inside a cluster using `<cluster>/<index>`:
```bash
./bin/gpunow vm status my-cluster/0
./bin/gpunow vm status my-cluster/2
```

SSH and SCP:
```bash
./bin/gpunow ssh my-cluster/0
./bin/gpunow ssh my-cluster/2 -u mo -- nvidia-smi
./bin/gpunow ssh            # defaults to vm.default_name
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

## Configuration
Configuration profiles live under `profiles/<name>` and contain:
- `config.toml`
- `cloud-init.yaml`
- `setup.sh`
- `zshrc`

The default profile is `profiles/default`. Use `-p/--profile` to select another profile:
```bash
./bin/gpunow cluster start my-cluster -n 3 -p gpu-l4
```

## GPUNOW_HOME, Profiles, and State
`gpunow` resolves its home directory in this order:
1. `GPUNOW_HOME` if set.
2. `~/.config/gpunow`.

Profiles are read from `<home>/profiles`, and state is written to `<home>/state/state.json`.
Use `gpunow install` to create `~/.config/gpunow/profiles/default`.

Key settings in `config.toml`:
- Project and zone
- VM/cluster defaults (machine type, GPU type/count, max run hours)
- Network defaults and exposed ports
- Service account and scopes
- Disk image and size
- Optional hostname domain for FQDN hostnames (`instance.hostname_domain`)

## Behavior Notes
- Each cluster gets its own VPC and subnet; all nodes are reachable internally.
- Instance 0 is the master node and gets a public IP.
- SSH/SCP to non-master nodes proxies through the master automatically.
- `gpunow ssh` forwards your agent so you can hop from the master to workers.
- `vm` commands accept either a name or a `cluster/index` target.
- VM creation uses the configured `network.default_network`.
- `vm start` with a `cluster/index` target only starts existing nodes; use `cluster start` to create nodes.
- Hostnames: GCE requires a fully qualified domain name (FQDN) if you set `instance.hostname_domain`.
  Leave it empty to use the default internal DNS hostname derived from the instance name.

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
