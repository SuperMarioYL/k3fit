#!/usr/bin/env bash
set -euo pipefail

# K3Fit install script — builds and installs the k3fit binary to GOPATH/bin.
# Usage: ./scripts/install.sh   (or)   curl -fsSL https://raw.githubusercontent.com/SuperMarioYL/k3fit/main/scripts/install.sh | bash

REPO="github.com/SuperMarioYL/k3fit"
INSTALL_PATH="${K3FIT_INSTALL_PATH:-${GOPATH:-$(go env GOPATH)}/bin/k3fit}"

echo "Building k3fit from $REPO …"
go install "$REPO/cmd/k3fit@latest"

BIN="${GOPATH:-$(go env GOPATH)}/bin/k3fit"
if [ -f "$BIN" ]; then
  echo "Installed: $BIN"
  echo "Run: k3fit --vram 32 --ram 128"
else
  echo "Build succeeded but binary not found at $BIN — check your GOPATH/bin is in PATH." >&2
  exit 1
fi
