#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
source "$HOME/.local/toolchains/env"
export DATABASE_CONFIG_FILE="${DATABASE_CONFIG_FILE:-$HOME/.config/lisw-panel/database.json}"
if [[ ! -f "$DATABASE_CONFIG_FILE" ]]; then unset DATABASE_CONFIG_FILE; fi
npm --prefix frontend ci --prefer-offline --no-audit --no-fund
npm --prefix frontend run build
mkdir -p bin
(cd backend && go build -o ../bin/panel .)
export STATIC_DIR="$PWD/frontend/dist"
export LISTEN_ADDR="${LISTEN_ADDR:-127.0.0.1:8081}"
if [[ -n "${DATABASE_CONFIG_FILE:-}" ]]; then ./bin/panel --migrate; fi
if systemctl --user cat lisw-panel-dev.service >/dev/null 2>&1 && [[ "${DEV_FOREGROUND:-0}" != "1" ]]; then
  systemctl --user restart lisw-panel-dev
  printf 'Preview: http://localhost:8081 (systemd user service)\n'
else
  exec ./bin/panel
fi
