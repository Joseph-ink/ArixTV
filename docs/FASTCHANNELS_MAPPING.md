# FastChannels → ArixTV Provider Mapping

## Inspection provenance and limits

Inspected on **2026-10-26 (Asia/Hong_Kong)** from the current upstream default
branch **development**, commit
[`7e61c1ef127f7119326eb98381726d6f7cfaa56d`](https://github.com/kineticman/FastChannels/tree/7e61c1ef127f7119326eb98381726d6f7cfaa56d)
(upstream commit date 2026-10-25; release subject 5.7.2).
The source checkout used for research stays outside ArixTV. No upstream code,
provider credentials, session responses, or media captures are included here.

Inventory method: inspect the README source groups, statically enumerate
`BaseScraper` subclasses and `source_name` attributes, follow registry aliases
and runtime playlist registration, expand `free_networks.NETWORKS`, then read
channel discovery, resolver, and relevant playback-route behavior. This finds
**52 built-in source identities**, plus **one dynamic custom-playlist source
family**. Free Networks expands into **35 network adapters** (24 dedicated
modules plus 11 static groups), listed below. These are source/adaptor counts,
not channel counts. Aliases are not additional providers.

This is **source inspection**, not a live-provider compatibility certification.
No account login, protected playback, provider API probing, or CDN playback test
was performed. “Direct candidate” is an inference from a URL resolver and its
upstream routing, not proof that every channel works, every player works, or
remote clients can use a server-issued URL. Unknown fields are intentional.
All ArixTV sources are **not implemented** in Phase 0; no stable/experimental
provider support is claimed. Registry-enforced status/capabilities begin later.

## Classification policy

- **V1 candidate**: anonymous free service, or explicitly separated anonymous
  free subset, with a plausible clear direct URL path. Actual acceptance still
  requires manifest/segment checks, region tests, expiry tests, and sanitized
  ArixTV-native fixtures in Phase 3.
- **Future candidate**: unresolved protocol/rights issue, or clear playback that
  requires an external relay. ArixTV core must stay a 302 resolver.
- **Out of scope**: account-based, subscription/TVE, DRM/bridge-only, or outside
  the specified FAST provider product. Free accounts are still account-based.
- **Mixed** means clear and protected possibilities exist. A source merely
  labeling a URL `hls` does not prove it is clear. “Clear path inferred” is
  weaker evidence than an explicit non-DRM field or upstream playback finding.
- Ordinary HLS AES-128 is not itself Widevine/PlayReady/FairPlay. It is recorded
  separately; authenticated keys, license exchanges or media rewriting cannot
  be smuggled into the direct resolver. Owner review must settle the initial
  AES-128 acceptance policy before Phase 3. No decryption/key retrieval is
  implemented here.
- Discovery endpoints with names such as `epg`, `schedulelist`, or `broadcast`
  sometimes also contain channel metadata or the current playback asset ID.
  Reading those fields in a future discovery/resolver does **not** authorize an
  ArixTV program/schedule model or XMLTV subsystem.

## Complete top-level source inventory

Each row's source link is immutable at the inspected commit. Shared playback
routing evidence is linked in the implementation discrepancies section.

| Provider / upstream ID | Access model | Discovery workflow | Tune-time behavior | Stream | DRM | Delivery | Region/session considerations | ArixTV decision | ArixTV status | Evidence |
|---|---|---|---|---|---|---|---|---|---|---|
| Pluto TV (`pluto`) | Free; optional account | Country-specific bootstrap and channel catalog | Anonymous device/session bootstrap; issue fresh JWT stitcher HLS URL | HLS | Clear path; full catalog unknown | Direct candidate | Country feeds; rotating anonymous device sessions; short-lived JWT | V1 candidate (anonymous clear only) | Not implemented | [pluto.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/pluto.py) |
| Xumo Play (`xumo`) | Anonymous/free | First-party Valencia market 10006 catalog; 454 entries observed without fixed geo/device query values | Fresh catalog check → current live broadcast asset → providers-only source → bounded optional JSON pointer/HLS validation | HLS for qualified reference; other formats unknown | Mixed/unknown: 18 DRM-marked callsigns; only independently verified reference qualified Direct | Reference clear Direct; observed cookies/explicit IP binding relay-only; unverified channels unknown | US-only website; API web context headers; generated ephemeral query identities; future refresh hint is not content start; opaque URL lifetime/cross-egress behavior unproven | V1 experimental, narrowly qualified reference only | Phase 3B implementation; offline server reload/302 and final single-channel live smoke pass | [current xumo.py](https://github.com/kineticman/FastChannels/blob/376c1a69b8c11f6271e4557859fee75fe9ec5cb8/app/scrapers/xumo.py), [public catalog](https://valencia-app-mds.xumo.com/v2/proxy/channels/list/10006.json?sort=hybrid), [evidence/limits](providers/XUMO.md) |
| Plex (`plex`) | Free; anonymous token | Anonymous auth then XML channel lineup | Best-effort tune; part-manifest request follows redirect to fresh MediaTailor URL | HLS | Clear path; full catalog unknown | Direct candidate | Anonymous Plex token, client/playback IDs; refresh rejected tokens; region-specific lineup | V1 candidate | Not implemented | [plex.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/plex.py) |
| The Roku Channel (`roku`) | Free; anonymous session | Session cookies; EPG collections, billboards and content API fallback | Playback API supplies fresh HLS; separate DASH path carries DRM | Mixed HLS/DASH | Mixed | Direct for clear HLS; bridge for DRM | Cloudflare-sensitive; geo/session cookies and expiring playback URLs | V1 candidate (clear only; higher operational risk) | Not implemented | [roku.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/roku.py) |
| Samsung TV Plus (`samsung`) | Free | Third-party gzipped per-region channel mirror; skip license_url entries | Follow short-link redirects to final CDN with appropriate request UA | HLS selected | Mixed catalog; licensed entries excluded | Direct candidate | Configurable regions; mirror and redirect service are extra dependencies | V1 candidate (clear only; mirror review) | Not implemented | [samsung.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/samsung.py) |
| LG Channels (`lg-channels`) | Free | Public schedulelist API contains channel metadata and mediaStaticUrl | Expand URL macros at tune time | HLS | Unknown catalog-wide; clear path inferred | Direct candidate | Country code and anonymous ad/device macros; verify media does not require API headers | V1 candidate (after clear-stream validation) | Not implemented | [lg_channels.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/lg_channels.py) |
| Vizio WatchFree+ (`vizio`) | Anonymous/free | First-party single-response catalog; explicit license/token fields and mobile eligibility | FastChannels still expands discovery URLs and inherits unchanged resolver; ArixTV refetches catalog and validates bounded HLS at tune time | HLS for selected references | Mixed: 1 explicit licensed/token-gated entry among 472 observed; missing protection fields remain unknown | Catalog-qualified Direct candidates; cookies/explicit IP binding relay-only; live Resolve not certified | US mobile market; no account/session bootstrap; URL lifetime and cross-egress behavior unproven; CDN errors observed | V1 experimental candidate, excluded protected/unknown paths | Experimental implementation, Phase 3A; offline server integration passes | [current vizio.py](https://github.com/kineticman/FastChannels/blob/40f5903cb43979e00e8d6cc7186d855c93eed6b1/app/scrapers/vizio.py), [first-party API](https://watchfreeplus-epg-prod.smartcasttv.com/api/channels), [current evidence/limits](providers/VIZIO.md) |
| Zeam (`zeam`) | Free | All Channels JSON embedded in live page; nearby API is only a subset | StreamInfo returns fresh short-lived signed master | HLS; ordinary AES-128 reported | Clear of DRM per upstream; AES-128 encrypted | Direct only with shared public egress; relay needed across IPs | Token IP-bound; variant/key access tied to latest master-fetch UA; do not prefetch after client | Future candidate for general deployment; conditional LAN evaluation | Not implemented | [zeam.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/zeam.py) |
| TCL TV+ (`tcl`) | Free | Country livetab → category programlist | format-stream-url API with bundle/device/source metadata | HLS | Unknown catalog-wide; clear path inferred | Mixed: direct paths; upstream proxies Publica manifests/segments | Country/device ID; Publica proxy is documented as a browser CORS workaround, not proof native clients require relay | V1 candidate for proven direct subset; proxy subset future pending native-client test | Not implemented | [tcl.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/tcl.py) |
| Whale TV+ (`whale`) | Free | rlaxx/Zeasn browser category/channel API | Re-fetch stale channel URL list; substitute fresh macros | HLS | Unknown catalog-wide; clear path inferred | Direct candidate | Browser/device/region headers and expiring channel URL cache | V1 candidate (validate clear paths) | Not implemented | [whale.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/whale.py) |
| STIRR (`stirr`) | Free | Public channel-list API | POST playable endpoint; upstream routes result through manifest proxies | HLS | Unknown catalog-wide; clear path inferred | Proxy-required in upstream | IP-bound SSAI tokens; some origins need special TLS handling and segment proxy | Future candidate through external relay only | Not implemented | [stirr.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/stirr.py) |
| FreeLiveSports (`freelivesports`) | Free | Public channel JSON (shared listing contains schedule data) | Substitute timestamp, UA and ad macros in channel URL | HLS | Unknown catalog-wide; clear path inferred | Direct candidate | US lineup; macro freshness; verify any required media headers | V1 candidate | Not implemented | [freelivesports.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/freelivesports.py) |
| Bally Sports Live (`ballysports`) | Free | Channel metadata in Next.js RSC page payload | Resolve stored host/path identity to public CDN URL | HLS | Clear path per upstream | Direct candidate | US sports lineup; public tokenless CDN; channel hostname varies | V1 candidate | Not implemented | [ballysports.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/ballysports.py) |
| Hallmark free channels (`hallmark`) | Free FAST; distinct from paid Hallmark service | Small fixed free-channel catalog | Map channel identity to stable unauthenticated Akamai HLS | HLS | Clear path per upstream | Direct candidate | US free feeds; no paid Hallmark+ account used | V1 candidate | Not implemented | [hallmark.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/hallmark.py) |
| Adult Swim (`adultswim`) | Free marathon feeds | Embedded Redux/Next.js stream catalog; fetch asset metadata | Map opaque channel reference to stable asset HLS | HLS | Unknown catalog-wide; clear path inferred | Direct candidate | Marathons differ from cable network; availability may vary by region | V1 candidate | Not implemented | [adultswim.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/adultswim.py) |
| Sling Freestream (`sling`) | Free anonymous mode; optional paid account | Anonymous device/catalog or signed-in premium lineup | Channel playback session returns protected manifest | DASH | DRM-only for generic clients | Browser/device DRM bridge | Free/paid mode; paid browser login and bearer session | Out of scope (including free DRM feeds) | Not implemented | [sling.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/sling.py) |
| Local Now (`localnow`) | Free subset; locked items filtered | Homepage runtime configuration → live/epg channel list | Channel playback API → fresh video/session HLS; variant selection | HLS | Unknown catalog-wide; clear path inferred | Direct candidate | US geo-dependent homepage/config; anonymous runtime DSP token | V1 candidate (unlocked clear only) | Not implemented | [localnow.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/localnow.py) |
| Vidaa Free TV (`vidaa`) | Free; app-authenticated anonymous backend | VIDAA 2.0 OAuth/app-signed category → column tiles → batched media details | Selected media streamingParam; refresh metadata when required; DRM wrappers excluded | Mixed HLS/DASH | Mixed; encryption flag and wrapper identify DRM | Direct candidate for clear HLS; bridge for DRM | US app identity; signed backend requires embedded app secrets in upstream, requiring owner review | Future candidate pending permitted credential-free protocol | Not implemented | [vidaa.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/vidaa.py) |
| PBS (`pbs`) | Free | IP/ZIP station discovery and curated station/feed details | Re-fetch matching station feed; select non_drm_url; never fallback to DRM in ArixTV | Mixed HLS/DASH | Mixed; explicit non_drm_url vs drm_dash_url | Direct for clear secondary feeds; bridge for protected main/Kids feeds | US station/ZIP coverage; geographic availability per feed | V1 candidate (clear secondary feeds only) | Not implemented | [pbs.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/pbs.py) |
| C-SPAN free events (`cspan`) | Free event coverage | Congress chamber pages; optional best-effort schedule live-event extraction | Discover currently live event; upstream rewrites live window/manifests and relays segments | HLS | Clear path per upstream | Proxy-required | Segments require Referer; WAF/rate sensitivity; event feeds go dark; not 24/7 networks | Future candidate; external relay; event lifecycle review | Not implemented | [cspan.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/cspan.py) |
| ViX (`vix`) | Free anonymous catalog | Anonymous user bootstrap plus channel GraphQL/catalog | Anonymous playback metadata gives DASH and per-session license location | DASH | Mixed service per README; implemented path is DRM-oriented; rare clear cases unknown | Browser/device DRM bridge on implemented path | US Spanish-language catalog; anonymous tokens; no reliable clear subset established | Out of scope for implemented DRM path; reassess only independently verified clear feeds | Not implemented | [vix.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/vix.py) |
| Free Networks (`free_networks`) | Free public feeds; exceptions below | 35 network adapters, separately selectable; see expanded inventory | Opaque network/channel reference dispatched to its network resolver | HLS selected | Unknown aggregate; selected clear paths inferred | Mostly direct; individual exceptions below | Per-network region, session and bot-protection differences | V1 candidate per network, not blanket support | Not implemented | [free_networks/__init__.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/__init__.py) |
| Tubi TV (`tubi`) | Free account required now | Authenticated tensor-cdn lineup; anonymous HTML catalog no longer populated | Channel cache with per-channel public EPG refetch fallback | HLS | Unknown catalog-wide | Direct candidate technically; account excluded | Email/password token; upstream records anonymous discovery break in September 2026 | Out of scope (account-based) | Not implemented | [tubi.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/tubi.py) |
| Amazon Prime Free Channels (`amazon_prime_free`) | Free channels; cookie/account-assisted pagination | Live page plus collection pagination and playback envelopes | Fresh playback-resources DASH URL using channel envelope/session | DASH | DRM-only implemented path (Widevine/PlayReady) | Browser/device DRM bridge | Region, cookies and channel playback envelopes; anonymous catalog incomplete | Out of scope (DRM and account/session complexity) | Not implemented | [amazon_prime_free.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/amazon_prime_free.py) |
| FreeCast (`freecast`) | Free account; paid value tier separate | Anonymous free-package channels API | Authenticated channel streams API returns IndiCue manifest | HLS free; paid DRM not scraped | Clear free tier; paid tier DRM | Direct candidate for free tier technically | Account JWT required for playback; credentials/session refresh | Out of scope (account-based) | Not implemented | [freecast.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/freecast.py) |
| DirecTV Stream (`directv`) | Subscription/account (FAST entries may be present) | Authenticated channel/entitlement catalog; exclude satellite-only entries | Channel authorization supplies expiring protected CDN URL | HLS protected | DRM-only implemented path | Browser/device DRM bridge | Account/device activation, entitlements and concurrency limits | Out of scope | Not implemented | [directv.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/directv.py) |
| Spectrum (`spectrum`) | Subscription/account | Authenticated lineup and streamable channels | Per-channel stream session mint; session caps and token renewal | DASH | DRM-only | Browser sign-in plus DRM bridge | Browser-authenticated account; access/refresh tokens; concurrent stream limits | Out of scope | Not implemented | [spectrum.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/spectrum.py) |
| Fubo TV (`fubo`) | Free FAST account or paid subscription | Public guide channels filtered by FAST tags/entitlements | Authenticated asset API returns channel URL; separate protected rendition handling | Mixed HLS/DASH | Mixed; FAST clear, some paid DRM | Direct only same public IP; DRM bridge for protected subset | Account required; TLS fingerprinting; IP-bound CDN tokens | Out of scope (account-based, including FAST mode) | Not implemented | [fubo.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/fubo.py) |
| Philo (`philo`) | Subscription/account | Passwordless account session → entitled channels | Fresh authenticated channel playback session | DASH | DRM-only | Browser/device DRM bridge | Email/text activation; account tokens and channel sessions | Out of scope | Not implemented | [philo.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/philo.py) |
| Frndly TV (`frndlytv`) | Paid subscription | Authenticated channel list | Guide/channel path → fresh playback URL | HLS | Mixed/unknown; upstream warns some may be Widevine | Direct for clear paths; protected cases unsupported | US-only account/session tokens | Out of scope | Not implemented | [frndlytv.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/frndlytv.py) |
| ESPN (`espn`) | ESPN plan and/or TV provider | Account/device or TVE entitlements → linear catalog; optional games | Authorized BAM playback metadata returns protected HLS | HLS Widevine CENC | DRM-only implemented path | Browser/device DRM bridge | Activation/account tokens; resource/plan entitlement; blackouts and event windows | Out of scope | Not implemented | [espn.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/espn.py) |
| NFL (`nfl`) | NFL Channel free; NFL Network/RedZone/games account or TVE | Device token and channel entitlement filtering; optional events | Current airing → Anvato/Lura playback URL | HLS | Clear per upstream | Direct candidate technically | Anonymous device credentials; US rights; other channels require account; mobile entitlement must not become a bypass | V1 candidate only NFL Channel after public-app credential review; paid/events out of scope | Not implemented | [nfl.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/nfl.py) |
| CBS (`cbs_sports`) | HQ/Golazo/Champions League free; affiliates/CBSSN TVE | Free fixed feeds plus entitled local/channel metadata; optional games | Free feeds Google DAI; paid feeds require authorization before Syncbak URL | HLS | Clear per upstream | Direct candidate technically | Local DMA and short-lived tokens for paid feeds; free subset needs no user login | V1 candidate free three-feed subset only; paid/events out of scope | Not implemented | [cbs_sports.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/cbs_sports.py) |
| FOX One (`fox_one`) | Mixed free and paid/TVE | Site container/channel discovery plus platform entitlements | Native device/access/location/entitlement playback request, or FOX TVE delegation | HLS selected | Unknown/mixed; request advertises Widevine capability | Unknown; delegated path may need proxy/browser | US geo/device session; paid resources need TVE or subscription | Future candidate for independently verified anonymous clear subset; paid out of scope | Not implemented | [fox_one.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/fox_one.py) |
| Paramount+ (`paramount`) | Paid/account; TV-provider-linked account also supported | Anonymous discrete live-match list; account gates actual playback | Signed-in match metadata → Google DAI HLS; upstream proxies authenticated key delivery | HLS AES-128 | No DRM per upstream, but encrypted keys require auth | Proxy-required for key authentication | Account cookie, per-event token and match availability | Out of scope (account and event-only design) | Not implemented | [paramount.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/paramount.py) |
| A+E Networks TVE (`aenetworks_tve`) | TV-provider cable networks; unauthenticated DAI behavior observed upstream | Fixed cable-network catalog | DAI master; upstream currently lets optional TVE failures fall through | HLS | Unknown catalog-wide; resolver comments imply clear path | Direct URL path in code; README says bridge-only | US cable rights; reachable endpoint does not establish free entitlement | Out of scope; do not reproduce authorization fall-through | Not implemented | [aenetworks_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/aenetworks_tve.py) |
| AMC Networks TVE (`amcn_tve`) | TV-provider subscription | Fixed AMC/BBC America/IFC/WE tv catalog | Adobe authorization → AMC access token → livestream playback API | HLS | Clear path documented in code; full catalog unknown | Direct-HLS code path; README says bridge-only | TVE authorization, device identity and account tokens | Out of scope (TVE) | Not implemented | [amcn_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/amcn_tve.py) |
| SundanceTV (`sundance_tve`) | Sundance account plus TV provider | DICE/IMG live-event/channel catalog | Entitled DICE stream callback returns encrypted HLS/DASH | Mixed HLS/DASH | DRM-only | Browser/device DRM bridge | Short playback tokens; signed-in account and TVE refresh | Out of scope | Not implemented | [sundance_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/sundance_tve.py) |
| CNN TVE (`cnn_tve`) | TV-provider subscription | Fixed CNN/CNN International/HLN catalog | Adobe token exchange → Bolt playback info → protected DASH | DASH | DRM-only | DRM bridge plus manifest rewriting in upstream | TVE, Bolt sessions, short media tokens and relative URL handling | Out of scope | Not implemented | [cnn_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/cnn_tve.py) |
| C-SPAN TVE (`cspan_tve`) | TV-provider subscription | Fixed 24/7 network catalog + Brightcove page metadata | Adobe-authorized Brightcove source plus page CDN token | HLS | Clear per upstream | Direct | US TVE; cached page token; WAF-aware fetches; distinct from free events | Out of scope (TVE) | Not implemented | [cspan_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/cspan_tve.py) |
| DAZN (`dazn_tve`) | Free account and paid DAZN/TVE tiers | Live TV rail filtered by entitlement IDs and geographic zones | Authorized playback API returns token-bound protected manifest | DASH | DRM-only implemented path | DRM bridge plus manifest/segment relay | Account/device linking, TLS fingerprinting and UA-bound CDN token; most listed channels not actually free | Out of scope | Not implemented | [dazn_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/dazn_tve.py) |
| Discovery TVE (`discovery_tve`) | TV-provider subscription | Fixed Discovery/HGTV/Food/TLC and sibling catalog | Authenticated playback API; explicitly reject drmEnabled streams | HLS selected | Clear selected path; protected responses rejected | Direct-HLS code path; browser sign-in; README says bridge-only | TVE session, browser renewal and package entitlement | Out of scope (TVE/browser auth) | Not implemented | [discovery_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/discovery_tve.py) |
| Disney TVE (`disney_tve`) | TV-provider subscription plus free news feeds | ABC/DisneyNOW channel modules and local-affiliate metadata | Brand-specific anonymous/TVE BAM playback token | HLS CENC | DRM-only on implemented path, including free news | Browser/device DRM bridge | Two TVE requestors, brand device keys, local rights | Out of scope (protected playback) | Not implemented | [disney_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/disney_tve.py) |
| FOX TVE (`fox_tve`) | TV-provider cable/sports; free/preview exceptions | Fixed network catalog; FOX Weather intentionally omitted from active lineup | Signed page HLS or sports playback; browser fallback | HLS | Unknown/mixed; inspect actual manifests | Some direct; selected manifest/segment proxy; browser fallback | TVE/preview passes and geo sessions; do not treat preview as free unlimited access | Out of scope (TVE/preview/browser path) | Not implemented | [fox_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/fox_tve.py) |
| NBCUniversal TVE (`nbc_tve`) | TV-provider plus anonymous digital feeds | Public guide metadata and local affiliate detection | Authorized/anonymous NBC playback stack returns protected manifest | DASH | DRM-only implemented path | Browser/device DRM bridge | Local IP/affiliate, TVE for premium feeds; anonymous digital feeds still protected | Out of scope (DRM) | Not implemented | [nbc_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/nbc_tve.py) |
| Versant TVE (`versant_tve`) | TV-provider subscription | ViewLift page LINEARCHANNEL metadata lists all feeds | Adobe/ViewLift entitlement → MediaTailor manifest | Mixed DASH/FairPlay HLS | DRM-only | Browser/device DRM bridge | TVE token and per-channel license state; USA/E!/Syfy/Oxygen/CNBC/MS NOW/Golf | Out of scope | Not implemented | [versant_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/versant_tve.py) |
| Warner Bros. Discovery TVE (`warner_tve`) | TV-provider subscription | Fixed TBS/TNT/truTV/TCM plus public site configuration | Adobe authorization → Turner NGTV protected playback | HLS CENC | DRM-only | Browser/device DRM bridge | Brand-specific TVE and short media tokens | Out of scope | Not implemented | [warner_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/warner_tve.py) |
| Univision TVE (`univision_tve`) | Mixed free news/FAST and TV-provider subscription | Public Quickplay channel/EPG catalog; filter free package | Guest or entitled QPAT/device session → content authorizer URL | Mixed catalog HLS/DASH; HLS selected | Clear per upstream delivery profiles | Direct candidate technically | App OAuth/device registration; free package skips Adobe user auth; Spanish US lineup | V1 candidate explicit free package only; TVE channels out of scope | Not implemented | [univision_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/univision_tve.py) |
| Tennis Channel TVE (`tennis_tve`) | TV-provider main feed; free TC2/Pickleball feeds | Public Sportradar EPG channel/now-playing catalog | Access API; TVE check for main feed, anonymous for free feeds | DASH selected | Mixed/uncertain: README calls free channels clear; code returns DRM token even for free | Main feed DRM bridge; free delivery unknown | US rights; TVE main-feed check-access not live-verified by upstream | Future candidate for proven anonymous clear free feeds; paid out of scope | Not implemented | [tennis_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/tennis_tve.py) |
| NESN TVE (`nesn_tve`) | NESN account plus TV provider | Fixed NESN/NESN Plus catalog | Entitled playback response supplies DASH URL and license metadata | DASH | DRM-only | Browser/device DRM bridge | New England TV-provider rights; emailed account code and TVE sessions | Out of scope | Not implemented | [nesn_tve.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/nesn_tve.py) |
| HDHomeRun (`hdhomerun`) | Owner local hardware; optional paid TV rights | Device discover.json and lineup.json | Map channel to local tuner URL | Other: MPEG-TS; optional hardware transcode | Mixed/unknown device lineup; protected channels must be excluded | Direct on reachable LAN for clear channels | Requires local tuner/network access; tuner capacity; not a public FAST API | Out of scope for V1 product; owner scope decision for future | Not implemented | [hdhomerun.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/hdhomerun.py) |
| Custom Channels (`custom`) | User-supplied; free/paid unknown | Manual URL/web-page stream detection; not automatically scraped | Resolve supplied URL/page; upstream can choose proxy or browser detector | Mixed/other | Unknown | Direct/proxy/browser depending on source | Arbitrary user sources and optional request headers | Out of scope for current V1 provider catalog | Not implemented | [custom.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/custom.py) |
| Custom Playlists (`m3u_<slug>`) | User-supplied; free/paid unknown | Runtime per-playlist M3U source; optional XMLTV import is upstream-only | Resolve imported channel reference; preserve/re-fetch configured URL | Mixed/other | Unknown | Direct/proxy unknown per entry | User authentication headers/URLs may be sensitive | Out of scope for current V1 provider catalog | Not implemented | [playlist.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/playlist.py) |

## Expanded Free Networks inventory

These are members of `free_networks`, not extra registry source identities.
All are **not implemented** in ArixTV. All select HLS and are classified as
**clear-path candidates / aggregate DRM unknown**, unless a row states otherwise.
Most adapters return a public URL unchanged at tune time; that still requires
an independent freshness strategy and clear-stream verification before support.
The common defaults are **no user login**, **no browser/device DRM bridge**, and
**no demonstrated media proxy requirement**. Per-network exceptions and unknowns
are explicit below. No schedule or XMLTV behavior is proposed for ArixTV.

| Network / adapter key | Access | Discovery → tune resolution | Stream / DRM | Delivery | Region/session considerations | Decision | Evidence |
|---|---|---|---|---|---|---|---|
| Sinclair: Comet, Charge!, TheNest, ROAR (`sinclair`) | Free | WordPress live pages → current stream code → public HLS URL | HLS / clear upstream description | Direct candidate | US diginets; placeholder-only ROAR skipped | V1 candidate | [sinclair.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/sinclair.py) |
| DW (`dw`) | Free | Language-specific graph API hlsVideoSrc → return selected feed | HLS / clear inferred | Direct candidate | German service; selected English/Spanish/Arabic feeds | V1 candidate | [dw.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/dw.py) |
| Al Jazeera (`aljazeera`) | Free | Live-page Apollo state → bcPlaybackUrl → selected public HLS | HLS / clear upstream description | Direct candidate | Qatar; English/Arabic/Mubasher/documentary variants | V1 candidate | [aljazeera.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/aljazeera.py) |
| TRT World / Arabi (`trt`) | Free | Fixed official live references → public master | HLS / clear inferred | Direct candidate | Turkey; English/Arabic | V1 candidate | [trt.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/trt.py) |
| NHK World-Japan (`nhk`) | Free | Fixed public live reference → master | HLS / clear upstream description | Direct candidate | Japan international English service | V1 candidate | [nhk.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/nhk.py) |
| Arirang (`arirang`) | Free | Fixed World feed → Akamai master | HLS / clear inferred | Direct candidate | South Korea international English service | V1 candidate | [arirang.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/arirang.py) |
| Bloomberg (`bloomberg`) | Free | Known regional DAI and media-manifest references → session master | HLS / clear upstream description | Direct candidate | US/Europe/Asia/Australia; bot protection affects site/guide, upstream says media works with ordinary HTTP | V1 candidate for public media; avoid guide dependency | [bloomberg.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/bloomberg.py) |
| CBS News (`cbsnews`) | Free | Live-collection JSON with static fallback → Google DAI feed | HLS / clear inferred | Direct candidate | US national/local; channel listing requires browser TLS fingerprint upstream | Future candidate pending ordinary-HTTP catalog or official feed evidence | [cbsnews.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/cbsnews.py) |
| Scripps News (`scripps`) | Free | Scheduler API extras.stream_url with public fallback → live URL | HLS / clear inferred | Direct candidate | US; endpoint doubles as channel metadata | V1 candidate | [scripps.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/scripps.py) |
| Newsmax2 / World at War (`newsmax`) | Free subset; main Newsmax/Spanish paywalled | Known non-paywall page feeds → public HLS | HLS / clear inferred | Direct candidate | US; exclude Newsmax+ paid feeds | V1 candidate free subset only | [newsmax.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/newsmax.py) |
| NewsNation (`newsnation`) | TV-provider service with preview; free status disputed | Known site video ID → Lura JSONP playback → session master | HLS / clear per upstream | Direct technically; entitlement unresolved | Web player requests login after short preview; upstream observed longer delivery | Future candidate only after owner rights review; never bypass preview limits | [newsnation.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/newsnation.py) |
| Free Speech TV (`fstv`) | Free | Known website Zype linear reference → master | HLS / clear inferred | Direct candidate | US; static channel reference | V1 candidate | [fstv.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/fstv.py) |
| EWTN (`ewtn`) | Free | Public feed API mediaUrl (exclude radio) → selected feed URL | HLS / clear inferred | Direct candidate | Regional TV feeds and languages; adoration feed | V1 candidate | [ewtn.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/ewtn.py) |
| PosiTV (`positiv`) | Free | Known JW live-broadcast reference → master | HLS / clear inferred | Direct candidate | US; separate from duplicate TBN feed | V1 candidate | [positiv.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/positiv.py) |
| TBN (`tbn`) | Public playlist; site places most feeds behind free account | JW Live Channels playlist → selected HLS source | HLS / clear upstream description | Direct technically; account intent unresolved | International feeds; upstream skips duplicate PosiTV | Future candidate pending anonymous-access/terms review | [tbn.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/tbn.py) |
| HSN / HSN2 (`hsn`) | Free | Fixed public channel references → Akamai live | HLS / clear inferred | Direct candidate | US shopping feeds | V1 candidate | [hsn.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/hsn.py) |
| QVC family (`qvc`) | Free | Fixed QVC1–4 feed references → Akamai live | HLS / clear inferred | Direct candidate | US; site guide bot wall irrelevant to media-only scope | V1 candidate | [qvc.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/qvc.py) |
| Tastemade (`tastemade`) | Free | Channel API → editions → stream endpoint → Amagi URL | HLS / clear upstream description | Direct candidate | App/service/device identifiers; Spanish variant; recheck at tune | V1 candidate | [tastemade.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/tastemade.py) |
| A+E Networks FAST (`aetn`) | Free FAST; cable entries excluded | Brand on-now flight payload, streamType=fast → public Wurl/Amagi HLS | HLS / clear upstream description | Direct candidate | US; distinct from cable TVE source | V1 candidate | [aetn.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/aetn.py) |
| KSL (`ksl`) | Free | Public JW Live Channels playlist → HLS source | HLS / clear inferred | Direct candidate | Utah/local sports; guide headers/TLS fingerprint are upstream guide concerns | V1 candidate media subset | [ksl.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/ksl.py) |
| CBN (`cbn`) | Free | Known channel/Brightcove IDs → fresh playback API HLS with no key_systems | HLS / explicitly filter DRM sources | Direct candidate | English/Spanish; public player policy header for API, not assumed needed on CDN | V1 candidate | [cbn.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/cbn.py) |
| Game & Fish TV (`gameandfish`) | Free | Known official-page Amagi reference → master | HLS / clear inferred | Direct candidate | US outdoors feed | V1 candidate | [gameandfish.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/gameandfish.py) |
| Outside TV family (`outside`) | Free | Known main Amagi feed and JW SSAI channel references → masters | HLS / clear inferred | Direct candidate | US; includes Climbing/Mixtape/Pinkbike/Warren Miller | V1 candidate | [outside.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/outside.py) |
| The CW FAST lineup (`cw`) | Free FAST | Public paged channel feed → stream_url; reject has_drm/non-HLS | HLS / explicit DRM filter | Direct candidate | US; third-party FAST channels also carried | V1 candidate | [cw.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/cw.py) |
| Sky News / Sky News Arabia (`sky`) | Free | Known public website feed references → master | HLS / clear inferred | Direct candidate | GB/AE; English and Arabic | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| Cheddar (`cheddar`) | Free | Known public live reference → master | HLS / clear inferred | Direct candidate | US news | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| BUZZR (`buzzr`) | Free | Known public Amagi reference → playlist | HLS / clear inferred | Direct candidate | US game shows | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| AccuWeather NOW (`accuweather`) | Free | Known public Amagi reference → playlist | HLS / clear inferred | Direct candidate | US weather; not assumed to include paid network | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| Court TV (`courttv`) | Free | Known public Yospace reference → per-request ad session master | HLS / clear inferred | Direct candidate | US; each master request opens a session | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| CMC / CMC-USA (`cmc`) | Free | Known public Amagi/streaming-host references → playlists | HLS / clear inferred | Direct candidate | US music and country feeds | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| Global News Canada (`globalnews`) | Free | Known national/regional DAI references → session masters | HLS / clear inferred | Direct candidate | Canada regional streams | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| NBC LX Home (`lx`) | Free | Known public Akamai live reference → playlist | HLS / clear inferred | Direct candidate | US lifestyle feed; distinct from NBC TVE | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| NRB TV (`nrb`) | Free | Known public live reference → playlist | HLS / clear inferred | Direct candidate | US faith | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| theDove TV (`thedove`) | Free | Known public live reference → playlist | HLS / clear inferred | Direct candidate | US Oregon local faith | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |
| EPlusTV 6 Jackson (`eplustv6`) | Free | Known TelVue live reference → playlist | HLS / clear inferred | Direct candidate | US Tennessee local feed | V1 candidate | [simple.py](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/simple.py) |

## Registry completeness and support infrastructure

- [`registry.py`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/registry.py) discovers the 52 built-ins
  through `BaseScraper` subclasses (including the Free Networks package).
  `amazon-prime-free` aliases `amazon_prime_free`; `amc_tve` and
  `amcnetworks_tve` alias `amcn_tve`. The LG source ID is `lg-channels`, although
  its file is `lg_channels.py`. Preserve this distinction in research updates.
- [`playlist.py`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/playlist.py) is a runtime source family,
  with `source_name=None` until registry creates `m3u_<slug>` subclasses. It is
  included explicitly above rather than missed by a static name scan.
- [`m3u_import.py`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/m3u_import.py) and
  [`xmltv_import.py`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/xmltv_import.py) are import helpers,
  not separate service providers. `amazon_auth.py`, `directv_device_auth.py`,
  `directv_numbers.py`, `stream_detector.py`, `base.py`, `category_utils.py`
  and the registry are support modules, not additional sources.
- The 24 dedicated Free Networks adapters and 11 `StaticNetwork` groups are
  wired in [`free_networks/__init__.py`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/scrapers/free_networks/__init__.py).
  Networks' individual carried channels (for example FilmRise via CW) are not
  invented as separate provider integrations.
- Adobe MVPD identities in `app/tve/` are authentication backends for listed
  TVE sources, not independent channel sources. PrismCast/FC Player are playback
  bridges, not free provider candidates. None belongs in ArixTV V1.
- Plex **as a content source** is the `plex.py` row. Upstream's `docs/plex.md`
  is about Plex **as an output player**, and does not invalidate that source.

## Resolver evidence and discrepancies requiring care

[`README Source Notes`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/README.md#source-notes) are useful orientation;
actual scrapers and [`play.py`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/routes/play.py) take precedence when
claims differ. These discrepancies are recorded rather than resolved by copying
upstream workarounds:

1. **STIRR:** `stirr_manifest_proxy`, `stirr_variant_proxy`, and
   `stirr_segment_proxy` explain IP-bound SSAI and legacy TLS origins.
   A public 302 from a VPS cannot be assumed to work for a remote viewer.
2. **Zeam:** `ZeamScraper` documents a roughly 90-second IP-bound token and
   latest-master user-agent binding, and sets `play_manifest_check=False`.
   Same-egress LAN tests and external-relay tests are separate acceptance cases.
3. **TCL:** `tcl_manifest_proxy` and the Publica-host branch in `play()` proxy
   all tiers for browser CORS. Native IPTV clients do not enforce browser CORS;
   do not automatically call this a mandatory relay requirement for them.
4. **C-SPAN:** `CSpanScraper` and `cspan_*` playback routes require Referer on
   segments and rewrite an event live window. Free event coverage is distinct
   from the clear but TVE-entitled 24/7 networks in `cspan_tve.py`.
5. **Samsung:** `fetch_channels()` drops `license_url` entries; the catalog
   comes from a third-party mirror, not a first-party discovery API. Mirror
   freshness/permission and final-CDN header requirements remain unverified.
6. **VIDAA:** code explicitly replaces the older backend with VIDAA 2.0,
   requiring OAuth/app-signed requests; clear HLS and encrypted mini-player DASH
   are separate. Do not copy embedded app credentials into ArixTV.
7. **A+E / AMC / Discovery TVE:** README labels them bridge-only. A+E code
   returns DAI HLS even if optional authorization fails; AMC describes validated
   direct HLS; Discovery explicitly rejects DRM-enabled responses. Their policy
   remains TVE/out of scope regardless of technical reachability. The inventory
   does not reclassify paid cable networks as free on that evidence.
8. **DAZN:** README broadly calls several channels free, but resolver research
   notes distinguish a free account's entitlement from paid tiers and geographic
   zones. A listed channel is not proof of authorization.
9. **Tennis:** README calls free TC2/Pickleball clear, while the current module
   describes the anonymous DASH access response as containing a DRM token.
   Leave the free subset unresolved until actual manifest evidence establishes
   whether protection is optional or mandatory.
10. **Free Networks:** NewsNation's player has a preview/sign-in boundary, and
    TBN's site puts most feeds behind free accounts. Endpoint reachability alone
    is insufficient to approve either for anonymous V1 playback. CBS News
    discovery uses TLS impersonation; standard-library Go compatibility is open.
11. **DRM detection:** upstream
    [`hls.py`](https://github.com/kineticman/FastChannels/blob/7e61c1ef127f7119326eb98381726d6f7cfaa56d/app/hls.py), `worker.py`, and the license-capable registry
    handle a broader protected playback product. Its bridge/audit semantics and
    treatment of AES-128 must not be imported as ArixTV policy. A clear ad pod
    is not proof the whole channel is clear.

## Recommended Phase 3 research order

Begin with **Vizio WatchFree+, Xumo Play, and Plex** as the first 2–3 protocol
candidates. Their inspected paths use anonymous discovery/session initialization
and return HLS URLs without an upstream media-proxy branch. This is a research
priority, not a compatibility promise. LG Channels is a useful reserve candidate;
Bally Sports Live, Hallmark's free feeds and selected Free Networks (DW, NHK,
The CW, CBN) provide narrower alternatives.

Pluto is also plausible but adds per-country JWT/session management. Roku adds
Cloudflare and mixed DRM complexity. Samsung adds a third-party mirror. Keep
Zeam's cross-IP case, STIRR and free C-SPAN events in future/external-relay
research. Keep VIDAA pending protocol/credential permission review. Never add
relay, browser automation, DRM licensing or video bytes to the core to make a
candidate fit.

For every chosen provider in Phase 3: recheck current upstream and permitted
public behavior; document first-party protocol and regions; independently write
Go discovery/resolution; create sanitized local fixtures/mock-server tests;
verify nested manifests/segments, token expiry and resolver/client IP differences;
exclude all protected/unknown cases until classified. Update this inventory and
`PROVIDERS.md` with actual maturity and `clear_direct`, `clear_relay_only`, `drm`,
or `unsupported` capabilities then. No provider work is authorized in Phase 0.

## Licensing and owner review

No root license or COPYING file was found in the inspected upstream tree. GitHub
visibility is not a reuse license. Treat FastChannels solely as a behavioral
research reference; request owner/legal review before any source reuse. No code
was copied or translated and no upstream runtime dependency was introduced.
ArixTV's own license remains unselected, awaiting explicit owner approval.

Before provider implementation, owner review should settle: third-party catalog
mirror acceptability; ordinary AES-128 without DRM policy; permitted anonymous
app identities/request signing (especially VIDAA and NFL); preview/account-gated
sources; and whether same-egress-only compatibility qualifies as a limited LAN
provider. These questions do not block the Phase 0 foundation. Protected content,
paid accounts and entitlement fall-through remain outside V1.

The initial Go parser dependency is `github.com/BurntSushi/toml v1.5.0` (MIT;
its module includes COPYING). It is Go-only and avoids writing an incomplete
custom TOML parser. This dependency's license does not choose ArixTV's license.

## Maintenance

Repeat this inspection when upstream providers change or ArixTV support changes.
Record the new branch, commit and inspection date; compare the registry and
Free Networks adapter list, update aliases/dynamic families, revise evidence and
unknowns, and update current ArixTV implementation status. Preserve explicit
exclusions even when a provider has no V1 implementation.

## Phase 3A Vizio-only reinspection (2026-10-26)

The table's Vizio row is now based on current default `development` commit
[`40f5903cb43979e00e8d6cc7186d855c93eed6b1`](https://github.com/kineticman/FastChannels/tree/40f5903cb43979e00e8d6cc7186d855c93eed6b1)
and independent public API requests. Other rows retain the original Phase 0
research pin and were not re-certified. References inspected: `app/scrapers/vizio.py`,
`app/scrapers/base.py` (`resolve`), README Sources / Source Notes. Vizio still uses
the anonymous first-party catalog and inherited unchanged-URL resolver; ArixTV
independently refetches current metadata at tune time instead of persisting URLs.

The public response contained 472 channels, explicit protection fields and one
licensed/token-gated NFL entry. API requests accepted ArixTV's own User-Agent and
JSON Accept header without credentials. Discovery needs no guide calls. Sample
CDN probes were mixed; two opt-in Go smoke attempts did not certify live Resolve
(one inspector limitation corrected offline, then catalog timeout). API quotas,
URL lifetime and remote-client egress compatibility remain unestablished.
[Vizio protocol/support notes](providers/VIZIO.md) record counts, classification,
headers, region/session assumptions, exact live instructions and limitations.
No upstream source or raw provider captures were incorporated.

## Phase 3B Xumo-only reinspection (2026-10-27, Asia/Hong_Kong)

Rechecked current default/development commit
[`376c1a69b8c11f6271e4557859fee75fe9ec5cb8`](https://github.com/kineticman/FastChannels/tree/376c1a69b8c11f6271e4557859fee75fe9ec5cb8).
Only Xumo's row/current evidence changes in Phase 3B; other provider evidence pins,
including Vizio's experimental status, are preserved. Inspected Xumo's scraper,
shared base/play/audit references, README and identified associated category/data
mapping paths. No upstream code, tests or guide mappings were incorporated.

Independent public requests returned a complete version-2 market-10006 catalog:
454 entries, no account authentication or fixed geo/device query parameters.
One CBS News identity was independently checked through live asset/provider-source
resolution and bounded clear HLS manifests, without playback cookies or special
headers. Eighteen callsigns indicated DRM; the remaining unverified identities are
not automatically Direct. The official website reports US-only availability.

Observed protocol details strengthened the initial inventory: a future
`refresh.from/to` range can accompany an already-live asset, and a valid opaque
source-query value can contain a literal semicolon. ArixTV preserves those values,
uses current asset validity and conservative expiry ceilings, and has regression
tests for both findings. Final Go live smoke passed after correcting those two
Xumo-local interpretations; one success is not stable/region/expiry certification.
[Xumo notes](providers/XUMO.md) document headers, generated anonymous identities,
all inspected paths, stream selection, limits and outstanding uncertainty.
