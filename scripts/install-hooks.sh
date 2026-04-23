#!/bin/sh
set -eu
cd "$(git rev-parse --show-toplevel)"
git config core.hooksPath .githooks
chmod +x .githooks/pre-commit .githooks/commit-msg 2>/dev/null || true
echo "Git hooks installed (core.hooksPath = .githooks)."
