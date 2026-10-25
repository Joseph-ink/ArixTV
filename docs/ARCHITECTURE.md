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

Phase 1 implements `internal/channel`, `provider`, `auth`, `logsafe`,
`atomicfile`, `statefile`, `filelock`, and `store`, alongside the Phase 0 CLI/config.
HTTP, playlist generation, refresh coordination and real providers remain planned.
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


## Phase 1 domain and persistence contracts

`channel.Channel` has explicit JSON field names, validated typed classifications,
an optional SHA-256 metadata fingerprint and last-seen time. It cannot carry a
resolved playback URL or arbitrary opaque session payload. `channel.Stream` is
an ephemeral result; its URL is excluded from JSON and formatting/log values.
Protection and delivery are separate: ordinary HLS encryption can be direct or
relay-only; licensed DRM and unknown classifications cannot become direct.

`provider.Registry` is built once, defensively copies metadata and sorts listings.
It exposes the small context-aware Discover/Resolve runtime interface plus
separate metadata (status, access, unofficial mirror and capabilities). No real
runtime is registered. Operational enabling requires anonymous-free access and
an available stable/experimental runtime; mirror-based metadata cannot be stable
by default. There are no global registries or hidden goroutines.

`store.Commit` validates an explicitly complete, nonempty candidate and the existing
snapshot, then atomically replaces a version-1 file with a monotonically increasing
generation. Duplicate IDs, cross-provider entries, stale/invalid candidates and
unrecognized/corrupt state are errors. Failed discovery/observation never rewrites
channel data. Providers/coordinator must truthfully mark partial results incomplete;
the persistence layer cannot detect a missing upstream page declared complete.

Successful generation/count/timestamps derive from the snapshot. Separate refresh
observations record the latest attempt, fixed safe error code and health, referencing
the corresponding generation. Older observations cannot mask a newer success.
There is no multi-file transaction or HTTP status endpoint. Snapshot reads are
bounded at 16 MiB, token/observation reads at 1 MiB, and JSON rejects duplicate,
unknown and trailing data. Single-file readers always see a complete old or new
version; files are not cached in mutable global state.

`atomicfile` serializes JSON before replacement and can also persist arbitrary
M3U bytes later. Temp files share the target directory, are private, synced and
closed before atomic rename; the parent is synced after rename. New directory
entries are synced during provisioning. Tests inject failures at each boundary.
Before-rename errors preserve the old file; after-rename directory-sync failure is
an explicit `DurabilityError` and requires reload/reconciliation. A returned error
must not be interpreted as unconditional rollback. Corrupt files require operator
recovery and are retained for diagnosis rather than automatically erased.

`filelock` uses nonblocking `flock` on Linux and macOS, with close/process exit
releasing the kernel lock. Lock files remain in place permanently. Future refresh
commands must hold `store.LockRefresh(stateDir)` across the full refresh; individual
snapshot/token mutations additionally lock their own files. Other OS targets fail
explicitly. Advisory locks require cooperating processes and operator-controlled
local paths; network filesystems and hostile local writers are not supported.

`auth.Store` persists only named SHA-256 digests and lifecycle metadata. Plaintext
is returned from creation/rotation as a redacting `Secret` wrapper with explicit
`Reveal`. Token mutations use process locks and the same atomic replacement. No
lifecycle CLI or public HTTP auth handler is added in Phase 1.
