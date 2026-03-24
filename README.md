# VK Calls Tunnel

Data tunnel through VK voice calls. Encodes data into Opus audio frames, transmits via VK's WebRTC media servers.

VK media server IPs are guaranteed whitelisted — making this one of the most resilient methods to bypass IP whitelists in Russia.

## How it works

```
Client (Russia) → VK call (whitelisted media servers) → Bot on the other end → Decode → Internet
```

1. Bot (Python) holds a VK account, waits for incoming calls
2. Client initiates VK voice call to the bot
3. WebRTC connection established through VK media servers
4. Client encodes data into Opus audio frames (48kHz) and sends as audio stream
5. Bot receives audio, decodes data, proxies requests to the internet
6. Response encoded back into audio, sent to client
7. Client decodes — data received

DPI sees a normal VK voice call.

## Performance

- **Speed:** ~50-200 Kbps
- **Latency:** ~200-500ms per round-trip
- **Encoding:** Opus payload replacement (faster) or LSB steganography (stealthier)

### What works
- Text messengers (Telegram, Signal)
- Email
- Web pages (slow)

### What doesn't
- Video streaming
- Normal browsing speed
- Large downloads

## Why this is resilient

VK calls use media servers with IPs that are **guaranteed** in any Russian whitelist. Blocking them = disabling VK calls for the entire country. RKN won't do that.

Unlike cloud IP fishing (Yandex Cloud, Cloud.ru) where you gamble on getting a whitelisted IP, VK media servers are always whitelisted by definition.

## Status

Working prototype tested. Code being cleaned up for public release.

## Need help with

- WebRTC / VK API expertise
- Opus codec encoding/decoding optimization
- Python asyncio
- Auto-reconnection on call drops
- VK bot account management (avoiding bans)

## Architecture

```
┌─────────────────┐     VK Call (WebRTC)     ┌─────────────────┐
│  Client          │ ──────────────────────► │  Bot (Python)    │
│  Data → Opus     │    VK Media Servers     │  Opus → Data     │
│  encode          │    (whitelisted IPs)    │  decode + proxy   │
└─────────────────┘                          └────────┬─────────┘
                                                      │
                                                      ▼
                                               Free Internet
```

## Related

- [vpn-gcloud](https://github.com/kobzevvv/vpn-gcloud) — VLESS+Reality deployment, whitelist architecture
- [Xray-core](https://github.com/XTLS/Xray-core) — VLESS+Reality protocol
- [ntc.party](https://ntc.party) — Censorship bypass community

## Author

[@kobzevvv](https://twitter.com/kobzevvv)
