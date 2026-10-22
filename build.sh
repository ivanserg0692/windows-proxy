#!/usr/bin/env bash
set -euo pipefail

if (( $# > 1 )) || [[ "${1:-}" != "" && "${1,,}" != "test" ]]; then
  echo 'Usage: ./build.sh [test]' >&2
  exit 2
fi

go_image='golang:1.27.3'
project_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

if ! command -v docker >/dev/null 2>&1; then
  echo 'ERROR: Docker is required to build.' >&2
  exit 1
fi
if [[ "$(docker info --format '{{.OSType}}')" != 'linux' ]]; then
  echo 'ERROR: Docker Linux engine is unavailable. Check Docker and WSL integration.' >&2
  exit 1
fi
if [[ "$project_dir" == *,* ]]; then
  echo 'ERROR: Project paths containing commas are unsupported by this bind mount.' >&2
  exit 1
fi

# Share the dependency and compiler caches with build.cmd.
docker_args=(
  run --rm
  --mount "type=bind,source=$project_dir,target=/src"
  -w /src
  -v wslproxy-go-mod:/go/pkg/mod
  -v wslproxy-go-build:/root/.cache/go-build
  -e GOPROXY=off
  -e GOTOOLCHAIN=local
)

if [[ "${1:-}" != "" ]]; then
  exec docker "${docker_args[@]}" "$go_image" sh -eu -c \
    'go test -mod=readonly -race ./... && go vet -mod=readonly ./...'
fi

mkdir -p -- "$project_dir/bin"
if ! docker "${docker_args[@]}" \
  -e CGO_ENABLED=0 -e GOOS=windows -e GOARCH=amd64 \
  "$go_image" go build -mod=readonly -trimpath -o bin/proxy.exe ./cmd/proxy; then
  echo 'ERROR: Build failed. Check the output above; dependencies must be downloaded first (see README.md).' >&2
  exit 1
fi

printf 'Built: %s/bin/proxy.exe\n' "$project_dir"
