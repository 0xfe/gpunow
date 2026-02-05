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
