# Security Policy

sshm is a local terminal SSH manager. It does not run a cloud service and does not include telemetry or runtime update checks.

## Supported Versions

Security fixes are applied to the latest release.

## Security Design

- Local only: server configuration, command history, and transfer history are stored on the user's machine.
- No telemetry: sshm does not collect analytics, crash reports, or usage data.
- No automatic update checks: running sshm does not contact GitHub or any project-owned server to check for updates.
- Native delegation: SSH login uses the system `ssh` command, and file transfer uses `rsync`.
- No agent installation: sshm does not install a background agent on remote servers.
- No SSH key upload: sshm does not upload private keys to remote servers or project infrastructure.
- Explicit network actions only: network access happens when the user connects to configured servers, runs commands, transfers files, runs the install script, or confirms remote `rsync` installation.

## Local Data

Typical local files:

| File | Purpose |
| --- | --- |
| `~/.config/sshm/servers.toml` | Server configuration |
| `~/.config/sshm/commands.toml` | Command templates |
| `~/.config/sshm/history.toml` | Command history |
| `~/.config/sshm/transfers.toml` | Transfer tasks and history |
| `~/.config/sshm/config.toml` | App configuration |

Sensitive local files are written with restrictive permissions where applicable.

## Reporting a Vulnerability

Please report security issues through GitHub's private vulnerability reporting or Security Advisories for this repository when available.

If private reporting is not available, open a GitHub issue with a minimal description and avoid posting secrets, private keys, server IPs, passwords, or exploit details publicly. The maintainer will follow up and coordinate a fix.

## Scope

In scope:

- Leaks of local server configuration, command history, or transfer history.
- Unintended network requests made by sshm at runtime.
- Command construction bugs that could execute unintended local or remote commands.
- Unsafe handling of passwords, private keys, or temporary files.

Out of scope:

- Vulnerabilities in a user's own SSH server, shell, package manager, or remote system configuration.
- Issues caused by intentionally running untrusted commands through sshm.
- Social engineering or physical access to the user's machine.
