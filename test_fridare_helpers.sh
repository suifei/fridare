#!/usr/bin/env bash
# Drives real generate_random_name / validation from fridare.sh (not a reimplementation).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
# Extract and eval only the helper function body from shipped script
# Strip CR so Windows-checked-out fridare.sh (CRLF) still parses under bash
eval "$(sed -n '/^generate_random_name()/,/^}/p' "$ROOT/fridare.sh" | tr -d '\r')"

fail=0
for i in $(seq 1 20); do
  name="$(generate_random_name)"
  if [[ ! "$name" =~ ^[a-z]{5}$ ]]; then
    echo "FAIL generate_random_name produced: '$name'"
    fail=1
  fi
done

ver="$(grep -E '^VERSION=' "$ROOT/fridare.sh" | head -1 | cut -d= -f2 | tr -d '"' | tr -d '\r')"
if [[ "$ver" != "4.0.1" ]]; then
  echo "FAIL VERSION want 4.0.1 got $ver"
  fail=1
fi

if ! grep -q 'find_frida_native_lib()' "$ROOT/fridare.sh"; then
  echo "FAIL missing find_frida_native_lib"
  fail=1
fi
if ! grep -q 'resign_if_darwin' "$ROOT/fridare.sh"; then
  echo "FAIL missing resign_if_darwin"
  fail=1
fi

if [[ $fail -ne 0 ]]; then
  echo "test_fridare_helpers: FAILED"
  exit 1
fi
echo "test_fridare_helpers: OK (20 random names + structural checks)"
