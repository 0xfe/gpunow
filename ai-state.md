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

Notes:
- Do not store secrets in repo.
- Default log level WARN; no logs on normal runs.
- All Go code must live under go/.
