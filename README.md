# GPU VM launcher (GCP)

This repo contains a single helper script, `manage-gpu.sh`, which creates and manages a spot (preemptible) GPU VM named `gpu0` in GCP.

## Prerequisites

Install the GCP SDK:

```bash
# Install on mac
brew install --cask google-cloud-sdk

# Login
gcloud auth login

# Set defaults
gcloud config set project symbolic-axe-717
gcloud config set compute/zone us-east1-d
```

Note: this tool expects that there are project-wide SSH keys setup, and the default
user is `mo`.

## Usage

```bash
./manage-gpu.sh start
./manage-gpu.sh start --max-hours 6

# ssh with your project-wide SSH key. you might get host key errors if you
# reprivisioned the host.

ssh mo@EXTERNAL_IP

# setup.sh runs on first boot via cloud-init
# if needed:
mo@gpu0$ ./setup.sh

# zsh is the default shell; PATH updates are sourced from ~/.local/bin/env
```

This will create a GPU VM named `gpu0` using a spot provisioning model with a 12-hour max run duration and auto-delete on termination.

You can also stop, delete, or show the instance:

```bash
./manage-gpu.sh stop
./manage-gpu.sh stop --delete
./manage-gpu.sh stop --delete --keep-disks
./manage-gpu.sh show
./manage-gpu.sh update --max-hours 24
```

Open custom ports (default is `22,80,443,8000`):

```bash
./manage-gpu.sh start --ports 22,80,443,8000,4000,5000
```

## Current Configuration

- GPU: 1x NVIDIA L4 (24 GB).
- CPU/RAM: `g2-standard-16` (16 vCPU, 64 GB RAM).

Keep this section in sync with `manage-gpu.sh` if `MACHINE_TYPE` or `ACCELERATOR` changes.

## Customization

Edit `manage-gpu.sh` to adjust:

- Project or zone (`--project`, `--zone`)
- Machine type (`--machine-type`)
- GPU type/count (`--accelerator`)
- Boot disk size/type (`--create-disk`)
- Service account and scopes (`--service-account`, `--scopes`)
- Network tags/labels
- Cloud-init template (`gpu-cloud-init.yaml`)
- Setup script (`setup.sh`)

## Behavior Notes

- `start` will start an existing stopped instance; if the instance was deleted, it will be recreated.
- If a boot disk named `gpu0` already exists, `start` reuses it when recreating the VM.
- `stop` without flags only stops the VM. Use `--delete` to delete the VM, and `--keep-disks` to preserve disks.
- Spot terminations or `--delete` remove the VM; with `auto-delete` enabled on the boot disk, data on that disk is lost unless you explicitly keep disks.
- Stopping the instance does not delete disks; storage is only removed when the VM is deleted and auto-delete is enabled.
- Use `--max-hours N` with `start` to set a different max run duration. The default is 12 hours.
- Use `update --max-hours N` to change the max run duration on an existing instance (instance must be stopped).

## Cloud-init

The VM is provisioned from the `gpu-cloud-init.yaml` template on creation. `manage-gpu.sh` renders the template by inlining `setup.sh`, then applies it as user-data. The cloud-init flow performs an apt update, installs base packages (including zsh), ensures the `mo` user exists, fixes ownership of `/home/mo`, writes and runs `/home/mo/setup.sh`, and configures UFW to allow ports 22 and 8000. It also adds `source "$HOME/.local/bin/env"` to `/home/mo/.zshrc` so `uv` is on PATH in new shells.

## Cleanup

If you need to delete the instance manually:

```bash
gcloud compute instances delete gpu0 --zone=us-east1-d
```

## Notes

- Spot capacity is not guaranteed; the instance may be terminated at any time.
- The script uses `gcloud beta` for the `max-run-duration` flag. Ensure your CLI has the beta components available.
