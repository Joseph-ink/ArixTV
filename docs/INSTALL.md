# ArixTV Installation Design

## Scope

V1 provides an interactive Linux installer.

The installer manages ArixTV, not the entire host.

It does not:
- install or reconfigure Nginx automatically;
- obtain TLS certificates;
- configure firewalls;
- configure VPN software.

It may detect Nginx and print the appropriate configuration.

## Supported deployment targets

Primary:
- Ubuntu
- Debian
- Oracle Linux
- practical RHEL-compatible systems

Architectures:
- amd64
- arm64

macOS arm64 uses the released binary directly; V1 does not provide a launchd installer.

## Required Linux layout

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

Systemd:

```text
/etc/systemd/system/arixtv.service
/etc/systemd/system/arixtv-refresh.service
/etc/systemd/system/arixtv-refresh.timer
```

Optional:

```text
/etc/systemd/system/arixtv-relay.service
```

## Environment detection

Installer should detect:
- Linux kernel;
- effective privileges;
- systemd;
- CPU architecture;
- distro family/version;
- network interfaces;
- interface addresses;
- candidate private LAN CIDRs;
- port availability;
- current ArixTV installation;
- Nginx presence;
- `curl` or `wget`;
- archive/checksum tools.

Unsupported environments should fail before performing a partial install.

## First-run flow

Suggested flow:

```text
ArixTV Setup

1. Environment
2. Network / LAN
3. Remote Direct
4. Providers
5. Optional Relay
6. Refresh Schedule
7. Access Tokens
8. Review
9. Install
```

## LAN setup

Detect candidate private networks.

Example:

```text
Detected networks:

1. 192.168.50.0/24 via eth0
2. 10.0.0.0/24 via eth1
3. Enter custom CIDR
```

Allow multiple CIDRs.

Validate user input before writing configuration.

Prefer binding explicit LAN interface addresses.

## Remote Direct setup

Ask whether Remote Direct is enabled.

If enabled:

```text
Public domain: tv.example.com
```

ArixTV public direct base becomes:

```text
https://tv.example.com/direct
```

The installer prints Nginx config only.

## Provider selection

Provider choices should come from the ArixTV binary/provider registry rather than being duplicated as a hard-coded Bash list whenever practical.

Default-enable only stable recommended providers.

## Refresh schedule

Use systemd timer.

Default concept:
- shortly after boot;
- every 6 hours;
- randomized delay;
- persistent missed-run handling.

Allow user customization.

## System user

Create a dedicated `arixtv` system account.

Do not run the daemon as root.

Set directory ownership so only required state paths are writable.

## Existing installation

When `/etc/arixtv/` already exists, management mode should offer:

1. Reconfigure
2. Update
3. Providers
4. LAN
5. Remote Direct
6. Relay
7. Refresh Schedule
8. Refresh Now
9. Restart Services
10. Status
11. Nginx Configuration
12. Uninstall

## Reconfiguration

Safe sequence:

1. create timestamped backup;
2. write candidate config;
3. validate config;
4. validate CIDRs/ports;
5. atomically replace active config;
6. restart/reload if required;
7. perform health check;
8. restore backup if health validation fails.

## Updating

Update flow:

1. query GitHub Release metadata;
2. select matching Linux architecture;
3. download release asset;
4. download/check SHA256;
5. preserve current binary;
6. atomically install new binary;
7. restart;
8. health check;
9. roll back binary on failure.

Never overwrite configuration during an update.

## Complete uninstall

Uninstall should:

1. ask for explicit confirmation;
2. stop ArixTV services;
3. disable units;
4. stop/disable optional relay;
5. remove unit files;
6. `systemctl daemon-reload`;
7. `systemctl reset-failed`;
8. remove installer-installed relay;
9. remove `/etc/arixtv/`;
10. remove installer-created `arixtv` system user/group.

Do not remove:
- Nginx;
- certificates;
- unrelated packages;
- unrelated configuration.

The uninstall routine should copy final cleanup logic outside `/etc/arixtv/` before deleting the directory that contains itself.
