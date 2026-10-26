# ArixTV

**ArixTV is a lightweight FAST channel aggregator and M3U resolver.**

ArixTV discovers supported free live-TV channels from multiple providers, filters unsupported/DRM playback paths, generates protected M3U playlists, and resolves fresh upstream stream URLs at tune time using HTTP 302 redirects.

The project is designed to stay small enough for low-resource Linux VPS deployments while remaining convenient to develop and test on Apple Silicon macOS.


## Origin

ArixTV was conceived as a lightweight redesign of the free FAST/IPTV discovery and resolver capabilities found in [kineticman/FastChannels](https://github.com/kineticman/FastChannels).

FastChannels is used as the primary research reference for ArixTV's initial provider catalog and provider behavior. ArixTV does **not** aim to reproduce the full FastChannels platform and does not mechanically translate its source code.

ArixTV intentionally keeps only the small architecture needed for:

```text
free FAST discovery → normalized channels → protected M3U → tune-time resolver → 302
```

See [`docs/FASTCHANNELS_MAPPING.md`](docs/FASTCHANNELS_MAPPING.md) for the migration/research inventory.

## What ArixTV is

```text
Providers
   ↓
ArixTV refresh
   ↓
normalized channel snapshots
   ↓
protected M3U
   ↓
Player requests channel
   ↓
ArixTV resolves fresh upstream URL
   ↓
HTTP 302
   ↓
Player connects directly to provider CDN
```

## What ArixTV is not

ArixTV intentionally has:

- no EPG/XMLTV;
- no DVR;
- no transcoding;
- no Web UI;
- no Redis;
- no browser automation;
- no built-in video proxy;
- no DRM bypass;
- no built-in TLS/ACME.

Optional video relay is handled by a separate, replaceable IPTV proxy.

## Required build targets

- Linux amd64
- Linux arm64
- macOS arm64

Linux is the primary production platform. macOS arm64 is supported for development, refresh, provider testing, M3U generation, and local serving.

## Deployment modes

### LAN Direct

```text
LAN Player → HTTP ArixTV → 302 → upstream CDN
```

Users select allowed LAN CIDR ranges during setup. A valid token is still required.

### Remote Direct

```text
Player → HTTPS Nginx → HTTP ArixTV(loopback) → 302 → upstream CDN
```

Nginx terminates TLS. ArixTV itself remains HTTP-only.

Public path:

```text
/direct/
```

### Optional Remote Relay

```text
Player → HTTPS Nginx → external IPTV proxy → ArixTV resolver → upstream
```

Public path:

```text
/relay/
```

The relay proxy consumes a local ArixTV-generated `relay.m3u`. It does not fetch ArixTV's public playlist.

## Linux installation layout

ArixTV-managed files live under:

```text
/etc/arixtv/
```

Systemd units live under:

```text
/etc/systemd/system/
```

Expected services:

```text
arixtv.service
arixtv-refresh.service
arixtv-refresh.timer
```

Optional:

```text
arixtv-relay.service
```

## Development status

Phase 1 provides typed channel/stream classifications, a provider registry,
strict configuration, secret-redacting logging, hashed token storage, versioned
atomic snapshots, refresh metadata and Linux/macOS refresh locks. Phase 2 adds `serve`, separate loopback/LAN policies, protected Direct M3U output,
tune-time 302 resolution, bounded limits, snapshot reload and graceful shutdown.
Phases 3A/3B add experimental anonymous **Vizio WatchFree+** and **Xumo Play**
runtimes, using first-party catalogs and fresh HLS resolution. No other real
providers are registered. Vizio remains experimental with live playback unverified.
See [Vizio limitations](docs/providers/VIZIO.md) and
[Xumo support and evidence limits](docs/providers/XUMO.md).
Refresh coordination/CLI, file playlist generation, relay integration and
installation remain planned. Enabling a provider does not initiate discovery; serving
consumes existing snapshots. The offline compatibility test demonstrates the full
discovery → snapshot → protected M3U → bodyless 302 path.

Requires Go 1.27.3 or newer. CI and release builds use Go 1.27.3. Build and run locally:

```sh
go build -o bin/arixtv ./cmd/arixtv
./bin/arixtv version
./bin/arixtv config validate --config config.example.toml
./bin/arixtv serve --config config.example.toml
```

`config validate` defaults to `./config.toml` on every platform. An eventual
Linux service will supply `/etc/arixtv/config.toml` explicitly. To create a local
configuration, copy `config.example.toml` to `config.toml` (ignored by Git).
Validation reads the file without writing state or making network requests.
See [configuration behavior](docs/CONFIGURATION.md) for defaults and limits, and
the [Phase 2 report](docs/PHASE2_REPORT.md) for HTTP design, routes and validation.

The available commands are `version`, `config validate`, `serve`, and help. Exit
codes are 0 for success/help, 1 for configuration/startup/runtime failure, and 2 for usage errors.
Development version metadata defaults to `dev` / `unknown` / `unknown`; build
systems can set `main.version`, `main.commit`, and `main.buildDate` with Go
`-ldflags -X`. Release publishing remains Phase 8.

Run the foundation checks:

```sh
gofmt -w .
go vet ./...
go test ./...
mkdir -p bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/arixtv-linux-amd64 ./cmd/arixtv
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/arixtv-linux-arm64 ./cmd/arixtv
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/arixtv-darwin-arm64 ./cmd/arixtv
```

GitHub Actions checks formatting, module integrity, vet, tests, pinned vulnerability scanning, example
configuration, and all three builds. Tests use local inputs, with no live
provider dependency. Linux deployment assets belong under `deploy/linux/`,
outside the portable Go packages.

The current upstream inventory is pinned to the inspected FastChannels commit
in the mapping document. Candidate status is research, not playback support.

See:

- [Product specification](ARIXTV_SPEC.md)
- [Implementation plan](PLANS.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Configuration](docs/CONFIGURATION.md)
- [Security](docs/SECURITY.md)
- [Provider model](docs/PROVIDERS.md)
- [FastChannels provider mapping](docs/FASTCHANNELS_MAPPING.md)
- [Nginx integration](docs/NGINX.md)
- [Installation design](docs/INSTALL.md)

## Project principles

- Keep the Go core small.
- Prefer the standard library.
- Preserve last-known-good provider data.
- Never let one provider failure break the whole refresh.
- Do not store expiring upstream URLs as long-lived playlist entries.
- Do not bypass DRM.
- Never commit real tokens, credentials, cookies, or private keys.

## License

License has not yet been selected. Do not assume reuse rights until a repository license is explicitly added.


## Phase 2 serving notes

The example starts an empty loopback service at `127.0.0.1:8787`; `/healthz` is
local-only. With no provisioned tokens or providers, public requests are denied.
The binary registers experimental `vizio` and `xumo`; the example keeps both disabled.
Specifying unknown or unavailable enabled providers is a safe startup error. Token lifecycle CLI
commands remain staged; tests provision tokens through the Phase 1 store API.

LAN mode additionally requires explicit listeners, allowed CIDRs and
`lan.public_origin`, for example `http://192.168.50.10:8787`. Remote Direct requires
an HTTPS `remote.public_origin` and the documented one-hop Nginx headers. Incoming
Host/forwarded-host values never determine playlist URLs. Both modes use the
configured `remote.direct_path` prefix (default `/direct`) without proxy rewrites.

The optional internal resolver is disabled in the CLI pending Phase 7 credential
setup; the server exposes a separate injected validator for integration tests and
future relay setup. No relay playlist or media proxy is implemented. See
[configuration](docs/CONFIGURATION.md), [Nginx](docs/NGINX.md) and
[security](docs/SECURITY.md) for the full deployment contract.
