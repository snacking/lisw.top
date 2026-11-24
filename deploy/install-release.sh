#!/usr/bin/env bash
# Runs on VPS: extract, verify and restart only. No Node, Go or Docker build.
set -euo pipefail
id="$1"
[[ "$id" =~ ^[a-zA-Z0-9-]+$ ]] || exit 2
release="/opt/lisw-panel/releases/$id"
archive="/tmp/lisw-panel-$id.tar.gz"
test -f /etc/lisw-panel/database.json
sudo install -d "$release"
sudo tar -xzf "$archive" -C "$release"
(cd "$release" && sha256sum --check SHA256SUMS)
sudo env DATABASE_CONFIG_FILE=/etc/lisw-panel/database.json DATABASE_CA_FILE="$release/certs/aiven-ca.pem" "$release/panel" --migrate
previous=$(readlink -f /opt/lisw-panel/current || true)
sudo ln -sfn "$release" /opt/lisw-panel/current
restart_panel() {
  if [[ -f /root/sw_proxy_infra/scripts/restart-panel.sh ]]; then
    sudo bash /root/sw_proxy_infra/scripts/restart-panel.sh
  else
    sudo cp /opt/lisw-panel/current/panel.service /etc/systemd/system/lisw-panel.service
    sudo systemctl daemon-reload
    sudo systemctl enable lisw-panel
    sudo systemctl restart lisw-panel
  fi
}
rollback() {
  if [[ -n "$previous" ]]; then
    sudo ln -sfn "$previous" /opt/lisw-panel/current
    restart_panel
  else
    sudo systemctl stop lisw-panel || true
  fi
}
if ! restart_panel; then rollback; exit 1; fi
for attempt in $(seq 1 20); do
  if curl -fsS http://127.0.0.1:18081/api/health; then
    rm -f "$archive"
    exit 0
  fi
  sleep 2
done
rollback
exit 1
