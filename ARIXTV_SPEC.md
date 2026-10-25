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
    ID() string
    Name() string
    Discover(ctx context.Context) ([]Channel, error)
    Resolve(ctx context.Context, ch Channel) (Stream, error)
}
```

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

The normal V1 public playlist should contain `clear_direct` only.

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

Sensitive provider session information must not be written into channel snapshots unless absolutely necessary and safely protected.

## 10. Stream model

Tune-time resolution should return enough information to represent:

```text
URL
stream type
DRM status
expiry if known
```

The public playlist must not permanently embed provider URLs that are signed or short-lived.

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
- persist token hashes instead of cleartext where practical;
- show cleartext token only when created/rotated;
- never log full token.

Target CLI:

```text
arixtv token list
arixtv token create <name>
arixtv token rotate <name>
arixtv token revoke <name>
```

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
[server]
loopback_address = "127.0.0.1"
port = 8787

[lan]
enabled = true
listen_addresses = ["192.168.50.10"]
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

Do not store public cleartext token values in normal config if avoidable.

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

Normal push/PR CI should run:
- `gofmt` check;
- `go vet ./...`;
- `go test ./...`;
- build validation.

Do not make normal CI depend on live third-party provider availability.

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
