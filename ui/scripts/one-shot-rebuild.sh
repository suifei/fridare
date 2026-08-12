#!/usr/bin/env bash
# One-shot: Docker bootstrap → clone → mod → compile → catalog StageDone
# Drives shipped e2e-rebuild / orchestrator only.
set -euo pipefail

VERSION="${VERSION:-17.16.4}"
TARGET="${TARGET:-android-arm64}"
MAGIC="${MAGIC:-abcde}"
PORT="${PORT:-27142}"
WORK="${WORK:-}"
PROXY="${FRIDARE_PROXY:-${HTTPS_PROXY:-${HTTP_PROXY:-http://localhost:8080}}}"
MIRROR="${FRIDARE_DOCKER_MIRROR:-docker.1ms.run}"
IMAGE="${IMAGE:-fridare/frida-builder:latest}"
TIMEOUT="${TIMEOUT:-6h}"
PROFILE="${PROFILE:-safe}"
DRY=0
AGENT=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      cat <<EOF
Fridare one-shot rebuild (orchestrator → StageDone)
  --version TAG   (default 17.16.4)
  --target ID     (default android-arm64)
  --magic NAME    (default abcde)
  --work DIR
  --proxy URL
  --mirror HOST   (default docker.1ms.run)
  --profile safe|explore
  --dry-run
  --agent         enable GrokAgent when grok on PATH
EOF
      exit 0
      ;;
    --version) VERSION="$2"; shift 2 ;;
    --target) TARGET="$2"; shift 2 ;;
    --magic) MAGIC="$2"; shift 2 ;;
    --work) WORK="$2"; shift 2 ;;
    --proxy) PROXY="$2"; shift 2 ;;
    --mirror) MIRROR="$2"; shift 2 ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    --profile) PROFILE="$2"; shift 2 ;;
    --dry-run) DRY=1; shift ;;
    --agent) AGENT=1; shift ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
UI_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$UI_ROOT"
export CGO_ENABLED=0

ARGS=(run ./cmd/e2e-rebuild
  -version "$VERSION" -target "$TARGET" -magic "$MAGIC" -port "$PORT"
  -proxy "$PROXY" -mirror "$MIRROR" -image "$IMAGE" -timeout "$TIMEOUT"
  -profile "$PROFILE"
)
[[ -n "$WORK" ]] && ARGS+=(-work "$WORK")
[[ "$DRY" -eq 1 ]] && ARGS+=(-dry-run)
[[ "$AGENT" -eq 1 ]] && ARGS+=(-agent)

echo "=== Fridare one-shot rebuild ==="
echo "go ${ARGS[*]}"
exec go "${ARGS[@]}"
