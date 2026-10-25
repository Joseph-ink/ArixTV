# ArixTV Architecture

## Overview

ArixTV separates three responsibilities:

1. **Discovery/control plane** — ArixTV
2. **TLS edge** — Nginx
3. **Optional media relay** — external IPTV proxy

ArixTV does not proxy video bytes.


## Upstream research context

ArixTV's initial provider catalog is researched from the current `kineticman/FastChannels` repository.

FastChannels is not an architectural dependency at runtime.

It is used to answer:

```text
Which providers exist?
How are channels discovered?
How is a fresh stream URL resolved?
Which streams are clear vs DRM?
Which providers need proxying or browser/device bridges?
```

The classification is maintained in:

```text
docs/FASTCHANNELS_MAPPING.md
```

ArixTV then independently implements only the providers that fit its smaller architecture.


## Core data flow

```text
                 ┌─────────────────┐
                 │ Provider APIs   │
                 └────────┬────────┘
                          │
                     refresh
                          │
                  ┌───────▼───────┐
                  │    ArixTV     │
                  │ discovery     │
                  │ normalization │
                  │ filtering     │
                  └───────┬───────┘
                          │
               provider snapshots
                          │
                    M3U generation
                          │
                          ▼
                 protected playlist
```

Tune flow:

```text
Player
  │
  │ GET /play/provider/channel
  ▼
ArixTV
  │
  │ provider.Resolve()
  ▼
fresh upstream URL
  │
  └── HTTP 302 ──► Player ──► provider CDN
```

## Portable core

Phase 0 currently contains only `cmd/arixtv/`, `internal/cli/`, and
`internal/config/`. The remaining packages below are planned, not stubbed out.
Linux deployment assets are reserved under `deploy/linux/` and do not affect
configuration validation or version reporting on macOS.

Portable Go core should contain responsibilities such as:

```text
cmd/arixtv/
internal/config/
internal/channel/
internal/provider/
internal/playlist/
internal/resolver/
internal/auth/
internal/store/
internal/server/
internal/status/
```

Linux deployment helpers must remain outside portable assumptions.

## Provider isolation

Provider interface concept:

```go
type Provider interface {
    ID() string
    Name() string
    Discover(ctx context.Context) ([]Channel, error)
    Resolve(ctx context.Context, ch Channel) (Stream, error)
}
```

The refresh coordinator executes providers independently.

A provider failure:
- records an error;
- retains previous valid provider snapshot;
- does not abort successful provider updates.

## Snapshot architecture

```text
/etc/arixtv/data/providers/
├── provider-a.json
├── provider-b.json
└── provider-c.json
```

Updates use temporary file + fsync + atomic rename.

The server consumes immutable snapshots and reloads completed generations.

## LAN Direct

```text
                   local network
                        │
                        ▼
                 ArixTV LAN listener
                        │
                CIDR ACL + token
                        │
                        ▼
                   provider resolver
                        │
                       302
                        │
                        ▼
                 client → upstream
```

Authorization requires both source network permission and token validity.

## Remote Direct

```text
Internet
   │ HTTPS
   ▼
Nginx
   │ HTTP
   ▼
127.0.0.1:ArixTV
   │
  302
   │
   └────────► client → upstream CDN
```

ArixTV does not terminate TLS.

Nginx route:

```text
/direct/
```

## Optional Remote Relay

```text
Internet
   │ HTTPS
   ▼
Nginx /relay/
   │
   ▼
external IPTV proxy
   │
   │ reads local relay.m3u
   ▼
ArixTV internal loopback resolver
   │
  302
   ▼
upstream CDN
   │
   ▼
external IPTV proxy
   │
   ▼
client
```

ArixTV core remains unchanged whether relay is installed or not.

## Why the relay is external

Keeping relay outside ArixTV avoids adding:
- HLS manifest rewriting;
- media segment streaming;
- buffer management;
- connection fan-out;
- video bandwidth accounting;
- codec/transcoding concerns.

ArixTV remains a small resolver/control-plane service.

## systemd architecture

```text
arixtv.service
    │
    ├── HTTP service
    ├── resolver
    ├── health/status
    └── snapshot reload

arixtv-refresh.timer
    │
    ▼
arixtv-refresh.service
    │
    ▼
/etc/arixtv/arixtv refresh
```

Do not build a periodic scheduler into the daemon on Linux.

## macOS architecture

The same Go core must run on `darwin/arm64`.

macOS V1 supports:
- serve;
- refresh;
- provider tests;
- config validation;
- playlist generation.

Linux-specific installation/systemd behavior remains outside core logic.
