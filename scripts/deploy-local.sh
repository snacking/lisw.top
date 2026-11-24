#!/usr/bin/env bash
# Compile on WSL/Linux; VPS receives only the finished artifact.
set -euo pipefail
cd "$(dirname "$0")/.."
./scripts/build-release.sh
release="$(git rev-parse --short=12 HEAD)-$(date -u +%Y%m%d%H%M%S)"
remote="${DEPLOY_SSH_HOST:-racknerd}"
scp artifacts/panel-linux-amd64.tar.gz "$remote:/tmp/lisw-panel-$release.tar.gz"
ssh "$remote" bash -s -- "$release" < deploy/install-release.sh
