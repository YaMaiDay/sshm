# Troubleshooting

This guide covers common issues. Before opening an issue, remove passwords, private keys, tokens, private IPs, and production hostnames from logs or screenshots.

## Installed Version Is Not The Latest

Run:

```sh
sshm --version
```

The install script downloads the latest GitHub Release. If the repository code has changed but a new Release has not been published, the install script still installs the previous Release.

You can pin a version:

```sh
SSHM_VERSION=vX.Y.Z curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
```

You can also download the matching package from Releases manually and verify it with the same-version `checksums.txt`.

## SSH Connection Fails

First confirm the system `ssh` command works:

```sh
ssh -p 22 user@example.com
```

Common causes:

| Symptom | Possible Cause |
| --- | --- |
| `Permission denied (publickey)` | Wrong key path, missing public key on the server, or wrong user |
| `Connection timed out` | IP, port, firewall, or security group is unreachable |
| `No route to host` | Network route is unavailable |
| `Host key verification failed` | `known_hosts` conflict |
| Password login fails | Server disabled password login, or local `sshpass` is missing |

sshm calls the system `ssh`; it does not bypass OpenSSH authentication rules.

## Bastion Connection Fails

The bastion path is:

```text
local machine --SSH--> bastion --SSH--> target server
```

Requirements:

- The local machine can SSH to the bastion.
- The bastion can reach the target server's SSH port.
- The local machine has both the bastion private key and target-server private key, or the local ssh-agent has them loaded.
- The target server's firewall or security group allows access from the bastion.

sshm does not copy the target-server key to the bastion. The target-server key path still points to a local file.

## Monitoring Shows 0 Or Offline

sshm collects monitoring data by running remote commands through SSH. If SSH fails, monitoring fails.

The remote system should have common commands:

```sh
uname
cat
df
awk
sed
free
ps
```

Missing metrics usually mean missing commands, insufficient permissions, or output formats that differ from common Linux systems.

## Why Disk Shows Mount Points Instead Of sda1

On Linux, usage belongs to mounted filesystems, not raw block device names.

Example:

```text
/      device /dev/mapper/cs-root
/boot  device /dev/sda1
/data  device /dev/sdb1
```

If `/dev/sdb3` is not mounted, it has no usable capacity or usage percentage, so sshm does not show it.

sshm reads mounted real filesystems with `df -PT -B1` and filters temporary/system filesystems such as `tmpfs`, `devtmpfs`, `proc`, `sysfs`, and `overlay`.

## Memory Differs From Proxmox

Different systems define "used memory" differently.

sshm is closer to Linux `free` and focuses on available memory:

```text
used = total - available
```

Proxmox may include or exclude cache, host-level statistics, or virtualization-layer data. To determine whether a server is actually under memory pressure, check available memory, swap usage, and business processes first.

## rsync Is Missing

File transfer and local-fetch deployment require rsync.

Both the local machine and the remote server need rsync.

Debian / Ubuntu:

```sh
sudo apt install rsync
```

RHEL / CentOS / Rocky:

```sh
sudo yum install rsync
```

macOS:

```sh
brew install rsync
```

If the remote server is missing rsync, sshm asks whether to try installing it. Without sudo permission, install rsync manually.

## Transfer Picker Shows Root Directories

The settings page has a custom transfer directories switch.

- Custom directories disabled: sshm lists directories directly under `/`.
- Custom directories enabled with empty local or remote values: sshm also lists directories directly under `/`.
- Custom directories enabled with values: those paths are shown as upload/download shortcuts.

On macOS, paths such as `/etc`, `/tmp`, and `/var` are symlink directories. sshm follows local symlinks when deciding whether an entry can be expanded and deduplicates entries that point to the same real path.

## GitHub Fetch Fails

Common errors:

| Error | Meaning |
| --- | --- |
| `Permission denied (publickey)` | SSH key cannot access the repository |
| `Repository not found` | Repository address is wrong, or the token/key has no access |
| `Could not resolve host: github.com` | The side doing the fetch cannot access GitHub |
| `HTTP 401/403` | Token is invalid, expired, or lacks permissions |

First identify the fetch mode:

- Local fetch then upload: check whether the local machine can access GitHub and has the right local credential.
- Remote fetch: check whether the target server can access GitHub and has the right target-server credential.

SSH repository example:

```text
git@github.com:owner/repo.git
```

Release repository field example:

```text
owner/repo
```

## Release Asset Not Found

If the asset field is an exact filename, the GitHub Release must contain an asset with that exact name.

If the filename contains a date or build number, use `*`:

```text
freedex-trade-kernel-amd64-*
```

An empty version or `latest` means the latest Release. `v1.2.3` means the Release for that tag.

If a full download URL is configured, sshm uses it directly and does not build a Release URL.

## Deployment Queue Fails

If any app in a queue fails, later apps do not continue.

On the deploy confirmation/output page:

- Press `r` to retry the failed app.
- Press `a` to redeploy from the first app.
- Press `Esc` to return to the deployment list.

Check the current stage output first. Common failure points are GitHub credentials, rsync, update commands, or health checks.

## What Does An Abnormal Docker Container Mean

sshm summarizes Docker's raw status while also showing the raw status in the detail page.

Common states:

| sshm Status | Docker Raw Status Example | Meaning |
| --- | --- | --- |
| Running | `Up 2 weeks` | Container is running |
| Abnormal | `Up 2 weeks (unhealthy)` | Container is running but its health check fails |
| Restarting | `Restarting (1) 10 seconds ago` | Container is repeatedly restarting |
| Stopped | `Exited (0) 2 hours ago` | Container has exited |

For deeper investigation, run on the target server:

```sh
docker ps -a
docker logs <container>
docker inspect <container>
```

## Resource Manager Keeps Reading Or Shows Old Data

The resource manager shows cached data first when it has a previous snapshot, then refreshes the selected server in the background.

If the page keeps showing "reading resources" or refreshes back to empty:

- Confirm the selected server can still be reached through SSH.
- Confirm the remote commands exist: `docker`, `systemctl`, `ps`, and `ss` or `netstat`.
- For Docker, sshm tries the normal command first and then `sudo -n docker`. If both fail, it reports a permission problem. If neither command exists, Docker is shown as not installed.
- For services, sshm uses a lightweight `systemctl list-units` query for the resource list and only loads heavier details when needed.

Container CPU usage comes from `docker stats`. The configured CPU limit shown after the percentage comes from `docker inspect`. `Unlimited` / `未限制` means Docker did not configure a CPU quota, NanoCPUs, or CPU set for that container.

## Database Resource Cannot Collect Metrics

Database discovery is best-effort. sshm may detect a likely database from a container, service, process, or listening port before it has enough information to connect.

Check:

- The database host and port are reachable from the target server.
- The right database engine is selected.
- The database user has permission to read basic runtime metrics.
- The target server has the required client command available, such as `mysql`, `mariadb`, `psql`, `redis-cli`, `mongosh`, or `mongo`.
- The saved password is correct, if a password is required.

If a database runs inside Docker, also check that the container name is still valid and that the database client can run from the target server.

Saved database passwords are stored locally in `~/.config/sshm/resources.toml` with restricted file permissions where practical. The value is still plain text inside that local file, so do not share it without removing secrets.

## Container Or Service Action Says Permission Denied

The resource view can start, stop, restart, log, and delete Docker containers, and can start, stop, restart, and log systemd services.

For Docker and systemd actions, sshm first runs the normal command. If it fails, sshm retries with `sudo -n`. If sudo also fails, the user account needs permission on the target server.

Common fixes:

- Add the user to the Docker group for Docker actions.
- Configure passwordless sudo for the exact Docker or systemctl commands you want to allow.
- Use a server user that already has the required permission.

sshm does not prompt for a sudo password inside these resource actions.

## What To Include In An Issue

Please provide:

- sshm version: `sshm --version`
- Operating system and CPU architecture
- Terminal application
- Reproduction steps
- Expected behavior
- Actual behavior
- Redacted screenshot or output

Do not provide:

- Passwords
- Private keys
- Tokens
- Private IPs
- Production hostnames
- Full server lists
