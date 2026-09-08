#!/usr/bin/env bash
set -euo pipefail
go run ./cmd/k3fit --vram 32 --ram 128
go run ./cmd/k3fit --vram 96 --ram 256 --quant Q3_K_M
