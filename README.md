# VK Calls Tunnel

WireGuard VPN tunnel through VK voice call infrastructure. Routes encrypted VPN traffic via VK's TURN servers — their IPs are guaranteed whitelisted in Russia.

DPI sees standard DTLS/STUN traffic to VK media servers. Indistinguishable from a real VK call.

## How it works

```
WireGuard Client (Russia)
  → vk-tunnel-client (127.0.0.1:9000)
  → DTLS 1.2 encrypt
  → STUN ChannelData wrap
  → VK TURN server (whitelisted IP)
  → relay to VPS
  → vk-tunnel-server
  → DTLS decrypt
  → WireGuard server (127.0.0.1:51820)
  → Internet
```

No audio encoding, no Opus, no steganography. VK's TURN servers are dumb relays — they forward bytes without inspecting content. We just need valid TURN credentials from a VK call link.

## Performance

- **Speed:** ~5 Mbps per stream, scale with `-n` flag (4 streams ≈ 20 Mbps)
- **Latency:** ~80ms
- **Enough for:** browsing, messengers, video calls, streaming

## Quick start

### 1. Server (VPS outside Russia)

```bash
# Install WireGuard
apt install wireguard

# Configure WireGuard (standard setup)
wg genkey | tee /etc/wireguard/privatekey | wg pubkey > /etc/wireguard/publickey

# Run tunnel server
./vk-tunnel-server -listen 0.0.0.0:56000 -connect 127.0.0.1:51820
```

### 2. Client (inside Russia)

```bash
# Create a VK call link: open vk.com → Calls → Create link
# Or have someone send you one

# Run tunnel client
./vk-tunnel-client \
    -peer YOUR_VPS_IP:56000 \
    -vk-link "https://vk.com/call/join/abc123def" \
    -n 4 \
    -listen 127.0.0.1:9000
```

### 3. WireGuard config

```ini
[Interface]
PrivateKey = <client-private-key>
Address = 10.0.0.2/32
MTU = 1280  # reduced for DTLS/TURN overhead

[Peer]
PublicKey = <server-public-key>
Endpoint = 127.0.0.1:9000  # points to vk-tunnel-client, NOT to VPS directly
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
```

## Build

```bash
make build          # build for current platform
make build-linux    # cross-compile for Linux amd64 (for VPS)
```

## Deploy to Google Cloud (or any VPS)

### One-time setup

```bash
# 1. Create GCE instance (e2-micro is enough)
gcloud compute instances create vk-tunnel \
    --machine-type=e2-micro \
    --image-family=debian-12 \
    --image-project=debian-cloud \
    --zone=europe-west1-b

# 2. Open tunnel port
gcloud compute firewall-rules create vk-tunnel-allow \
    --allow=tcp:56000,udp:56000 \
    --target-tags=vk-tunnel

# 3. SSH in and set up WireGuard
gcloud compute ssh vk-tunnel -- 'bash -s' < scripts/setup-wireguard.sh
# Save the client config it prints!

# 4. Deploy tunnel server
./scripts/deploy-server.sh <VPS_IP>
```

### Quick redeploy (after code changes)

```bash
./scripts/deploy-server.sh <VPS_IP>
# Rebuilds, uploads, restarts systemd service
```

### Client side (Russia)

```bash
# 1. Create VK call link: vk.com → Calls → Create link
# 2. Run tunnel client
make build
./vk-tunnel-client -peer <VPS_IP>:56000 -vk-link "https://vk.com/call/join/..." -n 4

# 3. Start WireGuard with the config from setup step
sudo wg-quick up ./wg0.conf

# 4. Test
curl https://ifconfig.me   # should show VPS IP
```

## Client flags

| Flag | Default | Description |
|------|---------|-------------|
| `-peer` | (required) | Server address (ip:port) |
| `-vk-link` | | VK call link for TURN credentials |
| `-turn` | | Manual TURN server address |
| `-turn-user` | | TURN username (manual mode) |
| `-turn-pass` | | TURN password (manual mode) |
| `-listen` | `127.0.0.1:9000` | Local WireGuard endpoint |
| `-n` | `1` | Parallel DTLS streams (~5 Mbps each) |
| `-tcp` | `true` | TCP transport to TURN server |
| `-psk` | | Pre-shared key (hex) for DTLS auth |
| `-session-id` | (auto) | Fixed session UUID (32-char hex) |
| `-no-dtls` | `false` | Disable DTLS (not recommended) |

## Server flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `0.0.0.0:56000` | Listen address for tunnel clients |
| `-connect` | `127.0.0.1:51820` | WireGuard backend |
| `-psk` | | Pre-shared key (hex) |
| `-no-dtls` | `false` | Disable DTLS |

## Architecture

```
CLIENT (Russia)                         SERVER (VPS abroad)
===============                         ===================

┌─────────────┐                         ┌──────────────┐
│  WireGuard   │                         │  WireGuard    │
│  Client      │                         │  Server       │
└──────┬───────┘                         └──────▲───────┘
       │ UDP :9000                              │ UDP :51820
┌──────▼───────┐                         ┌──────┴───────┐
│  vk-tunnel   │                         │  vk-tunnel   │
│  client      │                         │  server      │
├──────────────┤                         ├──────────────┤
│  Session UUID│                         │  Session Mgr │
│  + DTLS 1.2  │    VK TURN Servers     │  + DTLS 1.2  │
│  + STUN      │◄══════════════════════►│              │
│  ChannelData │   (whitelisted IPs)    │              │
└──────────────┘                         └──────────────┘
```

### Multi-stream

Each TURN stream gives ~5 Mbps. Use `-n 4` for ~20 Mbps. Streams are load-balanced round-robin. If one dies, traffic shifts to remaining streams.

### Session management

- Each client generates a 16-byte UUID
- Server maps UUID → WireGuard connection (prevents endpoint thrashing)
- Multiple streams per session, identified by UUID
- Reconnect-safe: same `-session-id` resumes the session

## Why VK TURN servers?

VK Calls uses TURN servers for NAT traversal in voice/video calls. These IPs are in every Russian ISP whitelist — blocking them would break VK calls nationwide. RKN won't do that.

Unlike cloud IP fishing (Yandex Cloud, Cloud.ru), VK media server IPs are **guaranteed whitelisted by definition**.

## Security

- WireGuard provides end-to-end encryption (ChaCha20-Poly1305)
- DTLS 1.2 encrypts tunnel traffic (makes it look like a real call)
- Optional PSK for DTLS authentication
- VK TURN credentials are temporary and session-scoped

## Limitations

- VK call links expire — need periodic refresh
- ~5 Mbps per stream cap (VK rate limiting)
- Using `-no-dtls` may trigger TURN provider bans
- TURN credentials require a valid VK call link

## Related

- [vk-turn-proxy](https://github.com/kiper292/vk-turn-proxy) — original Go implementation of TURN tunneling
- [vpn-gcloud](https://github.com/kobzevvv/vpn-gcloud) — VLESS+Reality deployment
- [ntc.party](https://ntc.party) — Censorship bypass community

## Author

[@kobzevvv](https://twitter.com/kobzevvv)
