# ai-state

Repo: /Users/mo/git/gpunow
Purpose: Go CLI to manage GPU VMs/clusters on GCP. Replaces legacy shell script.

Key files/dirs:
- plan.md: milestone plan + progress log. Update status + append progress.
- DESIGN.md: architecture + behavioral decisions.
- profiles/<name>/: config profiles. `profiles/default` contains config.toml, cloud-init.yaml, setup.sh, zshrc.
- state/state.json: cluster state (profile, timestamps, last action).
- go/: Go module. CLI entrypoint at go/cmd/gpunow.
- VERSION: semantic version used for build metadata.
- justfile: build/test helpers.

Decisions locked:
- CLI framework: urfave/cli/v2.
- UI stack: lipgloss + mpb.
- GCP integration: Compute API only (no gcloud CLI).

Config assumptions:
- Hostnames: <cluster>-<index>.
- Cluster index starts at 0; master = index 0 with public IP.
- Per-cluster VPC + subnet; deterministic CIDR derived from base.
- SSH/SCP uses ProxyJump via master for non-0 nodes.

Work status:
- M0 complete: go module + skeleton, VERSION, justfile, DESIGN.md, ai-state.md, profiles/default config.toml, README/AGENTS updated, cloud-init/setup moved.
- M1 complete: config loader/validation, CLI skeleton, logging/UI scaffolding, target parsing, initial tests.
- Home resolution: GPUNOW_HOME → ./profiles → ~/.config/gpunow. State written under <home>/state.
- `gpunow init` initializes ~/.config/gpunow/profiles/default from a source profile directory.
- M2 complete: VM ops using Compute API, cloud-init rendering, updated CLI, tests.
- M3 complete: cluster networking and lifecycle, subnet derivation, labels, tests.
- M4 complete: SSH/SCP resolution via master proxy, command execution, tests.
- M5 complete: progress bars for long ops.
- M6 complete: docs refreshed.

Notes:
- Do not store secrets in repo.
- Default log level WARN; no logs on normal runs.
- All Go code must live under go/.
