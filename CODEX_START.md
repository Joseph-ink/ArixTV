# Codex Start Prompt

Open this repository as the active project.

Read these files in order:

1. `AGENTS.md`
2. `ARIXTV_SPEC.md`
3. `PLANS.md`
4. `README.md`
5. the documents under `docs/`

Then complete **Phase 0 only** from `PLANS.md`.

Requirements for this first task:

- Treat `ARIXTV_SPEC.md` as the product specification.
- Treat `AGENTS.md` as persistent repository instructions.
- Keep `PLANS.md` updated as work progresses.
- Do not implement real providers yet.
- Initialize the Go module and clean repository structure.
- Implement only the initial CLI needed by Phase 0, including:
  - `arixtv version`
  - `arixtv config validate`
- Add `.gitignore`.
- Add GitHub Actions CI.
- CI must run formatting checks, `go vet ./...`, and `go test ./...`.
- Validate cross-builds for:
  - `linux/amd64`
  - `linux/arm64`
  - `darwin/arm64`
- Keep Linux deployment code separate from the portable Go core.
- Do not introduce EPG, DRM handling/bypass, Redis, Node.js, Chromium, Playwright, FFmpeg, Web UI, database servers, built-in TLS, or media proxying.
- Do not copy FastChannels source code.
- Do not commit secrets or runtime data.
- Run the complete Phase 0 test/build checks before finishing.
- At the end, show:
  1. the repository tree;
  2. files created/changed;
  3. commands/tests run and results;
  4. any design decisions that need owner review;
  5. the next Phase from `PLANS.md`.

Do not proceed into Phase 1 until I explicitly approve Phase 0.
