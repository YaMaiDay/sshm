# Security Policy

sshm is a local-first terminal SSH manager. It does not run a cloud service, does not include telemetry, and does not automatically check for updates at runtime.

## Supported Versions

Security fixes are applied to the latest Release.

## Security Design

- Local-first: server configuration, command history, and transfer history stay on the user's machine.
- No telemetry: sshm does not collect analytics, crash reports, or usage data.
- No automatic update checks: running sshm does not contact GitHub or project infrastructure to check for updates.
- System tools: SSH login uses the system `ssh`; file transfer uses `rsync`.
- No remote agent: sshm does not install a background agent on remote servers.
- No private-key upload: sshm does not upload private keys to remote servers or project infrastructure.
- Local bastion keys: bastion mode uses OpenSSH `ProxyJump`; both bastion and target-server private-key paths point to local files and are not copied to the bastion.
- Explicit network access: network access happens only when the user connects to servers, runs commands, transfers files, runs the install script, or confirms remote `rsync` installation.

## Bastion Connections

A bastion is a normal server configuration kept in the fixed `Bastion` category. Internal servers reference the bastion by `jump_host_ref`.

When connecting, sshm creates a temporary OpenSSH config locally and uses `ProxyJump` so system `ssh` / `rsync` reaches the target server through the bastion. The temporary config stores only connection parameters and local private-key paths; it does not contain private-key contents.

Security boundaries:

- The local machine must hold both the bastion private key and target-server private key, or use a local ssh-agent.
- The bastion only needs network access to the target server's SSH port.
- The target server does not need public SSH access; it only needs to allow access from the bastion.
- If the local machine or bastion is compromised, the attacker may use the existing network path to reach internal servers. Use least-privilege accounts, firewall allowlists, and read-only deploy keys where possible.

## Local Data

Common local files:

| File | Purpose |
| --- | --- |
| `~/.config/sshm/servers.toml` | Server configuration |
| `~/.config/sshm/commands.toml` | Command templates |
| `~/.config/sshm/history.toml` | Command history |
| `~/.config/sshm/transfers.toml` | Transfer jobs and history |
| `~/.config/sshm/deployments.toml` | Deployment apps and records |
| `~/.config/sshm/resources.toml` | Resource favorites, pins, custom resource commands, and configured database connection fields |
| `~/.config/sshm/config.toml` | App settings |

Files that may contain sensitive data are written with stricter permissions where practical.

Deployment configuration stores app metadata, repository references, paths, command stages, credential references, and execution records. Do not put GitHub token values, private-key contents, or production passwords into deployment commands. For private Git repositories, prefer least-privilege Deploy Keys. For local fetch, credential parameters reference local private-key paths or local environment variable names; for remote fetch, they reference target-server private-key paths or environment variable names.

Resource configuration may include database connection fields. If a database password is saved, it is stored locally in `resources.toml` with restricted file permissions, but the value is still plain text inside that local file. Do not commit, upload, or share `resources.toml` without removing database passwords and other sensitive values.

## Reporting A Vulnerability

Please use GitHub private vulnerability reporting or the repository's Security Advisories when available.

If private reporting is unavailable, open a GitHub Issue with the minimum necessary description. Do not publicly include passwords, private keys, server IPs, production hostnames, or directly exploitable details. Maintainers will follow up and coordinate a fix.

## Scope

In scope:

- Local server configuration, command history, or transfer history disclosure.
- Unexpected network requests at runtime.
- Command construction bugs that cause unexpected local or remote command execution.
- Unsafe password, private-key, or temporary-file handling.
- Unsafe temporary SSH config or connection-parameter handling in bastion mode.

Out of scope:

- Vulnerabilities in the user's own SSH servers, shells, package managers, or remote system configuration.
- Issues caused by users intentionally running untrusted commands through sshm.
- Social engineering or physical access to the user's machine.
