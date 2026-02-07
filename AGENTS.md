# Repository Guidelines

## Project Structure & Module Organization
- `go/`: Go module for the `gpunow` CLI.
- `go/cmd/gpunow`: CLI entrypoint.
- `go/internal/`: Core packages for config, GCP API, cluster/vm logic, SSH/SCP, UI, logging, and validation.
- `profiles/<name>/`: Config profiles with `config.toml`, `cloud-init.yaml`, `setup.sh`.
- `state/`: Local state (clusters, profiles, timestamps) under the resolved gpunow home.
- `VERSION`: Version string baked into binaries.
- `justfile`: Build/test helpers.
- `README.md`: Usage and operational notes.
- `DESIGN.md`: Design decisions and architecture.
- `plan.md`: Milestones and progress log.
- `ai-state.md`: Compact AI-facing repo context.

## Build, Test, and Development Commands
- `just build`
- `just test`
- `just fmt`
- `just vet`
- `just tidy`

## Coding Style & Naming Conventions
- Go code only under `go/`.
- Prefer explicit, readable names (e.g., `ProjectID`, `InstanceName`).
- Keep interfaces small and focused for testability.
- Use `zap` for logging; default level WARN.
- Keep CLI output consistent and styled via the `ui` package.

## Testing Guidelines
- Use unit tests for config parsing/validation and name parsing.
- Mock GCP interactions via interfaces.
- Document manual validation in PRs when cloud changes are involved.

## Commit & Pull Request Guidelines
- Short, imperative commit messages (e.g., "Add cluster subnet derivation").
- PRs should include a summary of changes and rationale.
- PRs should include commands run (or “Not run”).
- PRs should include configuration changes or risks.
- PRs should confirm `README.md` was reviewed and updated.

## Security & Configuration Tips
- Avoid committing secrets or credentials.
- Treat project/zone/service account values as sensitive operational config.
- Keep `README.md` and `profiles/default/config.toml` in sync when defaults change.
- Tag all GCP resources that support labels with `gpunow=0xfe` (requested as `0xfe/gpunow` but slashes are not allowed in GCP label keys). This label is used for status sync and discovery.
