# Repository Guidelines

## Project Structure & Module Organization
- `manage-gpu.sh`: Single entrypoint script to create/manage the GCP GPU VM.
- `README.md`: Usage, prerequisites, and operational notes.
- `AGENTS.md`: This contributor guide.

There are no submodules, libraries, or tests in this repo. Keep additions minimal and aligned with the single-script focus.

## Build, Test, and Development Commands
There is no build system or test suite.
- Run the script locally:
  - `chmod +x manage-gpu.sh`
  - `./manage-gpu.sh start` (create/start the instance)
  - `./manage-gpu.sh stop` (delete the instance and disks)
  - `./manage-gpu.sh show` (describe the instance)
- Prerequisites are described in `README.md` (gcloud CLI, auth, quotas).

## Coding Style & Naming Conventions
- Bash script only; keep it POSIX-friendly where possible.
- Indentation: 2 spaces; keep lines readable and wrap long flags with `\`.
- Use explicit variable names (`PROJECT`, `ZONE`, `INSTANCE_NAME`).
- Preserve `set -euo pipefail` and avoid silent failures.
- Script naming: verbs for actions, e.g., `manage-gpu.sh`.

## Testing Guidelines
No automated tests are present. If you change behavior, verify manually using:
- `./manage-gpu.sh show` before/after
- `./manage-gpu.sh start` and `./manage-gpu.sh stop`
Document any manual validation in your PR.

## Commit & Pull Request Guidelines
- Commit history does not follow a strict convention. Use short, imperative messages (e.g., "Update GPU deletion flow").
- PRs should include:
  - A brief summary of what changed and why.
  - Commands run (or “Not run” if none).
  - Any required configuration changes or risks (e.g., billing impact).
  - An explicit confirmation that `README.md` was reviewed and updated for any code changes in this directory.

## Security & Configuration Tips
- This repo references a specific GCP project, zone, service account, and image.
- Treat these values as sensitive operational config; avoid copying secrets into this repo.
- If you adjust project/zone defaults, update `README.md` and keep the script’s flags in sync.
