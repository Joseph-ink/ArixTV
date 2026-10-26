# ArixTV Provider Guide

## Purpose

Provider code is the most change-prone part of ArixTV.

Every provider must be independently testable and independently failure-tolerant.


## Primary provider reference

The initial ArixTV provider catalog is derived from systematic research of the current:

```text
https://github.com/kineticman/FastChannels
```

repository.

FastChannels should be treated as a provider-behavior and protocol reference, not as code to mechanically port.

Before implementing a provider, check:

```text
docs/FASTCHANNELS_MAPPING.md
```

The mapping should record whether the provider is:
- free FAST or paid/account-based;
- clear, mixed DRM, or DRM-only;
- direct-playable, proxy-required, or unknown;
- suitable for ArixTV V1, future work, or out of scope.


## Provider responsibilities

A provider performs two operations.

### Discover

Return normalized channel candidates.

Discovery may involve:
- public JSON APIs;
- anonymous sessions;
- provider-specific identifiers;
- region-specific catalogs.

### Resolve

At tune time, return a fresh upstream playback URL for one channel.

Do not assume a discovered URL remains valid indefinitely.

## Conceptual interface

```go
type Provider interface {
    Discover(ctx context.Context) ([]Channel, error)
    Resolve(ctx context.Context, ch Channel) (Stream, error)
}
```

Implementations may add internal helpers, but the public coordinator should remain small.

## Provider states

Suggested maturity states:
- `stable`
- `experimental`
- `disabled`
- `unsupported`

Suggested playback capability:
- `clear_direct`
- `clear_relay_only`
- `drm`
- `unsupported`

## V1 research pool

Phase 3A implements **Vizio WatchFree+ (`vizio`) only**, registered as experimental,
anonymous/free and independent of unofficial mirrors. See
[Vizio protocol/support notes](providers/VIZIO.md) and the
[Phase 3A report](PHASE3A_REPORT.md). The broader historical upstream inventory is
pinned to commit `7e61c1ef127f7119326eb98381726d6f7cfaa56d` in the mapping.
Its initial research priority is Vizio, Xumo and Plex; LG is a reserve candidate.
Zeam's IP/UA-bound playback, STIRR's proxy behavior, and VIDAA's signed-app
protocol require further review. The table below describes original intent;
the mapping is authoritative for current candidate decisions and limitations.

Providers to investigate:

| Provider | V1 intent |
|---|---|
| Xumo Play | candidate |
| Plex | candidate |
| Pluto TV | candidate |
| Vizio WatchFree+ | experimental implementation; live Resolve not certified |
| LG Channels | candidate |
| Samsung TV Plus | candidate |
| Roku Channel | candidate |
| Zeam | candidate |

Do not mark a provider stable merely because an older project supported it.

Verify current behavior during implementation.

## DRM and unsupported content

ArixTV may classify streams in order to exclude unsupported content.

If playback requires:
- Widevine;
- PlayReady;
- FairPlay;
- a DRM license exchange;
- protected browser/device playback;

the channel is not supported in V1.

Do not implement DRM circumvention.

## Provider failure behavior

On provider refresh error:

```text
refresh fails
    ↓
record error
    ↓
keep previous provider snapshot
    ↓
continue other providers
```

Do not replace a valid snapshot with empty data because of a temporary upstream error.

## Provider fixtures

Normal CI must not depend on provider uptime.

Use:
- local HTTP test servers;
- sanitized JSON/playlist fixtures.

Fixtures must not contain:
- real cookies;
- bearer tokens;
- signed credentials;
- personal account data;
- private keys.

If a fixture was recorded from a live service, sanitize it before commit.

## Rate limiting

Providers may rate limit or block abusive request patterns.

Use:
- shared HTTP transports;
- context deadlines;
- bounded concurrency;
- backoff where appropriate;
- conservative defaults.

Do not retry in tight loops.

## Incremental refresh

Avoid revalidating every unchanged historical channel.

Track provider/channel fingerprints when practical.

Focus validation on:
- new channels;
- changed channels;
- unknown capability;
- previously failed resolution.

## Provider documentation checklist

Before marking a provider stable, document:

1. discovery workflow;
2. required headers;
3. anonymous/session behavior;
4. region behavior;
5. stream format;
6. DRM/mixed-content behavior;
7. tune-time resolver behavior;
8. known expiration behavior;
9. rate-limit concerns;
10. tests and fixtures.

## Code provenance

Provider behavior may be learned from:
- public APIs;
- public network behavior;
- public documentation;
- compatible open-source projects.

Do not copy third-party implementation code without confirming license compatibility.

ArixTV provider code should be independently implemented and tested.


## Approved Phase 1 policy for later implementation

The owner policies in `ARIXTV_SPEC.md` govern implementation over historical
mapping questions: V1 accepts anonymous/free sources only; free-account sources
are excluded; unofficial mirrors are experimental/future by default; and embedded
application/authentication secrets cannot be copied or distributed. Standard HLS
AES-128 is classified separately from DRM. Same-egress/IP-bound clear channels
are always `clear_relay_only`, excluded from both normal LAN and Remote Direct
playlists. No exception is inferred just because a particular LAN shares egress.

The immutable `provider.Registry` now owns identity/status/access/mirror metadata,
separate from the two-method Discover/Resolve runtime. Phase 3 providers must
validate channel classification and honor the complete-discovery contract before
committing snapshots. See `PHASE1_REPORT.md` for the persistence and security API
contracts. No provider-specific implementation is part of Phase 1.
