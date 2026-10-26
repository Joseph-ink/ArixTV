# ArixTV Implementation Plan

This file is the active implementation roadmap. Keep it current as work progresses.

## Guiding principle

ArixTV should remain a small, auditable FAST/free-IPTV discovery and 302 resolver service. Do not turn it into a full IPTV management suite.

## Phase 0 — Repository foundation

### Goals
Establish a clean Go repository that builds locally and in CI before provider work begins.

### Tasks
- [x] Initialize Go module.
- [x] Add portable command entry point under `cmd/arixtv/`.
- [x] Add internal package layout.
- [x] Add `arixtv version`.
- [x] Add initial config parser and `arixtv config validate`.
- [x] Add `.gitignore`.
- [x] Add GitHub Actions CI for formatting, vet, tests, and build validation.
- [x] Validate cross-builds for:
  - linux/amd64;
  - linux/arm64;
  - darwin/arm64.
- [x] Add basic unit tests.
- [x] Ensure no generated/runtime files are tracked.
- [x] Inspect the current `https://github.com/kineticman/FastChannels` repository as the primary upstream provider reference.
- [x] Create/populate `docs/FASTCHANNELS_MAPPING.md` with every current FastChannels provider that can be identified.
- [x] Classify each mapped provider by free/paid, clear/mixed DRM/DRM-only/unknown, direct/proxy/bridge/unknown, and ArixTV V1/future/out-of-scope.
- [x] Record FastChannels evidence paths/URLs for non-obvious classifications.
- [x] Do not implement providers during Phase 0.

### Phase 0 progress and decisions (2026-10-26)

- **Phase 0 approved by owner.** Formatting, vet, unit tests,
  example/CLI smoke checks, module verification, all three cross-builds, inventory
  completeness, evidence paths, secret-pattern and ignore-rule checks passed.
  See `docs/PHASE0_REPORT.md` for commands, results and the repository tree.
- Phase 0.1 replaces the former compatibility baseline with Go 1.27.3 for
  minimum support, CI and release builds. Linux binaries were built, not executed on
  Linux; native macOS arm64 CLI smoke tests passed.
- Module path follows the existing origin: `github.com/Joseph-ink/ArixTV`.
  Minimum Go version is 1.27.3; runtime has no cgo dependency.
- One small MIT-licensed dependency, BurntSushi/toml v1.5.0, provides full TOML
  parsing. An incomplete custom parser would be a larger correctness risk.
- Config validation uses the spec's initial schema, local `./config.toml`
  default, explicit Linux path override, safe defaults, strict unknown-field
  checks, and diagnostics that do not echo supplied values. Phase 1's config
  task remains for model extensions and integration, not duplicate parsing.
- Upstream default `development` inspected at
  `7e61c1ef127f7119326eb98381726d6f7cfaa56d`: 52 built-in source identities,
  one dynamic playlist family, and 35 expanded Free Networks adapters.
- Mapping distinguishes source-code evidence from live playback validation.
  Initial Phase 3 research shortlist: Vizio, Xumo, Plex; LG as reserve.
- Owner has since settled mirror, AES-128, anonymous identity, account and
  same-egress policies in the specification. ArixTV license and upstream reuse
  rights remain open; no upstream provider code is copied.
- No commit or push requested. Hosted CI execution will require pushing the
  workflow; local execution of its checks is recorded on completion.
- Owner approved Phase 0 and authorized Phase 0.1 followed by Phase 1 only.

### Exit criteria
- Repository builds.
- `docs/FASTCHANNELS_MAPPING.md` contains a current, evidence-backed provider migration inventory derived from FastChannels.
- CI passes.
- No provider implementation yet.
- Cross-build succeeds for all three required targets.

## Phase 0.1 — Security/toolchain baseline

- [x] Set minimum, CI and future release baseline to Go 1.27.3.
- [x] Remove obsolete compatibility claims/checks from documentation.
- [x] Validate tidy, modules, formatting, vet, tests and all three builds.
- [x] Official govulncheck v1.9.0 added in separate CI job; local scan passed.

## Phase 1 — Core domain and state

**Phase 1 approved by owner.** Domain/state foundations, deterministic
failure tests, race checks and Go 1.27.3 cross-builds are complete. No HTTP server,
refresh coordinator, provider runtime or lifecycle CLI was implemented in Phase 1. See
`docs/PHASE1_REPORT.md` for exact commands and design limits.

### Goals
Implement the portable core models and safe local state handling.

### Tasks
- [x] Channel model.
- [x] Stream model.
- [x] Playback capability model, including unknown and ordinary HLS encryption policy.
- [x] Provider metadata/status model.
- [x] Provider registry.
- [x] Config model.
- [x] Logging with secret redaction.
- [x] Token creation/validation/hash persistence.
- [x] Provider snapshot storage.
- [x] Atomic JSON write helper.
- [x] Atomic M3U write helper.
- [x] Last-known-good snapshot semantics.
- [x] Refresh lock.
- [x] State metadata.
- [x] Tests for atomic updates and rollback behavior.

### Exit criteria
- State writes are crash-safe.
- Failed refresh cannot destroy last known good data.
- Token and logging tests demonstrate no cleartext leakage.

## Phase 2 — HTTP server and network policy

**Phase 2 complete; awaiting owner review.** Serve, separate listener policies,
strict routes, explicit origins, bounded resource limits, immutable catalog reload
and managed shutdown are implemented and validated. Phase 1 schema/LKG/corrupt-
state/durability contracts are unchanged. See `docs/PHASE2_REPORT.md` for commands,
results and deployment decisions. No real providers or media proxy were added.

### Goals
Serve protected playlists and resolver URLs without proxying media.

### Tasks
- [x] `arixtv serve --config PATH`, SIGINT/SIGTERM and bounded graceful shutdown.
- [x] Explicit advertised origins; deterministic clear-direct-only HTTP M3U.
- [x] Bounded accepted connections and global nonqueued resolver work.
- [x] Loopback listener.
- [x] Optional explicit LAN listeners.
- [x] LAN CIDR ACL.
- [x] Public direct prefix `/direct/`.
- [x] Protected playlist route.
- [x] Protected playback resolver route.
- [x] Internal loopback-only relay resolver route.
- [x] Trusted proxy handling.
- [x] Ignore forwarded headers from untrusted peers.
- [x] Request method restrictions.
- [x] HTTP server timeouts and header limits.
- [x] Basic rate limiting.
- [x] `/healthz` loopback-only endpoint.
- [x] Snapshot reload without restart.
- [x] Optional systemd notify/watchdog support on Linux.
- [x] Tests for ACL, auth, 404 behavior, trusted proxy behavior.

### Exit criteria
- LAN requests require valid network + token.
- Reverse-proxy direct mode works on loopback.
- Internal relay resolver cannot be reached from LAN/public listeners.
- Resolver responses can return a mocked 302.

## Phase 3 — First providers

### Goals
Prove the provider interface using a small number of stable free providers selected from the FastChannels migration inventory and re-verified against current upstream behavior.

### Initial candidates
Start with 2–3 providers classified as V1 candidates in `docs/FASTCHANNELS_MAPPING.md` that currently expose clear, legally accessible FAST streams and have understandable public APIs.

Candidate research pool:
- Xumo Play
- Plex
- Pluto TV
- Vizio WatchFree+
- LG Channels
- Samsung TV Plus
- Roku Channel
- Zeam

### Tasks per provider
- [ ] Document discovery endpoint/workflow.
- [ ] Document required headers/session data.
- [ ] Implement discovery.
- [ ] Normalize channels.
- [ ] Implement tune-time resolver.
- [ ] Classify unsupported/DRM cases.
- [ ] Add sanitized fixtures.
- [ ] Add mock-server tests.
- [ ] Document limitations in `docs/PROVIDERS.md`.

### Exit criteria
- At least 2–3 providers refresh successfully.
- Generated playlist contains only supported clear channels.
- Tune-time resolver obtains fresh URLs.

## Phase 4 — Refresh engine

### Goals
Make scheduled refresh robust on small VPS hardware.

### Tasks
- [ ] Multi-provider refresh coordinator.
- [ ] Global concurrency bound.
- [ ] Per-provider concurrency bound.
- [ ] Per-request deadlines.
- [ ] Failure isolation.
- [ ] Last-known-good preservation.
- [ ] Incremental validation/fingerprinting.
- [ ] `arixtv refresh`.
- [ ] `arixtv refresh --provider <id>`.
- [ ] Provider list/enable/disable/test commands.
- [ ] Status reporting.
- [ ] Tests for partial failures.

### Exit criteria
- One provider failure does not fail successful providers.
- Refresh cannot run concurrently with itself.
- 1C1G target remains comfortable.

## Phase 5 — Linux interactive installer and service management

### Goals
Install and manage ArixTV on supported Linux distributions.

### Tasks
- [ ] `install.sh`.
- [ ] Detect Linux, root/sudo, systemd, architecture, distribution, networking, LAN CIDRs, ports, Nginx presence, and download tools.
- [ ] Support Debian, Ubuntu, Oracle Linux, and practical RHEL-compatible systems.
- [ ] Install all project files under `/etc/arixtv/`.
- [ ] Create `arixtv` system user/group.
- [ ] Generate config.
- [ ] Interactive provider selection.
- [ ] Interactive LAN CIDR selection/custom input.
- [ ] Remote domain input.
- [ ] Token creation.
- [ ] Create systemd units.
- [ ] Create refresh timer.
- [ ] Create `/etc/arixtv/manage.sh`.
- [ ] Create `/etc/arixtv/uninstall.sh`.
- [ ] Reconfiguration workflow.
- [ ] Config backup/validation/rollback.
- [ ] Restart/status workflow.
- [ ] Complete uninstall workflow.

### Exit criteria
- Fresh Linux VM can install, configure, run, reconfigure, and fully uninstall ArixTV.
- No ArixTV project files are installed outside `/etc/arixtv/` except systemd unit files.
- User Nginx configuration is never automatically modified in V1.

## Phase 6 — Nginx config generation

### Goals
Generate safe, copy/paste Nginx snippets for one domain with path routing.

### Paths
- `/direct/` → ArixTV
- `/relay/` → external relay backend

### Tasks
- [ ] Direct location generator.
- [ ] Relay location generator.
- [ ] Preserve prefixes; avoid fragile rewrite rules.
- [ ] Secret-safe access logging defaults.
- [ ] Correct forwarded headers.
- [ ] Relay buffering/timeouts guidance.
- [ ] Management option to show Nginx configuration.

### Exit criteria
- Single-domain path routing works without content rewriting by Nginx.

## Phase 7 — Optional external relay integration

### Goals
Allow clients to use the VPS as the upstream egress without adding relay logic to ArixTV.

### Tasks
- [ ] Define relay backend adapter/config boundary.
- [ ] Generate `/etc/arixtv/data/relay.m3u`.
- [ ] Relay M3U uses loopback ArixTV internal resolver URLs.
- [ ] Use separate internal relay secret.
- [ ] Evaluate a lightweight existing IPTV proxy.
- [ ] Do not hard-code ArixTV core to one proxy implementation.
- [ ] Build automated integration test with mock HLS upstream.
- [ ] Verify nested HLS playlists and segments.
- [ ] Verify client disconnect cancellation if supported.
- [ ] Install optional relay backend into `/etc/arixtv/relay/`.
- [ ] Add optional `arixtv-relay.service`.

### Exit criteria
- Relay works end-to-end.
- Relay backend is replaceable without provider changes.
- Relay never fetches ArixTV's public M3U through Nginx.

## Phase 8 — CI/CD and releases

### Goals
Produce reproducible release assets.

### Tasks
- [ ] Tag-triggered release workflow.
- [ ] Build linux/amd64, linux/arm64, darwin/arm64.
- [ ] Embed version, commit, and build date.
- [ ] Produce SHA256 checksums.
- [ ] Create GitHub Release assets.
- [ ] Installer downloads release assets, not CI artifacts.
- [ ] Update workflow with checksum validation.
- [ ] Binary backup and rollback if health check fails.

### Exit criteria
- GitHub releases provide all required binaries and checksums.
- Linux install/update flow can verify and roll back.

## Phase 9 — Hardening and documentation

### Goals
Prepare for V1 public release.

### Tasks
- [ ] Review systemd sandboxing.
- [ ] Review HTTP rate limits.
- [ ] Review all log redaction.
- [ ] Review token storage.
- [ ] Implement `arixtv doctor`.
- [ ] Failure-mode tests.
- [ ] Long-running refresh tests.
- [ ] OCI 1C1G validation.
- [ ] macOS arm64 functional validation.
- [ ] Final README.
- [ ] Provider compatibility table.
- [ ] Security documentation.
- [ ] Installation/uninstallation verification.

### V1 release gate
V1 is ready only when:
- Linux amd64/arm64 and macOS arm64 binaries build;
- Linux installer is reproducible;
- no EPG/DRM/browser/Redis/FFmpeg/Node dependency exists;
- provider failures preserve last-known-good data;
- direct M3U and 302 playback work;
- LAN ACL and reverse-proxy modes are tested;
- optional relay integration is tested or explicitly marked experimental;
- complete uninstall works.
