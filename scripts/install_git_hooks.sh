#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HOOKS_DIR="${ROOT_DIR}/.git/hooks"

if [[ ! -d "${HOOKS_DIR}" ]]; then
  echo "error: .git/hooks not found (run inside repo clone)"
  exit 1
fi

install -m 0755 "${ROOT_DIR}/scripts/hooks/pre-commit" "${HOOKS_DIR}/pre-commit"
install -m 0755 "${ROOT_DIR}/scripts/hooks/pre-push" "${HOOKS_DIR}/pre-push"

echo "Installed git hooks:"
echo "  - pre-commit (runs make verify)"
echo "  - pre-push   (runs make verify)"
