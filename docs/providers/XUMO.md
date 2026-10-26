# Xumo Play (`xumo`)

Status: **experimental**, anonymous/free, first-party APIs, no unofficial mirror.
Phase 3B adds only Xumo. The production registry contains exactly `vizio` and
`xumo`; Vizio remains experimental with live tune-time playback unverified.

Xumo discovery retains the complete supported catalog response, but Direct
qualification currently covers **one independently checked reference identity**:
CBS News 24/7, channel `99991158`, callsign `XCBSNWS-CMS`. Other unverified channels
remain unknown. A successful single-channel live test is not stable certification.

## Current research and provenance

Inspected the current FastChannels default/development branch on **2026-10-27
(Asia/Hong_Kong)** at
[`376c1a69b8c11f6271e4557859fee75fe9ec5cb8`](https://github.com/kineticman/FastChannels/tree/376c1a69b8c11f6271e4557859fee75fe9ec5cb8).
Protocol references were
[`app/scrapers/xumo.py`](https://github.com/kineticman/FastChannels/blob/376c1a69b8c11f6271e4557859fee75fe9ec5cb8/app/scrapers/xumo.py),
[`app/scrapers/base.py`](https://github.com/kineticman/FastChannels/blob/376c1a69b8c11f6271e4557859fee75fe9ec5cb8/app/scrapers/base.py),
[`app/routes/play.py`](https://github.com/kineticman/FastChannels/blob/376c1a69b8c11f6271e4557859fee75fe9ec5cb8/app/routes/play.py),
[`app/worker.py`](https://github.com/kineticman/FastChannels/blob/376c1a69b8c11f6271e4557859fee75fe9ec5cb8/app/worker.py)
and README Sources / Source Notes. Xumo-related references also occur in
`app/scrapers/category_utils.py`, `app/data/gracenote_map.csv`,
`app/data/gracenote_exclusions.csv` and `app/data/station_index.json`. These mapping/category data are not imported into
ArixTV. No upstream code, tests, guide data or fixtures were copied/translated.

Upstream uses an opaque channel reference, resolves a current broadcast asset,
fetches provider sources, and supports a JSON-string playlist indirection. Its
browser headers, fixed geo value, ad macros, caches, retry behavior and guide
logic are not implementation requirements for ArixTV. ArixTV independently tests
the narrower anonymous protocol and fails closed on unsupported variants.

Independent first-party observations:

- The [market 10006 catalog](https://valencia-app-mds.xumo.com/v2/proxy/channels/list/10006.json?sort=hybrid)
  returned HTTP 200, 232,316 bytes, version 2 and 454 entries in `channel.item`.
  No fixed `geoId`, `deviceId`, advertising identifier or account token was needed
  in that request. The observed envelope is one complete response, not paginated.
- 453 entries declared `properties.is_live="true"`; one declared false. Eighteen
  callsigns carried DRM suffixes. The catalog has no explicit positive clear-DRM
  field; absence of a suffix is insufficient to qualify arbitrary channels.
- A current CBS News broadcast lookup returned one live SIMULCAST asset. A
  provider-only asset request supplied HLS from Google DAI. Its master and first
  media manifest were reachable without cookies or special playback headers;
  no key tags or end-list were observed in that sampled media manifest.
- The [official web entry point](https://play.xumo.com/) redirected to its
  [US-only geo-block page](https://play.xumo.com/geo-block) in this environment.
  API/CDN success does not establish unrestricted country or cross-egress access.

These are dated observations, not service stability or rights guarantees.

## Discovery and completeness

`Discover` makes exactly one GET to the public US market route, with
`Accept: application/json`, `User-Agent: ArixTV/experimental`, public web
`Origin: https://play.xumo.com` and `Referer: https://play.xumo.com/`. There is no
login, fixed geo override or device query identifier. Market 10006 is a public
route selector, not a credential. The two web context headers are public protocol
conventions, not authentication secrets. No additional TOML settings are required.

Only normalized identity, trimmed title, the first supplied genre/group, country
`US`, classification, last-seen and a SHA-256 fingerprint persist. IDs retain their
upstream numeric string representation. Language is not guessed. The current
catalog has no safe artwork field; no image URL is fabricated or image fetched.
The fingerprint covers normalized metadata, callsign and live/simulcast flags,
excluding last-seen, descriptions, unused properties and all ephemeral identities.
No playback URL, broadcast asset ID, raw payload or guide information persists.

Reads are capped at 2 MiB and 2,000 catalog rows; row allocation is bounded during
decoding. Missing/invalid required identities, names/callsigns, duplicate IDs,
empty sets, malformed JSON/types, truncated/oversized bodies, unknown/duplicate
root/container fields or unsupported version fail the **whole** discovery with
nil/error. Any new continuation/count/region envelope is rejected for protocol
review instead of being mistaken for completeness. Unknown row metadata or
properties disqualify the positive Direct profile; descriptions/opaque fields
are discarded, never modeled as programs. No guide, schedule or image requests
occur during discovery. A later coordinator must commit only successful complete
results; Phase 4 is not implemented.

## Conservative classification

The reference profile requires matching ID, callsign and normalized name,
`is_live="true"`, `is_simulcast="true"` and understood metadata. It is qualified
from the independent anonymous HLS observations and revalidated on every tune.
Reusing the ID for different content or adding unknown security-related metadata
removes qualification. This deliberately small profile is not an all-provider
clear-stream assumption or an account/DRM exception.

DRM-marked callsigns become `other_drm` / `drm`; the DRM system is not guessed.
Non-live entries are unsupported. Unverified entries remain unknown and do not
enter public Direct M3U output. At runtime, cookie-setting responses and explicit
IP-binding URL parameters yield `clear_relay_only`; special source headers,
unknown formats/protection fields, protected/unknown HLS keys and unsupported
renditions fail closed. No cookies or API web-context headers are replayed to a
player. Ordinary identity-key HLS AES-128 is `hls_standard_encrypted`; no keys are
fetched. Hidden server-side IP binding cannot be disproved by a single-egress
smoke test and remains a stable-promotion research question.

## Fresh resolution

Resolve first refetches the catalog and rechecks the current normalized channel.
It then requests only:

```text
/v2/channels/channel/<channel>/broadcast.json?hour=<UTC hour>
/v2/assets/asset/<current-live-asset>.json?f=providers
```

The broadcast request obtains strictly necessary current-playback identity and
validity metadata, not a guide or program listing. No `/epg/` request is made.
Exactly one unambiguous live asset with an active timestamp window is required;
there is no fallback to a scheduled/non-live or first arbitrary asset. The asset
response must match that identity and be SIMULCAST. Provider availability windows
and declared HLS source format are checked. Unknown protocol envelopes fail closed.

The observed `refresh.from/to` values can be **in the future for an already-live
asset**. They are treated as a polling/freshness hint, not the content's start
window. Ordered, positive hints with a future upper bound are required, and that
upper bound conservatively limits the result. Stream expiry is the earliest of
that ceiling, the live asset's end, provider sunset and recognized URL `exp` /
`expires` timestamps. These bounds do not promise the CDN URL survives until that
time; actual signed/session URL lifetime remains unmeasured.

ArixTV generates two independent 128-bit random, hexadecimal anonymous identifiers
using `crypto/rand`, held only for the provider instance's lifetime. They fill the
observed `[deviceID]` and `[PPID]` URL placeholders. `[LIMITED_AD_TRACKING]` is 1;
`[app_bundle]` is ArixTV's own `org.arixtv` identifier. These are anonymous public
query conventions, not app secrets, signing keys or account credentials. No
hardware identifier, stable cross-restart identity, cookie jar or secret storage
is involved. Provider formatting/logging redacts identity fields.

Only observed query placeholders are expanded; unknown macros fail closed.
Untouched signed/opaque query values are preserved byte-for-byte. In particular,
the observed ad-routing value contains a literal semicolon. Expiry checking scans
only recognized expiry parameters instead of treating the entire opaque query as
form data. Duplicate/malformed actual expiry parameters are still rejected.

One optional JSON-string pointer (8 KiB maximum) may precede at most two HLS
manifests (256 KiB each): one master and its first muxed media variant. The existing
pure inspector is shared with Vizio. Xumo returns that validated media-manifest
URL, omitting adaptive bitrate selection and optional subtitles. Separate audio/
video renditions, deeper nesting, unknown variables/protection and unsafe URLs
fail closed. Nothing rewrites manifests or fetches keys, segments or video bytes.
The approved HTTP server returns its bodyless 302, without persisting the URL.

## Networking and performance

One reused standard-library transport/client, normal certificate verification,
no environment proxy, no cookie jar and no retry loop. Production URLs and
redirects must be public HTTPS/443; resolved private/internal addresses are
rejected before dialing. API Origin/Referer are stripped on CDN/indirection hops,
including redirects. Cookies are observed only as a boolean restriction and never
replayed or logged. Errors are fixed safe codes/messages, not upstream bodies,
URLs, headers or transport/parser text.

Each operation shares an eight-second deadline and at most two active provider
operations, with no admission queue. Transport limits include four connections
per host, 16 idle total, four idle per host, explicit dial/TLS/header timeouts and
bounded response headers. At most two redirects per GET are followed. Resolve
uses three metadata GETs plus at most one pointer and two manifests, all under the
same operation deadline. There is no asset, URL, catalog or session cache, internal
scheduler, unbounded goroutine fan-out or per-channel discovery probing.

No published quota was established. 429/status/network failures stop immediately;
existing server rate/concurrency limits and sanitized 503/Retry-After apply.
This design trades a fresh catalog GET per tune for avoiding stale URLs, within
the 1 CPU / 1 GB target. Cache/freshness changes require later explicit review.

## Offline and opt-in live validation

Normal tests use hand-authored, manually reviewed synthetic fixtures and local
TLS servers. They test completeness, classification, metadata omissions, identity,
size/time/cancellation/resource bounds, fresh lookup, expiry, pointers, HLS,
unsafe destinations/redirects, TLS verification, header isolation and secret-safe
errors. The server test proves actual Xumo runtime → temporary snapshot commit →
catalog reload → protected M3U → bodyless 302, with Vizio also in the registry.
Existing Vizio resolver/server tests pass unchanged. No live internet is used by
normal tests.

Live validation requires **both** the integration tag and explicit environment gate:

```sh
GOTOOLCHAIN=local ARIXTV_XUMO_LIVE=1 go test -tags=integration \
  ./internal/provider/xumo -run '^TestLiveXumo$' -count=1 -v
```

It discovers once and resolves only the evidence-qualified reference, without
trying unknown fallback channels, keys or segments. It prints safe counts and
fixed diagnostics, never playback URLs, query values or identities. Without the
environment gate it skips even when built with the tag. Standard CI does not
supply either live opt-in.

Phase 3B's final live run **passed** in 1.48 seconds: 454 channels, one qualified
Direct, 18 DRM-marked, 435 other/unknown (including non-live entries), followed by
a fresh validated HLS media URL without special playback headers or cookies.
The two earlier smoke failures identified the future-refresh and opaque-query
interpretations described above. Both now have offline regression coverage; a
bounded diagnostic trace isolated the second failure and was removed afterward.
There was no retry loop or Vizio live debugging. No raw captures entered the repo.

One channel on one egress does not establish repeated reliability, other channels,
all countries, no hidden IP binding or actual expiry behavior. Keep Xumo
experimental. Provider service terms and ArixTV distribution-license selection
remain owner review items; public API access is not a redistribution license.
