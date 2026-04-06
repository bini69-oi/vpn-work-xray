#!/usr/bin/env bash
# Install pre-commit / pre-push hooks (delegates to repository scripts/).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
exec "${ROOT}/scripts/install_git_hooks.sh"
