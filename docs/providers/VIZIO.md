# Vizio WatchFree+ (`vizio`)

Status: **experimental**, anonymous/free, provider-owned catalog, no unofficial
mirror. This is the sole Phase 3A real provider. Live catalog access succeeded;
live tune-time playback is **not certified**. No account or proprietary secret is
used. ArixTV source/tests were independently authored, not ported from FastChannels.

## Evidence and current behavior

Rechecked the FastChannels default branch `development` at
[`40f5903cb43979e00e8d6cc7186d855c93eed6b1`](https://github.com/kineticman/FastChannels/tree/40f5903cb43979e00e8d6cc7186d855c93eed6b1)
on 2026-10-26. Relevant behavioral references:

- [`app/scrapers/vizio.py`](https://github.com/kineticman/FastChannels/blob/40f5903cb43979e00e8d6cc7186d855c93eed6b1/app/scrapers/vizio.py): anonymous catalog, ad placeholders, token-gated exclusion, rotating FEATURED identities.
- [`app/scrapers/base.py`](https://github.com/kineticman/FastChannels/blob/40f5903cb43979e00e8d6cc7186d855c93eed6b1/app/scrapers/base.py): inherited resolver returns stored URL unchanged.
- [`README.md`, Sources / Source Notes](https://github.com/kineticman/FastChannels/blob/40f5903cb43979e00e8d6cc7186d855c93eed6b1/README.md): free source, public API/HLS description.

Independent requests to the first-party
[channel API](https://watchfreeplus-epg-prod.smartcasttv.com/api/channels)
returned HTTP 200 anonymously, 2,328,281 bytes and **472 channels**. The current
response is one `channels` array, not paginated. `channelIds` was an empty UI list.
Observed state IDs: 464 enabled TV/mobile (2), seven grayed-out (3), one TV-only
(0). Every row had one URL; 471 URL paths ended in `.m3u8`. NFL
`118NFLDigitalChannel1` declared both `licenseUrl` and `tokenUrl`; its actual DRM
system was not established or contacted. Other rows had explicit empty protection
fields. These observations strengthen the old source-only inventory; upstream's
approximate 425 count is not current.

The API accepted `Accept: application/json` and `User-Agent: ArixTV/experimental`,
without Authorization, cookies, Origin or Referer. FastChannels uses a different
mobile User-Agent; ArixTV does not impersonate that app. The only production API
constant is the public endpoint. Catalog-provided query values are transient;
none are embedded in ArixTV or retained in snapshots/fixtures.

The catalog describes the US mobile lineup. ArixTV sets country `US` as catalog
market, leaves language unset rather than guessing, and offers no country/zip/IP
spoofing or account configuration. API access from this environment is not proof
that every CDN permits every country or a different client egress.

## Discovery and persistence

`Discover` does one catalog GET, with **no manifest probes**, images, guide calls
or program parsing. It decodes only channel identity/name/artwork/category,
protection flags, mobile status/windows and current playback references. Unknown
row fields are ignored, never stored. The entire operation has an eight-second
context deadline; the decoded body is capped at 4 MiB and 2,000 channels.

Missing identities/names/URL arrays, malformed types/JSON/dates, duplicate IDs,
empty sets, continuation indicators, inconsistent counts/ID sets, duplicate or
unrecognized envelope keys and oversized responses return **nil plus an error**.
There is no partial success. A future coordinator must commit only successful,
complete results; Phase 3A does not implement that coordinator.

Snapshots contain normalized channel fields and a SHA-256 fingerprint. The
fingerprint covers normalized metadata/classification and playback origin/path;
it excludes all playback query values, last-seen time and unused provider data.
Rotating ad/signature values and guide fields do not churn fingerprints. Names
and delivery paths do; this also notices changes in reused FEATURED slots. Artwork
with credentials, query values, fragments or unsuitable URLs is omitted. No
playback URL, cookie, token, raw payload, program or guide identifier is persisted.

## Classification

`clear_direct` is a **catalog-qualified candidate**, requiring all three explicit
empty fields (`licenseUrl`, `licenseToken`, `tokenUrl`), FAST type, mobile state 2,
an active supplied mobile window, a supported HTTPS HLS reference and recognized
query placeholders. It is not assigned solely from a `.m3u8` suffix or upstream
README. Missing protection data, unrecognized macros/formats and uncertain mobile
states are `unknown`; inactive windows are `unsupported`. Nonempty license data
is `other_drm` / `drm`; token-gated entries without license evidence are
`unsupported`. Neither is resolved. Explicit IP-binding query parameters produce
`clear_relay_only`.

This metadata qualification is deliberately experimental, **not a full stream
or cross-egress audit**. Missing/hidden DRM or session restrictions can exist.
Resolve rechecks current protection metadata and the selected manifest before
returning a URL. Protected or uncertain manifests fail closed; clear responses
setting cookies (even at an intermediate redirect) or explicit IP-bound URLs are
relay-only and are rejected by the existing public server. No special headers or
cookies are forwarded to a player. Absence of cookies/IP markers cannot prove
absence of all server-side IP binding; wider live validation is needed before
stable status. Ordinary identity-key HLS AES-128 is separately classified
`hls_standard_encrypted`; keys are never requested by ArixTV.

## Fresh resolution and resource limits

Every Resolve refetches the catalog by stable channel ID. It never uses a stored
playback URL and does not cache signed URLs. Ad placeholders are substituted only
in decoded query values, preserving untouched signed values byte-for-byte.
ArixTV uses its own public client name, a zero advertising ID, do-not-track/limited
tracking values and empty unsupported consent/ad targeting context. It does not
copy app credentials, secrets, device identity or cookies. Unknown macros fail
closed. These conservative defaults can reduce CDN compatibility.

Resolution inspects at most **two manifests**, each at most 256 KiB: an HLS media
manifest directly, or one master and its first muxed media variant. It returns
the validated **media-manifest URL**, sacrificing adaptive bitrate and optional
subtitles to avoid claiming unchecked variants are supported. Separate audio/video
renditions, deeper masters, HLS variable substitution and protected/unknown key
methods fail closed. No rewriting, segment requests, key retrieval or playback
buffering occurs. The existing HTTP server emits its empty-body 302.

Recognized `exp`/`expires` Unix timestamps must be valid and future; the earliest
encountered expiry is returned. Other opaque/session URL lifetimes are **unknown**,
not assumed permanent. There was no lifetime/expiry soak test.

A provider instance reuses one standard-library transport, with at most two active
Discover/Resolve operations, no waiting work queue or retries. Each operation's
whole catalog/manifest sequence shares an eight-second deadline, within the
server's default ten-second resolver limit. Transport caps: four connections per
host, 16 total idle connections, four idle per host; explicit dial/TLS/header/idle
limits. Only HTTPS/443 public destinations are accepted, including redirects;
production dialing checks resolved IPs before connecting and retains normal TLS
certificate/hostname verification. No environment proxy, cookie jar or custom CA
bypass is used. At most two redirects per GET are followed. Boundaries are injected
only inside offline tests, not exposed as production config knobs.

No published quota or API stability guarantee was established. Upstream 429/403,
network failures and slow catalogs are possible; ArixTV returns fixed safe errors,
never upstream bodies/URLs. Existing server admission limits and sanitized 503 /
Retry-After behavior apply. No retry loop, internal scheduler or catalog cache is
added. A catalog fetch per tune is a deliberate freshness/bandwidth tradeoff; any
future cache needs an explicit freshness contract.

## Validation and opt-in live smoke

Normal `go test ./...` uses synthetic reviewed fixtures and local TLS test servers.
The compatibility test uses the actual Vizio runtime through snapshot persistence,
server catalog, protected M3U and bodyless 302; no production server redesign.

Live tests require **both** an integration build tag and explicit environment opt-in:

```sh
ARIXTV_VIZIO_LIVE=1 go test -tags=integration ./internal/provider/vizio \
  -run '^TestLiveVizio$' -count=1 -v
```

Optionally set `ARIXTV_VIZIO_CHANNEL` to a catalog channel ID. The default is the
Bloomberg reference `98wurl_bloomberg_quicktake_3`. A run does one Discover and one
Resolve (one more catalog GET and at most two manifests, plus bounded redirects).
It prints counts/fixed diagnostics only, never URLs or query values. No segment,
key, account or DRM request is made. Normal CI never enables this test.

During Phase 3A, independent three-channel headerless/queryless sample probes
returned one HLS master (HTTP 200), HTTP 400 and HTTP 403. Those probes establish
neither all-channel compatibility nor production macro compatibility. The first
Go live run discovered 472 channels (429 catalog-qualified Direct, one protected,
42 other/unknown), then rejected optional subtitles too conservatively. The
inspector was corrected and a synthetic regression test added. The second run
hit the eight-second deadline fetching the catalog. **Live Resolve remains
unverified**; the failed smoke attempts are not counted as offline test success.
No further live retry loop was run. Region, CDN, privacy macro compatibility,
cross-egress behavior and URL lifetime require further low-volume operator testing.

Public API access and upstream visibility do not themselves establish service
terms or redistribution rights. No upstream license was found and no upstream
code was copied; ArixTV's own license still requires owner selection.
