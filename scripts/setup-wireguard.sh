#!/bin/bash
# Set up WireGuard on a fresh VPS (server side)
# Usage: ssh into VPS, then run this script
#
# After running, copy the client config printed at the end

set -euo pipefail

WG_PORT=51820
WG_NETWORK="10.66.66"
WG_INTERFACE="wg0"

echo "=== Installing WireGuard ==="
sudo apt-get update -qq
sudo apt-get install -y -qq wireguard qrencode

echo "=== Generating keys ==="
SERVER_PRIVKEY=$(wg genkey)
SERVER_PUBKEY=$(echo "$SERVER_PRIVKEY" | wg pubkey)
CLIENT_PRIVKEY=$(wg genkey)
CLIENT_PUBKEY=$(echo "$CLIENT_PRIVKEY" | wg pubkey)
PRESHARED_KEY=$(wg genpsk)

echo "=== Server config ==="
sudo tee /etc/wireguard/$WG_INTERFACE.conf > /dev/null << EOF
[Interface]
Address = ${WG_NETWORK}.1/24
ListenPort = ${WG_PORT}
PrivateKey = ${SERVER_PRIVKEY}
PostUp = iptables -A FORWARD -i %i -j ACCEPT; iptables -t nat -A POSTROUTING -o $(ip route | grep default | awk '{print $5}' | head -1) -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -t nat -D POSTROUTING -o $(ip route | grep default | awk '{print $5}' | head -1) -j MASQUERADE

[Peer]
PublicKey = ${CLIENT_PUBKEY}
PresharedKey = ${PRESHARED_KEY}
AllowedIPs = ${WG_NETWORK}.2/32
EOF

sudo chmod 600 /etc/wireguard/$WG_INTERFACE.conf

echo "=== Enabling IP forwarding ==="
sudo sysctl -w net.ipv4.ip_forward=1
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf > /dev/null

echo "=== Starting WireGuard ==="
sudo systemctl enable wg-quick@$WG_INTERFACE
sudo systemctl start wg-quick@$WG_INTERFACE

echo ""
echo "=========================================="
echo "  WireGuard server is running!"
echo "=========================================="
echo ""
echo "--- CLIENT CONFIG (save as wg0.conf) ---"
echo ""
cat << EOF
[Interface]
PrivateKey = ${CLIENT_PRIVKEY}
Address = ${WG_NETWORK}.2/32
DNS = 1.1.1.1, 8.8.8.8
MTU = 1280

[Peer]
PublicKey = ${SERVER_PUBKEY}
PresharedKey = ${PRESHARED_KEY}
Endpoint = 127.0.0.1:9000
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
EOF
echo ""
echo "NOTE: Endpoint is 127.0.0.1:9000 — this points to vk-tunnel-client, not directly to VPS"
echo "=========================================="
