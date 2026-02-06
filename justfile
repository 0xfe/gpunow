set shell := ["/bin/zsh", "-lc"]

version := `cat VERSION`
commit := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
build_time := `date -u +%Y-%m-%dT%H:%M:%SZ`

# Show available tasks
list:
  @just --list

# Build the gpunow binary into ./bin
build:
  mkdir -p bin
  cd go && \
    go build -ldflags "-X gpunow/internal/version.Version={{version}} \
    -X gpunow/internal/version.Commit={{commit}} \
    -X gpunow/internal/version.BuildTime={{build_time}}" \
    -o ../bin/gpunow ./cmd/gpunow

# Run all tests

test:
  cd go && go test ./...

# Format Go code
fmt:
  cd go && gofmt -w ./...

# Run go vet
vet:
  cd go && go vet ./...

# Tidy module dependencies

tidy:
  cd go && go mod tidy

# Tag and push a release. If no version is provided, bump patch from VERSION (strips -dev).
release version="":
  @set -euo pipefail; \
  just _check_clean; \
  ver="$(just _resolve_version "{{version}}" true)"; \
  if git rev-parse -q --verify "refs/tags/v$ver" >/dev/null; then \
    echo "error: tag v$ver already exists"; \
    exit 1; \
  fi; \
  echo "$ver" > VERSION; \
  if git diff --quiet -- VERSION; then \
    echo "error: VERSION already set to $ver"; \
    exit 1; \
  fi; \
  git add VERSION; \
  git commit -m "Release v$ver"; \
  git tag -a "v$ver" -m "Release v$ver"; \
  git push origin HEAD; \
  git push origin "v$ver"; \
  echo "released v$ver"

# Build release artifacts locally (same matrix as GitHub Actions).
release-build version="":
  @set -euo pipefail; \
  ver="$(just _resolve_version "{{version}}" false)"; \
  commit="$(git rev-parse --short HEAD 2>/dev/null || echo "none")"; \
  build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"; \
  checksum() { \
    if command -v sha256sum >/dev/null 2>&1; then \
      sha256sum "$1" > "$2"; \
    else \
      shasum -a 256 "$1" > "$2"; \
    fi; \
  }; \
  rm -rf dist; \
  mkdir -p dist; \
  for os in linux darwin; do \
    for arch in amd64 arm64; do \
      pkg="gpunow_${ver}_${os}_${arch}"; \
      out_dir="dist/$pkg"; \
      mkdir -p "$out_dir"; \
      ( \
        cd go && \
        GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
        go build -ldflags "-X gpunow/internal/version.Version=$ver -X gpunow/internal/version.Commit=$commit -X gpunow/internal/version.BuildTime=$build_time" \
        -o "../$out_dir/gpunow" ./cmd/gpunow \
      ); \
      tar -czf "dist/$pkg.tar.gz" -C dist "$pkg"; \
      checksum "dist/$pkg.tar.gz" "dist/$pkg.tar.gz.sha256"; \
    done; \
  done; \
  echo "built dist/* for v$ver"

# Run the same steps as the release workflow locally: tests + build artifacts.
release-local version="":
  @just test
  @just release-build "{{version}}"

# Internal: ensure the git tree is clean.
_check_clean:
  @set -euo pipefail; \
  if [ -n "$(git status --porcelain)" ]; then \
    echo "error: working tree is dirty"; \
    git status --porcelain; \
    exit 1; \
  fi

# Internal: resolve version; optionally bump patch when version is not provided.
_resolve_version version="" bump="false":
  @set -euo pipefail; \
  ver="{{version}}"; \
  bump="{{bump}}"; \
  if [ -z "$ver" ]; then \
    current="$(cat VERSION)"; \
    if [ "$bump" = "true" ]; then \
      base="${current%-dev}"; \
      IFS=. read -r major minor patch <<< "$base"; \
      if [ -z "$major" ] || [ -z "$minor" ] || [ -z "$patch" ]; then \
        echo "error: VERSION must be x.y.z (or x.y.z-dev) when no version is provided"; \
        exit 1; \
      fi; \
      if ! [[ "$major" =~ '^[0-9]+$' && "$minor" =~ '^[0-9]+$' && "$patch" =~ '^[0-9]+$' ]]; then \
        echo "error: VERSION must be numeric semver (x.y.z)"; \
        exit 1; \
      fi; \
      patch=$((patch + 1)); \
      ver="$major.$minor.$patch"; \
    else \
      ver="$current"; \
    fi; \
  fi; \
  ver="${ver#v}"; \
  if ! [[ "$ver" =~ '^[0-9]+\.[0-9]+\.[0-9]+$' ]]; then \
    echo "error: invalid version '$ver' (expected x.y.z)"; \
    exit 1; \
  fi; \
  echo "$ver"
