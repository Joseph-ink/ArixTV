# ArixTV Product and Technical Specification

## 1. Purpose

ArixTV is a lightweight FAST/free-IPTV channel aggregator and M3U resolver.

Its job is to discover supported free live television channels from multiple providers, normalize the channel list, exclude DRM or unsupported playback paths, generate protected M3U playlists, and resolve the latest upstream playback URL at tune time.

The desired user experience is:

1. install once;
2. choose providers;
3. choose LAN/remote access options;
4. subscribe to a private M3U URL;
5. let systemd refresh providers automatically;
6. use the playlist without routine manual intervention.

ArixTV is intentionally not a general-purpose IPTV management platform.

## 2. Project identity

- Project: `ArixTV`
- Binary: `arixtv`
- Minimum supported Go: **1.27.3**; CI and future release builds use Go 1.27.3.
  Older toolchains are unsupported.
- Linux install root: `/etc/arixtv/`
- Linux systemd unit directory: `/etc/systemd/system/`

Required build targets:
- `linux/amd64`
- `linux/arm64`
- `darwin/arm64`

Linux is the production deployment target.

macOS arm64 is a required runtime/development target for:
- local development;
- provider testing;
- manual refresh;
- M3U generation;
- HTTP serving;
- functional debugging.

V1 does not add launchd installation/management.

## 3. Origin and upstream reference

ArixTV began as a redesign of the free FAST/IPTV discovery and resolver capabilities available in:

```text
https://github.com/kineticman/FastChannels
```

FastChannels is the primary upstream research reference for ArixTV's initial provider catalog.

ArixTV is not intended to reproduce FastChannels as a whole. It deliberately extracts only the conceptual capabilities required for a much smaller system:

```text
free FAST provider discovery
        ↓
channel normalization
        ↓
exclude DRM/unsupported playback
        ↓
protected M3U generation
        ↓
tune-time resolver
        ↓
HTTP 302
```

The current FastChannels repository must be reviewed to identify:

- all free/FAST provider implementations;
- paid/account-based providers;
- discovery endpoints/workflows;
- tune-time resolver behavior;
- clear versus DRM/mixed content;
- direct-playable versus proxy-required behavior;
- region/session requirements;
- provider-specific limitations.

The provider migration inventory is maintained in:

```text
docs/FASTCHANNELS_MAPPING.md
```

ArixTV implementations must be independently written in Go. Do not mechanically translate or copy FastChannels code. If reuse rights are unclear, FastChannels must be treated only as a behavioral/protocol reference.

## 4. Core scope

ArixTV core implements:
- provider registry;
- provider discovery;
- channel normalization;
- clear-stream classification;
- provider snapshots;
- protected M3U generation;
- tune-time stream resolution;
- HTTP 302 redirects;
- LAN access policy;
- reverse-proxy direct mode;
- local health/status;
- CLI administration;
- systemd-compatible operation on Linux.

ArixTV core does not implement:
- EPG;
- XMLTV;
- Gracenote;
- DVR;
- recording;
- transcoding;
- video restreaming;
- HLS segment proxying;
- Web UI;
- browser automation;
- DRM bypass;
- Widevine;
- PlayReady;
- FairPlay;
- Redis;
- database servers;
- FFmpeg;
- Node.js;
- ACME;
- certificate issuance;
- VPN management.

## 5. Provider model

A provider supplies channel discovery and tune-time resolution.

Conceptual interface:

```go
type Provider interface {
    Discover(ctx context.Context) ([]Channel, error)
    Resolve(ctx context.Context, ch Channel) (Stream, error)
}
```

Provider identity/display metadata is registered separately from the runtime interface.
Provider implementations must be isolated from each other.

A failure in one provider must not:
- crash the daemon;
- invalidate other providers;
- delete another provider's snapshot;
- erase the failed provider's last-known-good snapshot.

Provider maturity/status should support:
- stable;
- experimental;
- disabled;
- unsupported.

Playback capability should support:
- `clear_direct`;
- `clear_relay_only`;
- `drm`;
- `unsupported`.

The normal V1 public playlist contains `clear_direct` only; `unknown` is an
explicit capability and must fail closed. Stable V1 sources require anonymous,
free access and should use provider-owned/public APIs. Unofficial mirrors are
experimental/future by default. Account-based sources (including free accounts)
are out of scope. Provider metadata represents access type and mirror dependency
separately from maturity and playback capability.

## 6. Initial provider research pool

Initial free FAST/IPTV providers to evaluate:
- Pluto TV
- Xumo Play
- Plex
- Roku Channel
- Samsung TV Plus
- LG Channels
- Vizio WatchFree+
- Zeam

This is a research pool, not a compatibility promise.

Implement only providers that fit ArixTV's product boundaries.

Provider documentation must state:
- current status;
- discovery strategy;
- region considerations;
- whether a provider contains mixed DRM/clear channels;
- known resolver limitations.

## 7. DRM policy

ArixTV must not bypass DRM.

If content requires:
- Widevine;
- PlayReady;
- FairPlay;
- DRM license acquisition;
- browser/device DRM bridge;
- unsupported protected playback;

then the content is excluded from V1 playback.

ArixTV may detect and classify protected content only to exclude it.

Do not implement decryption or circumvention.

Ordinary standards-compliant HLS AES-128 is not automatically DRM. Use explicit
classification values `clear`, `hls_standard_encrypted`, `widevine`, `playready`,
`fairplay`, `other_drm`, and `unknown`. A normal IPTV client may fetch ordinary
HLS keys through its standard playback workflow. If special cookies, headers,
request identity, or same-egress/session context prevent direct client playback,
classify the clear stream as `clear_relay_only`. Same-egress/IP-bound streams
never enter either LAN Direct or Remote Direct playlists, even if a particular
LAN happens to share public egress. External relay integration remains Phase 7.

Anonymous protocols may use public constants, non-secret client IDs, independently
understood algorithms, and locally generated session IDs. Do not copy/distribute
private keys, embedded application secrets, proprietary-client authentication
secrets, account credentials, or secrets from repositories with unclear rights.
If such a secret is indispensable, classify the provider unsupported/out of scope
until the owner explicitly revisits the decision.

## 8. No EPG by design

ArixTV has no EPG subsystem.

There should be no core data model for:
- programs;
- schedules;
- XMLTV;
- guide enrichment.

This is a deliberate performance and scope decision.

## 9. Channel model

A normalized channel should represent at least:

```text
provider
provider_channel_id
name
logo
group
country
language
stream_type
playback_capability
drm_status
last_seen
provider_metadata/fingerprint
```

Avoid storing large opaque provider payloads unless needed for reliable resolution.

Phase 1 channels persist identity/metadata and an optional SHA-256 metadata
fingerprint, with explicit JSON field names. Arbitrary provider payloads, headers,
resolved URLs and session secrets are absent from the channel schema. Public
artwork URLs must have no credentials, query values or fragments; later providers
must normalize or omit unsuitable artwork. Snapshot schema version 1 validates
all enum values, timestamps and provider/channel identity consistency.

## 10. Stream model

Tune-time resolution should return enough information to represent:

```text
URL
stream type
DRM status
expiry if known
```

The public playlist must not permanently embed provider URLs that are signed or short-lived.
The Phase 1 ephemeral `channel.Stream` excludes its URL from JSON, formatting,
and structured log representations. It records capability, type, protection and
optional expiry; validation rejects expired results and contradictory labels.

## 11. Snapshot model

Store provider snapshots under:

```text
/etc/arixtv/data/providers/
```

Example:

```text
pluto.json
xumo.json
plex.json
```

Snapshot update algorithm:

1. fetch;
2. normalize;
3. validate;
4. write temporary file;
5. fsync;
6. atomically rename over previous snapshot.

If refresh fails, preserve the previous valid snapshot.

This is the Last Known Good rule.

Phase 1 `store.Commit` requires an explicitly complete, nonempty candidate, rejects
invalid/duplicate/cross-provider channels and stale generations, and increments
provider generation only on replacement. The coordinator must mark any partial or
errored discovery incomplete. Corrupt/unknown-version state requires operator
recovery and is not silently reset. Failed observations are separate from snapshots.
JSON reads reject unknown/duplicate keys and oversized/trailing content.

Persistence writes mode 0600 files in private newly created directories (0700),
serializes before replacement, syncs the temporary file, closes, renames in the
same directory and syncs that directory. New directory entries are synced in their
parents. A failure before rename preserves the previous file. A directory-sync
failure after rename returns an explicit durability-uncertain error; reload the
committed state instead of assuming rollback. Paths must be on an operator-owned
local filesystem with atomic rename/advisory locking; network-filesystem semantics
are outside the current guarantee.

## 12. Playlist generation

Generated files:

```text
/etc/arixtv/data/playlist.m3u
/etc/arixtv/data/relay.m3u
```

Playlist writes must be atomic.

Public direct M3U entries point to ArixTV:

```text
https://tv.example.com/direct/s/<TOKEN>/play/<provider>/<channel>
```

LAN equivalents may use HTTP and the LAN server address.

Do not persist expiring upstream signed URLs into the long-lived playlist.

## 13. Tune-time resolution

Playback flow:

```text
Player
  ↓
ArixTV /play/<provider>/<channel>
  ↓
Provider.Resolve()
  ↓
fresh upstream URL
  ↓
HTTP 302
  ↓
Player connects to upstream CDN
```

ArixTV core does not carry video bytes.

## 14. Token model

Public playlist and playback routes require strong bearer tokens.

Requirements:
- generated with `crypto/rand`;
- target strength: 256 bits;
- optionally associated with a friendly name;
- persist only SHA-256 token hashes, never recoverable cleartext;
- show cleartext token only when created/rotated;
- never log full token.

Target CLI:

```text
arixtv token list
arixtv token create <name>
arixtv token rotate <name>
arixtv token revoke <name>
```

Phase 1 creates 32 random bytes encoded as `atv_` plus raw URL-safe base64. The
store exposes plaintext only from successful creation/rotation via an explicit
redacting `Secret` wrapper. Verification hashes once and compares against every
stored digest using constant-time comparisons; corrupted state fails closed.
Create/rotate/revoke operations lock and atomically replace `state/tokens.json`.
Names and creation/rotation/revocation timestamps are retained; revoked tokens
cannot be rotated back into activity. CLI lifecycle commands are staged later.

Unauthorized requests should normally return `404`.

## 15. Public endpoints

Remote Direct public surface should be minimal:

```text
/direct/s/<token>/playlist.m3u
/direct/s/<token>/play/<provider>/<channel>
```

Do not publicly expose:
- admin;
- refresh;
- config;
- provider management;
- debug;
- detailed health.

Administration belongs to CLI/systemd/local-only interfaces.

## 16. LAN Direct mode

Purpose: trusted local networks where HTTP is acceptable.

Flow:

```text
LAN Player
   ↓ HTTP
ArixTV
   ↓ 302
Upstream CDN
```

Installation must allow user-selected LAN CIDR(s).

Examples:
- `192.168.0.0/24`
- `192.168.1.0/24`
- `192.168.31.0/24`
- `192.168.50.0/24`
- `10.0.0.0/24`
- custom valid CIDR

LAN authorization requires:

```text
source address in allowed CIDR
AND
valid ArixTV token
```

Prefer explicit LAN address binding over `0.0.0.0`.

The installer should detect candidate private subnets and allow custom input.

## 17. Remote Direct mode

Flow:

```text
Internet Player
    ↓ HTTPS
Nginx
    ↓ HTTP loopback
ArixTV 127.0.0.1
    ↓ 302
Player → upstream CDN
```

TLS is terminated by Nginx.

ArixTV does not implement TLS.

The user supplies an external domain, for example:

```text
tv.example.com
```

Remote Direct path:

```text
/direct/
```

Example playlist:

```text
https://tv.example.com/direct/s/<TOKEN>/playlist.m3u
```

V1 installer only prints required Nginx configuration. It does not modify Nginx or request certificates.

## 18. Trusted proxy behavior

ArixTV may trust forwarded headers only if the socket peer is in configured trusted proxy ranges.

Typical V1 trusted proxy:

```text
127.0.0.1/32
```

Requests from untrusted peers must not be able to spoof address decisions with:
- `X-Forwarded-For`;
- `Forwarded`;
- `X-Real-IP`;
- `X-Forwarded-Proto`.

## 19. Remote Relay architecture

Relay is optional and external.

Flow:

```text
Player
  ↓ HTTPS
Nginx /relay/
  ↓
External IPTV proxy
  ↓
ArixTV loopback resolver
  ↓ 302
Upstream CDN
  ↓
External IPTV proxy
  ↓
Player
```

ArixTV core remains a resolver.

Do not add HLS manifest rewriting or segment proxying to ArixTV.

## 20. Relay local M3U

ArixTV generates:

```text
/etc/arixtv/data/relay.m3u
```

The external proxy reads this local file.

It must not fetch ArixTV's public M3U through Nginx.

Relay entries point to a loopback-only internal resolver, conceptually:

```text
http://127.0.0.1:8787/internal/<INTERNAL_SECRET>/play/<provider>/<channel>
```

The internal secret:
- is separate from public user tokens;
- is not exposed in the public M3U;
- is not reachable via LAN/public listeners.

## 21. External relay backend

The relay backend is replaceable.

An initial lightweight IPTV proxy may be evaluated, but ArixTV must not make provider/core logic depend on one specific implementation.

Before declaring a relay backend supported, integration tests must verify:

```text
local relay.m3u
→ relay proxy
→ ArixTV 302 resolver
→ mock upstream HLS
→ client playback requests
```

Verify nested playlists and segment fetches.

## 22. Nginx path routing

Use one domain and one certificate.

Paths:

```text
/direct/ → ArixTV
/relay/  → relay backend
```

Avoid extra maintenance subdomains.

Nginx should route, not rewrite IPTV content.

Backends should understand their own public path prefixes.

Default examples should avoid writing secrets into Nginx access logs.

## 23. Linux installation layout

Hard requirement:

```text
/etc/arixtv/
├── arixtv
├── config.toml
├── manage.sh
├── uninstall.sh
├── backups/
├── data/
│   ├── playlist.m3u
│   ├── relay.m3u
│   └── providers/
├── state/
└── relay/
```

Project-managed files must not be scattered across `/opt`, `/usr/local/bin`, or `/usr/bin`.

Systemd unit files are the exception and live in:

```text
/etc/systemd/system/
```

## 24. Linux system user

The installer creates an `arixtv` system user/group.

The daemon should not run as root after startup.

Directory permissions should allow only the minimum required writes.

During uninstall, remove the user/group only if the installer created them.

## 25. systemd units

Required:

```text
/etc/systemd/system/arixtv.service
/etc/systemd/system/arixtv-refresh.service
/etc/systemd/system/arixtv-refresh.timer
```

Optional when relay is installed:

```text
/etc/systemd/system/arixtv-relay.service
```

### arixtv.service

Responsibilities:
- HTTP serving;
- playlist serving;
- resolver requests;
- health/status;
- snapshot reload.

It does not run its own periodic provider scheduler.

### arixtv-refresh.service

Oneshot service that runs:

```text
/etc/arixtv/arixtv refresh
```

### arixtv-refresh.timer

Default concept:

```text
OnBootSec=2min
OnUnitActiveSec=6h
RandomizedDelaySec=15min
Persistent=true
```

User may choose another interval.

## 26. Process monitoring

Use systemd restart behavior.

Where practical, support systemd notify/watchdog so a stuck process can be restarted even if the process still exists.

The daemon should expose loopback-only health information.

## 27. Health/status

Local health should include:
- server state;
- channel count;
- enabled providers;
- healthy/stale/failed providers;
- last successful refresh;
- snapshot generation/version.

Target CLI:

```text
arixtv status
arixtv doctor
```

Public `/direct/` must not expose health details.

## 28. Refresh execution

Use systemd timer for scheduled refresh.

Manual refresh:

```text
arixtv refresh
arixtv refresh --provider <provider>
```

Prevent concurrent refresh runs using an OS lock/lockfile.

Phase 1 provides nonblocking kernel advisory locks on both Linux and Darwin,
with a persistent `state/refresh.lock` inode. A future coordinator holds it across
all scheduled/manual refresh work. Kernel locks release on close/process exit;
never delete the lock file or steal it based on PID/age. Per-provider snapshot
mutations and token mutations have separate locks to prevent lost updates.
Other platforms fail explicitly instead of pretending cross-process locking works.

Default refresh concurrency should be conservative for 1C1G:
- global: about 8;
- per-provider: about 2–4.

These should be configurable.

## 29. Refresh optimization

Do not blindly re-audit every historical channel every run.

Prefer work on:
- new channels;
- changed fingerprints;
- unknown channels;
- previously failed channels.

Reuse stable classification when provider metadata indicates no meaningful change.

## 30. CLI provider management

Target:

```text
arixtv providers list
arixtv providers enable <provider>
arixtv providers disable <provider>
arixtv providers test <provider>
arixtv refresh --provider <provider>
```

Provider list should display useful operational state.

## 31. Interactive Linux installer

Repository supplies:

```text
install.sh
```

The installer must:
- detect environment;
- detect architecture;
- detect distribution;
- detect systemd;
- detect interfaces and candidate LAN CIDRs;
- detect existing ArixTV installation;
- detect Nginx presence;
- detect required download/archive tools;
- create `/etc/arixtv/`;
- install the binary there;
- create config;
- create user/group;
- create systemd units;
- configure timer;
- allow provider selection;
- allow LAN configuration;
- allow remote domain configuration;
- optionally install relay backend;
- create management/uninstall scripts.

The installer should work without `dialog`, `whiptail`, or `fzf`.

## 32. Installation management menu

If ArixTV is already installed, the management flow should offer:

1. Reconfigure
2. Update ArixTV
3. Configure Providers
4. Configure LAN
5. Configure Remote Direct
6. Configure Relay
7. Change Refresh Schedule
8. Refresh Now
9. Restart Services
10. Show Status
11. Show Nginx Configuration
12. Uninstall

## 33. Reconfiguration safety

Before configuration changes:
- backup current config;
- validate new config;
- validate CIDRs/ports;
- only replace active config after validation.

After service-affecting changes:
- restart/reload;
- check health;
- roll back configuration if startup/health validation fails.

## 34. Complete uninstall

Uninstall must:
- stop/disable ArixTV units;
- stop/disable optional relay unit;
- remove ArixTV unit files;
- run systemd daemon reload/reset-failed;
- remove `/etc/arixtv/`;
- remove installer-created system user/group;
- remove installer-installed relay component.

Do not remove:
- Nginx;
- user TLS certificates;
- unrelated Nginx configuration;
- unrelated system packages.

Because uninstall script itself resides under `/etc/arixtv/`, use a safe self-removal strategy such as copying final cleanup logic to `/tmp`.

## 35. macOS arm64 behavior

The Go binary must run on Apple Silicon macOS.

Supported macOS V1 use:
- `arixtv version`;
- config validation;
- provider refresh/testing;
- M3U generation;
- HTTP serving;
- local functional development.

Linux-specific:
- `/etc/arixtv/` installer;
- systemd units/timers;
- Linux network/service management;
- Nginx deployment helper workflow.

Do not make the Go core depend on systemd.

## 36. Configuration

Target Linux path:

```text
/etc/arixtv/config.toml
```

Conceptual structure:

```toml
[storage]
data_dir = "./data"
state_dir = "./state"

[server]
loopback_address = "127.0.0.1"
port = 8787

[lan]
enabled = true
listen_addresses = ["192.168.50.10"]
public_origin = "http://192.168.50.10:8787"
allow_networks = ["192.168.50.0/24"]

[remote]
enabled = true
public_origin = "https://tv.example.com"
direct_path = "/direct"
trusted_proxies = ["127.0.0.1/32"]

[refresh]
interval = "6h"
global_concurrency = 8
provider_concurrency = 3

[providers]
enabled = ["xumo", "plex"]

[relay]
enabled = false
```

Do not store public cleartext token values in config. Storage paths are caller-
configured and portable; Linux deployment will supply `/etc/arixtv/data` and
`/etc/arixtv/state` later. Parsing/validation never creates directories or binds
ports. Omitted storage settings preserve compatibility with Phase 0 configs.

## 37. HTTP hardening

Configure:
- read header timeout;
- read timeout;
- write timeout;
- idle timeout;
- maximum header size;
- bounded concurrency/rate limiting.

Public surface needs only `GET`/`HEAD`.

Reject irrelevant methods.

Do not accept request bodies on resolver routes.

## 38. Logs and secrets

Never log:
- complete ArixTV token;
- complete relay credentials;
- cookies;
- Authorization values;
- provider session secrets;
- private keys;
- sensitive signed URL query values.

Nginx examples for `/direct/` and `/relay/` should default to secret-safe logging, such as disabling access logging until a redacted format is deliberately configured.

## 39. CI

CI uses Go 1.27.3 explicitly, with `GOTOOLCHAIN=local`. Future Phase 8 release
work must use that same baseline or an explicitly approved newer security baseline.
This baseline includes the crypto/x509 mitigation for CVE-2026-111943; see the
[Go release history](https://go.dev/doc/devel/release#go1.27.3).

Normal push/PR CI should run:
- `gofmt` check;
- `go vet ./...`;
- `go test ./...`;
- build validation.

Do not make normal CI depend on live third-party provider availability.
A separate job runs the official `golang.org/x/vuln/cmd/govulncheck@v1.9.0`
against the current Go vulnerability database. The scanner code/version and Go
compiler are pinned; the database intentionally updates. No `@latest` install is
used, and the scanner is not a production module dependency.

Provider tests should use:
- local HTTP test servers;
- sanitized fixtures.

Cross-build validation:
- `GOOS=linux GOARCH=amd64`
- `GOOS=linux GOARCH=arm64`
- `GOOS=darwin GOARCH=arm64`

## 40. Release CI/CD

Git tag `vX.Y.Z` creates release assets.

Required targets:
- linux/amd64;
- linux/arm64;
- darwin/arm64.

Release includes SHA256 checksums.

Embed in the binary:
- version;
- git commit;
- build date.

`arixtv version` displays them.

Installer/update logic downloads GitHub Release assets and validates SHA256.

## 41. Update and rollback

Linux management workflow should:
1. fetch release metadata;
2. select architecture asset;
3. download;
4. verify checksum;
5. preserve current binary;
6. atomically replace;
7. restart;
8. health check;
9. roll back if health check fails.

Never overwrite user config during update.

## 42. Test requirements

Core test areas:
- config parsing;
- CIDR validation;
- token generation;
- token validation;
- token/log redaction;
- M3U generation;
- provider normalization;
- atomic snapshot writes;
- failed refresh LKG behavior;
- resolver 302 behavior;
- provider failure isolation;
- direct prefix routing;
- trusted proxy behavior;
- LAN ACL;
- unauthorized 404;
- internal route loopback restriction.

## 43. Performance target

Primary small-server target:

```text
1 CPU
1 GB RAM
```

Design for low idle resource use.

Performance comes primarily from:
- bounded concurrency;
- HTTP connection reuse;
- incremental refresh;
- avoiding redundant validation;
- no EPG;
- no media data plane;
- no browser/runtime stack.

## 44. Third-party code and licensing

ArixTV is an independent implementation.

Do not copy FastChannels code unless license/permission is explicitly verified.

Use upstream projects to understand public behavior and protocol, then independently implement ArixTV's own code and tests.

Before bundling/copying any third-party component, verify its license.

Do not select the ArixTV repository license without owner approval.

## 45. V1 acceptance criteria

V1 should satisfy:

1. Go binary builds on linux/amd64, linux/arm64, darwin/arm64.
2. Linux interactive installer works.
3. Linux project files live under `/etc/arixtv/`.
4. systemd units live under `/etc/systemd/system/`.
5. Provider selection works.
6. Free FAST providers can refresh.
7. No EPG subsystem exists.
8. DRM channels do not enter supported playlist output.
9. Playlist entries use ArixTV resolvers.
10. Tune-time resolution returns fresh 302 URLs.
11. LAN CIDRs are configurable during setup.
12. LAN access requires CIDR + token.
13. Remote Direct uses `/direct/`.
14. Nginx config is generated, not automatically modified in V1.
15. Optional Relay uses `/relay/`.
16. Relay consumes local `relay.m3u`.
17. Relay does not fetch public ArixTV M3U.
18. Provider refresh failure preserves LKG.
19. Automatic refresh uses systemd timer.
20. Server crash is automatically restarted.
21. Concurrent refresh is prevented.
22. Reconfiguration validates and can roll back.
23. Update verifies checksums and can roll back.
24. Complete uninstall removes ArixTV-created files/services.
25. CI is green.
26. No browser, Redis, Node.js, FFmpeg, database-server dependency exists in ArixTV core.


## 46. Approved Phase 2 serving contract

Phase 2 adds `arixtv serve [--config PATH]`, using explicit loopback/LAN listener
policies and the Phase 1 schema version 1 stores. Corrupt/unsupported state is
preserved and fails closed; LKG, post-rename DurabilityError and trusted local
POSIX filesystem semantics remain unchanged. No real provider, refresh scheduler,
media proxy, generated relay playlist, TLS manager or installer is added.

Public paths are the configured Direct prefix plus `/s/<token>/playlist.m3u` and
`/s/<token>/play/<provider>/<channel>`. LAN uses socket IP + token, while Remote
Direct uses a loopback backend and mandatory token. Only trusted peers may supply
the documented single-hop Nginx IP/proto convention. Advertised URLs use explicit
LAN HTTP / remote HTTPS origins; incoming Host/forwarded-host values are ignored.
Both modes use the same Direct prefix. Strict paths never redirect/canonicalize
secret-bearing input and reject percent-escaped delimiters/aliases, dot/empty
components, backslashes and controls. Non-path-safe opaque channel IDs use a
canonical `~`+base64url component.

Public playlists include only clear_direct channels in deterministic order,
without EPG/XMLTV fields or upstream playback URLs, with no-store headers. Text is
written incrementally; no per-client full-playlist buffer or file write is needed.
Resolve validates the snapshot channel and returned ephemeral Stream and emits
only a zero-body 302 Location. Invalid auth/routes/channel classes are 404;
temporary provider failures/capacity errors are sanitized 503 with Retry-After 30.
Public routes accept only GET/HEAD without request bodies.

Health/internal paths are loopback-listener and loopback-peer only and reject
forwarding metadata. Internal authentication is a distinct injected validator,
using an `irv_` namespace separate from public tokens and disabled in the CLI until
Phase 7 secret setup. Internal resolution may select clear_direct or clear_relay_only;
protected/unknown content is excluded. No relay.m3u is generated in Phase 2.

Explicit HTTP timeouts, header limits, bounded per-IP rate buckets, a shared TCP
connection cap and nonqueued resolver admission bound resource use. Default
limits/config validation are documented in CONFIGURATION.md. Snapshot/observation
stamps are polled; completed valid generations replace an immutable catalog.
Failed runtime reloads keep LKG and degrade local health, without rewriting disk.
Aggregate catalog capacity is 100000 channels. Health reports safe counts/states,
generations and last-success timestamps, with stale age after two configured
refresh intervals; it does not initiate refresh or claim upstream playback health.

SIGINT/SIGTERM cancels contexts, stops listeners and applies a bounded common
Shutdown deadline. Linux optionally sends READY=1, WATCHDOG=1 and STOPPING=1 through
its service-manager socket; Darwin uses a no-op implementation. Final Linux unit
files remain Phase 5. Phase 3 real provider implementations require separate review.
