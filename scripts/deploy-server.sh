#!/bin/bash
# Deploy vk-tunnel-server to a Google Cloud VPS
# Usage: ./scripts/deploy-server.sh <VPS_IP> [SSH_KEY_PATH]
#
# Prerequisites:
#   - GCE instance with Debian/Ubuntu
#   - Firewall rule allowing UDP/TCP 56000
#   - WireGuard installed on the VPS

set -euo pipefail

VPS_IP="${1:?Usage: $0 <VPS_IP> [SSH_KEY_PATH]}"
SSH_KEY="${2:-$HOME/.ssh/google_compute_engine}"
SSH_USER="${SSH_USER:-vova}"
TUNNEL_PORT="${TUNNEL_PORT:-56000}"

echo "=== Building server for Linux amd64 ==="
GOOS=linux GOARCH=amd64 go build -o vk-tunnel-server-linux ./cmd/server/

echo "=== Uploading to $VPS_IP ==="
scp -i "$SSH_KEY" vk-tunnel-server-linux "$SSH_USER@$VPS_IP:/tmp/vk-tunnel-server"

echo "=== Installing on VPS ==="
ssh -i "$SSH_KEY" "$SSH_USER@$VPS_IP" bash -s "$TUNNEL_PORT" << 'REMOTE'
set -euo pipefail
TUNNEL_PORT="$1"

sudo mv /tmp/vk-tunnel-server /usr/local/bin/vk-tunnel-server
sudo chmod +x /usr/local/bin/vk-tunnel-server

# Create systemd service
sudo tee /etc/systemd/system/vk-tunnel.service > /dev/null << EOF
[Unit]
Description=VK Calls TURN Tunnel Server
After=network.target wg-quick@wg0.service

[Service]
Type=simple
ExecStart=/usr/local/bin/vk-tunnel-server -listen 0.0.0.0:${TUNNEL_PORT} -connect 127.0.0.1:51820
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable vk-tunnel
sudo systemctl restart vk-tunnel

echo "=== Status ==="
sudo systemctl status vk-tunnel --no-pager
REMOTE

echo ""
echo "=== Done ==="
echo "Server running on $VPS_IP:$TUNNEL_PORT"
echo ""
echo "Client command:"
echo "  ./vk-tunnel-client -peer $VPS_IP:$TUNNEL_PORT -vk-link \"https://vk.com/call/join/YOUR_LINK\" -n 4"
