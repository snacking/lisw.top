#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ -f "$HOME/.local/toolchains/env" ]]; then source "$HOME/.local/toolchains/env"; fi
npm --prefix frontend ci --prefer-offline --no-audit --no-fund
npm --prefix frontend run build
mkdir -p artifacts/stage/certs
(cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o ../artifacts/stage/panel .)
cp -r frontend/dist artifacts/stage/
cp backend/database/certs/aiven-ca.pem artifacts/stage/certs/
cp deploy/panel.service artifacts/stage/
(cd artifacts/stage && sha256sum panel > SHA256SUMS)
tar -czf artifacts/panel-linux-amd64.tar.gz -C artifacts/stage .
printf 'Built artifact: artifacts/panel-linux-amd64.tar.gz\n'
