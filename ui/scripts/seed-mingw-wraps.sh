#!/usr/bin/env bash
# Pre-seed meson wrap-git dependencies for Frida MinGW cross builds.
# Must tolerate incomplete .wrap metadata (grep may return empty under set -e/pipefail).
# On any clone/checkout failure, remove the partial tree so later runs can retry.
# Canonical copy is also embedded at ui/internal/rebuild/seed_mingw_wraps.sh (go:embed).
set -euo pipefail
SRC="${1:-/work/frida}"
seed_wraps() {
  local wrapdir="$1"
  [ -d "$wrapdir" ] || return 0
  local f name url rev dest
  shopt -s nullglob
  for f in "$wrapdir"/*.wrap; do
    [ -f "$f" ] || continue
    name=$(basename "$f" .wrap)
    dest="$wrapdir/$name"
    if [ -f "$dest/meson.build" ] || [ -f "$dest/configure.ac" ] || [ -f "$dest/CMakeLists.txt" ]; then
      echo "[fridare] wrap $name already present"
      continue
    fi
    # grep may exit 1 when no match — never abort the seed loop
    url=$(grep -E '^url\s*=' "$f" 2>/dev/null | head -1 | sed 's/.*=[[:space:]]*//' || true)
    rev=$(grep -E '^revision\s*=' "$f" 2>/dev/null | head -1 | sed 's/.*=[[:space:]]*//' || true)
    if [ -z "$url" ] || [ -z "$rev" ]; then
      echo "[fridare] skip wrap $name (no url/revision)"
      continue
    fi
    # Never use smashed magic org URLs
    url=${url//github.com\/*\//github.com/frida/}
    case "$url" in
      *github.com/frida/*) ;;
      *) url=$(echo "$url" | sed -E 's#github.com/[^/]+/#github.com/frida/#') ;;
    esac
    echo "[fridare] seeding wrap $name @ $rev from $url"
    rm -rf "$dest"
    ok=0
    if git clone --depth 1 "$url" "$dest" 2>/dev/null; then
      if (cd "$dest" && git fetch --depth 1 origin "$rev" 2>/dev/null && git checkout "$rev" 2>/dev/null); then
        ok=1
      elif (cd "$dest" && git checkout "$rev" 2>/dev/null); then
        ok=1
      fi
    fi
    if [ "$ok" -ne 1 ]; then
      echo "[fridare] WARN: seed failed for $name (clone/checkout); removing partial tree"
      rm -rf "$dest"
      continue
    fi
  done
  shopt -u nullglob
  return 0
}
seed_wraps "$SRC/subprojects/frida-gum/subprojects" || true
seed_wraps "$SRC/subprojects/frida-core/subprojects" || true
echo "[fridare] seed-mingw-wraps done"
exit 0
