# Linux deployment boundary

Phase 5 will add Linux installation and systemd assets here. Phase 6 will
add Nginx configuration generation. No deployment scripts or units are
implemented in Phase 0.

Keep host service management outside the portable Go packages. The eventual
installer must place project files under `/etc/arixtv/` and systemd units
under `/etc/systemd/system/`, as specified in `ARIXTV_SPEC.md`.
