# AGENTS.md — ArixTV Repository Instructions

## Project identity

- Project name: **ArixTV**
- Binary name: `arixtv`
- Primary language: Go
- Minimum Go / CI / release baseline: **1.27.3** (no older compatibility target).
- Primary production target: Linux
- Required binary targets:
  - `linux/amd64`
  - `linux/arm64`
  - `darwin/arm64`

ArixTV is a lightweight FAST/free-IPTV channel aggregator and M3U resolver. It discovers supported free live-TV channels, excludes DRM/unsupported streams, generates protected M3U playlists, and resolves fresh upstream playback URLs with HTTP 302 redirects.

## Primary upstream research reference

ArixTV originated from the idea of extracting and redesigning the free FAST/IPTV discovery and resolver capabilities found in:

```text
https://github.com/kineticman/FastChannels
```

FastChannels is the primary upstream research reference for ArixTV's initial provider catalog and provider behavior.

Codex/developers should systematically inspect the current FastChannels repository to understand:

- which free FAST/IPTV providers are implemented;
- which sources are paid/account-based and therefore outside V1;
- how channel discovery works;
- provider API/session initialization;
- required headers/cookies when applicable;
- tune-time stream resolution;
- region behavior;
- clear versus DRM/mixed-stream behavior;
- whether playback is direct or requires manifest/segment proxying;
- whether browser/device bridge behavior is required.

ArixTV MUST NOT mechanically translate, line-by-line port, or blindly copy FastChannels source code.

The correct process is:

1. inspect current FastChannels behavior and provider structure;
2. document the provider protocol/behavior;
3. classify suitability for ArixTV;
4. independently implement the provider in Go;
5. add ArixTV-native tests and sanitized fixtures.

Where upstream license/permission is unclear, treat FastChannels strictly as a behavioral/protocol research reference.

The authoritative migration inventory lives in:

```text
docs/FASTCHANNELS_MAPPING.md
```

That document must be updated whenever provider support changes upstream or in ArixTV.

## Product boundaries

ArixTV core MUST remain small and focused.

ArixTV core DOES:
- discover channels from provider APIs;
- normalize channel metadata;
- classify supported clear streams;
- maintain provider snapshots;
- generate M3U playlists;
- authenticate playlist/play requests;
- resolve fresh playback URLs;
- return HTTP 302 redirects;
- expose local health/status information;
- support LAN and reverse-proxy deployment.

ArixTV core DOES NOT:
- implement EPG or XMLTV;
- bypass DRM;
- implement Widevine, PlayReady, FairPlay, or browser DRM bridges;
- transcode;
- record/DVR;
- proxy or restream video;
- include a Web UI;
- depend on Redis, PostgreSQL, MySQL, Chromium, Playwright, Node.js, or FFmpeg;
- manage TLS certificates or ACME;
- manage VPN software.

Optional video relay is an external, replaceable component.

## Architecture rules

1. Prefer the Go standard library.
2. Use small, explicit packages. Avoid framework-heavy architecture.
3. Provider implementations MUST be isolated behind a provider interface.
4. One broken provider MUST NOT break refresh for other providers.
5. Refresh failures MUST preserve the provider's last known good snapshot.
6. M3U and provider snapshots MUST be written atomically.
7. Public playlist/play endpoints MUST require strong random tokens.
8. Unauthorized public requests SHOULD return `404` rather than reveal auth state.
9. Internal relay resolver endpoints MUST be loopback-only.
10. ArixTV MUST NOT persist expiring upstream playback URLs as permanent playlist entries.
11. M3U entries MUST point to ArixTV resolvers; the resolver obtains a fresh upstream URL at tune time.
12. Platform-specific deployment code MUST remain separate from portable Go core logic.
13. Linux systemd/Nginx assumptions MUST NOT leak into core packages.
14. macOS arm64 MUST support development, refresh, M3U generation, provider testing, and HTTP serving. Do not add launchd management in V1.

## Installation layout

Linux deployment created by the installer MUST use:

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

Systemd units MUST live in:

```text
/etc/systemd/system/
```

Expected units:
- `arixtv.service`
- `arixtv-refresh.service`
- `arixtv-refresh.timer`
- optional `arixtv-relay.service`

Do not install the ArixTV binary into `/usr/bin`, `/usr/local/bin`, or `/opt`.

## Network model

Supported V1 deployment models:

### LAN Direct
Player → HTTP → ArixTV → 302 → upstream CDN

- User selects allowed LAN CIDR(s) during setup.
- LAN access requires both:
  - source IP inside configured CIDR(s);
  - valid token.
- Prefer binding explicit LAN interface addresses plus loopback instead of `0.0.0.0`.

### Remote Direct
Player → HTTPS Nginx → HTTP loopback ArixTV → 302 → upstream CDN

- Nginx terminates TLS.
- Remote direct path is `/direct/`.
- ArixTV listens on `127.0.0.1` for reverse-proxy traffic.
- Only configured trusted proxies may supply trusted forwarded headers.

### Remote Relay
Player → HTTPS Nginx → external IPTV proxy → local ArixTV resolver → upstream

- Nginx path is `/relay/`.
- ArixTV does not implement the relay engine.
- Relay consumes a local ArixTV-generated `relay.m3u`.
- Relay backend is replaceable and not part of ArixTV core.

## Provider policy

V1 is anonymous/free FAST only. Free-account and paid-account providers remain
out of scope. Stable providers should use provider-owned/public APIs; unofficial
mirrors are experimental/future by default. Ordinary HLS AES-128 is not itself DRM.
Clear IP-bound/session-bound streams or streams requiring special client headers
are `clear_relay_only` and never enter LAN/Remote Direct playlists. Do not copy
embedded application secrets, authentication secrets or private keys; anonymous
protocols may use non-secret public IDs/constants and locally generated identities.
See `ARIXTV_SPEC.md` for the authoritative owner policies.

Initial candidates include:
- Pluto TV
- Xumo Play
- Plex
- Roku Channel
- Samsung TV Plus
- LG Channels
- Vizio WatchFree+
- Zeam

Do not assume all candidate providers are currently compatible. Verify each implementation against current public behavior and document status.

Provider status values should support concepts such as:
- stable
- experimental
- disabled
- unsupported

Playback capability should distinguish:
- `clear_direct`
- `clear_relay_only`
- `drm`
- `unsupported`

V1 public playlist should normally contain only `clear_direct`.

## No DRM bypass

Never implement or assist with:
- extracting or bypassing DRM keys;
- Widevine/PlayReady/FairPlay circumvention;
- browser/device DRM bridge intended to evade content protection;
- converting protected streams into unprotected output.

If a provider/stream requires DRM, mark it unsupported for V1.

## Refresh and state rules

- Use bounded concurrency.
- Reuse HTTP transports and keep-alive connections.
- Do not launch unbounded goroutines.
- Default global refresh concurrency should be conservative (for example 8).
- Default per-provider concurrency should be conservative (for example 2–4).
- Avoid re-validating unchanged historical channels unnecessarily.
- Refresh must be single-instance; use a lock.
- Refresh success replaces snapshot atomically.
- Refresh failure preserves previous snapshot.
- Generated M3U updates must be atomic.
- The long-running server should reload completed snapshots without requiring a full service restart.

## systemd responsibilities

Use systemd rather than recreating OS functionality.

`arixtv.service`:
- serves HTTP;
- serves playlists;
- resolves playback;
- provides health/status;
- reloads snapshots.

`arixtv-refresh.service`:
- oneshot;
- executes refresh.

`arixtv-refresh.timer`:
- controls scheduled refresh;
- should use `Persistent=true`;
- should include randomized delay to avoid predictable provider bursts.

Use restart policy and, when practical, systemd watchdog notification for the daemon.

## CLI expectations

Target CLI:

```text
arixtv version
arixtv config validate
arixtv setup
arixtv serve
arixtv status
arixtv doctor
arixtv refresh
arixtv refresh --provider <id>

arixtv providers list
arixtv providers enable <id>
arixtv providers disable <id>
arixtv providers test <id>

arixtv token list
arixtv token create <name>
arixtv token rotate <name>
arixtv token revoke <name>
```

Do not implement commands merely to satisfy naming if the underlying feature is not ready; stage them according to `PLANS.md`.

## Security and repository hygiene

NEVER commit:
- real access tokens;
- provider credentials;
- cookies;
- authorization headers;
- API keys;
- GitHub PATs;
- private keys;
- TLS private keys;
- SSH private keys;
- real IPTV subscription URLs;
- provider session data;
- unsanitized captured provider responses.

Documentation and tests MUST use placeholders.

Runtime state, generated playlists, local configuration, provider snapshots, and private captures MUST not be committed unless they are explicitly sanitized fixtures.

Before commits:
1. inspect `git diff`;
2. search for secrets;
3. ensure examples contain placeholders only.

Logs MUST redact:
- ArixTV tokens;
- relay credentials;
- provider cookies;
- provider bearer tokens;
- signed upstream URLs when they contain sensitive query values.

## Third-party code and licensing

ArixTV is a new project.

Do not translate FastChannels source line-by-line.

When studying an upstream project:
1. understand public behavior/protocol;
2. document the behavior;
3. independently implement it;
4. use original tests/fixtures created for ArixTV.

Before copying any third-party code, verify license compatibility and attribution requirements.

Do not choose ArixTV's own license without explicit owner approval.

## Coding standards

Before considering a task complete:

```bash
gofmt -w .
go vet ./...
go test ./...
```

CI should enforce formatting and tests.

Prefer:
- clear error wrapping;
- context-aware network operations;
- explicit timeouts;
- deterministic tests;
- local mock servers instead of live-provider CI tests.

Avoid:
- global mutable state;
- hidden background goroutines;
- sleeps in tests;
- live internet dependency in normal CI;
- huge files containing unrelated responsibilities.

## Working process for Codex

1. Read `ARIXTV_SPEC.md`.
2. Read `PLANS.md`.
3. Work only on the current phase unless explicitly instructed otherwise.
4. Keep `PLANS.md` updated with completed items and discovered blockers.
5. Do not silently broaden product scope.
6. If a current upstream provider behavior has changed, document the change rather than bending the architecture around undocumented assumptions.
7. Preserve backward-compatible configuration unless a migration is explicitly planned.
8. Add or update tests with every behavior change.
