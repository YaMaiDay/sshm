# Resource Manager

The resource manager lets you inspect and operate resources on one selected server without installing a remote agent.

Press `n` on a dashboard server to open it.

## Resource Types

| Resource | What sshm discovers |
| --- | --- |
| Containers | Docker containers from `docker ps`, `docker stats`, and `docker inspect` |
| Services | systemd services from `systemctl list-units` and `systemctl show` |
| Processes | Processes from `ps` and `/proc` details |
| Ports | Listening TCP/UDP ports from `ss` or `netstat` |
| Databases | MySQL, MariaDB, PostgreSQL, Redis, and MongoDB inferred from containers, services, processes, and ports |

Discovery is best-effort. If a command is unavailable or the user lacks permission, sshm reports the missing command or permission problem instead of hiding it silently.

## Cache Behavior

sshm keeps a local resource cache per server and resource type in:

```text
~/.config/sshm/resource_cache.toml
```

When cached data exists, sshm shows it immediately and refreshes in the background. This avoids an empty resource page while remote discovery is still running.

Cached data is a convenience snapshot. The remote server remains the source of truth.

## Managed Resources

Discovered resources can be added to the managed list for faster access, pinning, favorites, and custom actions.

Managed resource configuration is stored in:

```text
~/.config/sshm/resources.toml
```

Containers and services are rediscovered on every refresh. Removing a discovered container from sshm only removes its local managed entry; it does not delete the Docker container on the server.

## Containers

Container cards show:

- Status and raw Docker status.
- Image.
- Ports.
- CPU and memory usage.
- Configured CPU limit when Docker exposes it.

Details can include Docker inspect output, size, block I/O, mounts, networks, labels, and related port information.

Actions:

- Logs.
- Start.
- Stop.
- Restart.

Built-in Docker actions first try the normal `docker` command. If that fails, sshm retries with `sudo -n docker`. sshm does not prompt for sudo passwords inside resource actions.

## Services

Service cards show systemd state, description, unit file state, memory, PID, working directory, and command information when available.

Details can include systemd properties such as active state, sub state, main PID, restart policy, unit file path, user, group, timestamps, and recent logs.

Actions:

- Logs through `journalctl`.
- Start.
- Stop.
- Restart.

Built-in systemd actions first try normal `systemctl`. If that fails, sshm retries with `sudo -n systemctl`.

## Processes

Process resources show PID, parent PID, user, state, CPU, memory, elapsed time, command, working directory, executable path, and cgroup information when available.

Processes are inspection targets by default. To run custom actions for a process-like workload, add it as a managed resource and configure explicit commands.

## Ports

Port resources show listening protocol, address, port, state, process, related service, and related container when sshm can infer them.

Port filters distinguish system, Docker, app, specific-address, container, and process ports. Ports are usually inspection targets rather than action targets.

## Databases

Database resources cover:

- MySQL.
- MariaDB.
- PostgreSQL.
- Redis.
- MongoDB.

sshm discovers likely databases from:

- Container names and images.
- systemd service names.
- Process names.
- Common listening ports.

Discovery does not guarantee that sshm can connect to the database. Save database connection settings before collecting database metrics.

Database details can include:

- Version.
- Uptime.
- Storage size.
- Data size.
- Index size.
- Connection count.
- Table count or keyspace details.
- Cache and engine-specific runtime metrics when available.

If a database password is saved, it is stored locally in plain text inside:

```text
~/.config/sshm/resources.toml
```

The file is written with restricted permissions where practical, but the value is still plain text. Do not commit, upload, or share `resources.toml` without removing sensitive values.

## Custom Commands

Managed resources can store custom start, stop, restart, log, health, and delete commands where the UI supports them.

Custom commands are user-provided shell. sshm executes them as-is on the target server. This is intentional so operators can express service-specific behavior, but it also means the user is responsible for command content.

Generated built-in scripts and user-provided commands are separate security boundaries. See [Remote Script Security](remote-script-security.md).

## Permissions

Common permission fixes:

- Add the SSH user to the Docker group for Docker actions.
- Configure passwordless sudo for exact Docker, systemctl, journalctl, or database client commands you want to allow.
- Use an operations account with the required permissions.

sshm does not collect sudo passwords in resource actions. If `sudo -n` fails, fix permissions on the target server.

## Troubleshooting

See [Troubleshooting](troubleshooting.md) for resource refresh failures, Docker status interpretation, permission errors, database connection failures, and missing remote commands.
