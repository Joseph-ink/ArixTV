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
Phase 2 adds `internal/server` and `internal/servicenotify`. Refresh coordination,
file playlist generation and relay integration remain planned. Phase 3A adds
`internal/provider/vizio` and the production wiring in `internal/provider/builtin`.
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
separate metadata (status, access, unofficial mirror and capabilities).
`builtin.NewRegistry` registers the experimental Vizio runtime without network I/O.
Operational enabling requires anonymous-free access and
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
There is no multi-file transaction. Phase 2 consumes these values for local health. Snapshot reads are
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


## Phase 2 HTTP architecture

`cli.RunContext` and the signal-aware entry point load strict config, build the
production runtime registry and initialize `server.App`. Startup checks token
state and enabled snapshots/refresh metadata without writing state. Unknown or
out-of-scope enabled runtimes fail operational validation. A valid empty registry
serves local health; there are no fake/demo production providers.

Each listener carries an explicit kind. The loopback bind supports Remote Direct,
local health and the future internal resolver. Explicit LAN binds expose Direct
only, enforcing actual socket IP + token. All binds complete before readiness;
failed startup closes earlier sockets. The embedding `Serve` boundary also checks
loopback/LAN binding addresses. Wildcards are never loopback bindings.

A small explicit parser accepts exact GET/HEAD routes, without ServeMux cleaning
or redirects. Percent-escaped paths, duplicate separators, dot components,
backslashes, controls, non-ASCII path components, query strings and absolute-form
request targets are denied. Readable unreserved ASCII channel IDs remain literal;
other opaque IDs are represented as `~` plus canonical base64url. Thus IDs that
contain a slash can be addressed without encoded path delimiters. Decoding only
produces a lookup key, never a filesystem path.

M3U output is sorted by provider/channel identity and includes validated
`clear_direct` channels only. URLs use explicit LAN/remote origins and the same
configured Direct prefix. No Host/forwarded-host value is consulted. The response
is no-store, includes no EPG/guide fields, and is written incrementally to avoid a
complete playlist allocation per client; HEAD computes the same Content-Length.
No generated playlist file or resolved upstream URL is persisted by HTTP serving.

Tune-time resolution performs policy, authentication, strict lookup, classification,
runtime lookup, concurrency admission and context-bounded Resolve. It validates
fresh stream classifications/expiry and emits a bodyless 302 with Location. No
media request is made by the HTTP layer. Errors/panics become sanitized 503s;
unsupported/unknown public channels are 404. Internal resolution can additionally
use `clear_relay_only` channels, but still excludes all protected/unknown cases.

The internal validator is a separate interface with an `irv_` namespace, distinct
from public `atv_` tokens. CLI credential setup is intentionally deferred to Phase 7,
so internal routes default disabled. No public token can substitute. Health/internal
routes require a loopback listener and socket peer, and reject forwarded metadata.

A bounded per-IP limiter holds pre-auth, playlist and resolver buckets. Only actual
LAN peers or strictly validated one-hop proxy clients identify buckets. Extra
connections close before handler creation; active resolver slots have no waiting
queue. Invalid authentication remains 404, even on pre-auth admission denial;
authenticated class limits use 429, and full resolver capacity uses 503.

`catalog` polls enabled snapshot/observation inode/mtime/size stamps, parses only
changed files using Phase 1 strict stores, and atomically publishes immutable views.
It caps aggregate cached channels at 100000. Startup corrupt state fails closed;
runtime corruption/deletion/regression retains in-memory LKG and marks health
unavailable/degraded. Disabled providers are not scanned. Temporary files are not
candidates. Snapshot and observation races are retried on later polls; no disk state
is rewritten. Health includes persisted refresh state and derived stale ages (>2
configured refresh intervals), which does not trigger provider work.

Cancellation stops new work, cancels server/provider contexts, closes listeners and
uses one bounded Shutdown deadline for all servers. On deadline, connections are
force-closed and a safe error returns. Providers must honor context; Go cannot kill
an arbitrary uncooperative provider function, while the CLI process exits when
serving returns. All owned Serve/poll/watchdog goroutines are joined/stopped.

The optional Linux notifier uses Unix datagrams for READY, WATCHDOG and STOPPING;
Darwin is a no-op. WATCHDOG_PID is honored and heartbeat ticks at half WATCHDOG_USEC
in the main reload/service loop, not an unconditional independent background loop.
It indicates main-loop responsiveness, not upstream playback health. Socket writes
have a short deadline. No libsystemd or unit files were added. Protocol references:
[sd_notify](https://www.freedesktop.org/software/systemd/man/latest/sd_notify.html)
and [sd_watchdog_enabled](https://www.freedesktop.org/software/systemd/man/latest/sd_watchdog_enabled.html).

## Phase 3A reference provider

Vizio is the sole production runtime, wired by `provider/builtin`; provider
interfaces and Phase 1/2 schemas, stores, serving/network policies are unchanged.
Constructors do no I/O. Discover returns nil/error for incomplete candidates;
only the later coordinator will decide when to commit a complete snapshot.
Resolve fetches current provider metadata and bounded HLS manifests, returning an
ephemeral validated media URL. The HTTP layer still only emits 302 responses.
This is provider-local format/protection inspection, not a media data plane.
See [Vizio notes](providers/VIZIO.md) for classification limits, resource budgets
and the deliberately incomplete live certification. No refresh scheduler,
coordinator or second provider was introduced.
