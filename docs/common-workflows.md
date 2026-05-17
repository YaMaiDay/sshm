# Common Workflows

This page covers the day-to-day workflows that start from the dashboard or from small command-line helpers.

## Command-Line Helpers

Start the TUI:

```sh
sshm
```

List parsed servers and exit:

```sh
sshm --list
```

Collect monitoring data for one server and exit:

```sh
sshm --probe demo-web
```

If the same server name exists in multiple categories, use `category/name`:

```sh
sshm --probe production/demo-web
```

List common remote directories for one server:

```sh
sshm --remote-dirs demo-web
```

Print the app settings path:

```sh
sshm --config-path
```

Print the installed version:

```sh
sshm --version
```

## Dashboard Navigation

| Key | Action |
| --- | --- |
| `?` | Open full shortcut help |
| `↑` / `↓` / `←` / `→` / `h` / `j` / `k` / `l` | Move selection |
| `Tab` | Switch category |
| `z` | Switch dashboard view: cards, group, category |
| `s` | Switch sort |
| `/` | Search |
| `o` | Toggle online-only filter |
| `p` | Toggle problems-only filter |
| `v` | Toggle favorites-only filter |
| `r` | Refresh monitoring |
| `q` / `Esc` | Quit or go back |

## Server Management

| Key | Action |
| --- | --- |
| `a` | Add server |
| `c` | Copy selected server |
| `e` | Edit selected server |
| `x` | Delete selected server, with confirmation |
| `t` | Pin or unpin selected server |
| `f` | Favorite or unfavorite selected server |
| `Space` | Open server details |
| `Enter` | Open SSH for selected server |

The server form can also manage categories. Normal categories can be created, renamed, and deleted when empty. The fixed `Bastion` category is reserved for jump hosts.

## Monitoring And Details

Use the dashboard for a high-level view. Press `Space` on a server for details.

The detail page shows:

- System overview.
- CPU, memory, swap, mounted disks, and inode usage.
- Docker and service summaries.
- Health port checks.
- Login summary and SSH risk hints.
- Resource and problem summaries.

Useful keys in detail pages:

| Key | Action |
| --- | --- |
| `↑` / `↓` / `j` / `k` | Scroll |
| `Tab` / `←` / `→` | Switch detail section |
| `Enter` | Open SSH |
| `m` | Command templates for this server |
| `n` | Resource manager for this server |
| `u` / `d` | Upload or download |
| `r` | Refresh |
| `q` / `Esc` | Back |

## Command Templates

Press `m` from the dashboard or detail page.

Command templates can be global or server-scoped. sshm executes template commands as user-provided shell on the selected target server and records command history locally.

Common keys:

| Key | Action |
| --- | --- |
| `a` | Add template |
| `e` | Edit template |
| `x` | Delete template |
| `Enter` | Run template |
| `q` / `Esc` | Back |

Press `i` on the dashboard to view command history. From history you can rerun a previous command or delete a record.

## Batch Commands

Press `b` from the dashboard.

Flow:

1. Select target servers.
2. Pick an existing command template or enter a temporary command.
3. Confirm the batch.
4. Review per-server results.

Batch execution is serial. Failed targets are visible in the result page and can be retried.

## File Transfer

Press `u` to upload or `d` to download.

sshm uses `rsync`. Both the local machine and the target server need `rsync` for transfers. If remote `rsync` is missing, sshm asks whether it should try to install it.

Transfer jobs continue while you return to the dashboard. Press `y` to open transfer jobs and history.

## Resource Manager

Press `n` on a server.

The resource manager covers Docker containers, systemd services, processes, listening ports, and databases. See [Resources](resources.md) for the full model.

Common keys:

| Key | Action |
| --- | --- |
| `Tab` | Switch resource type |
| `Space` | Detail |
| `a` | Add managed resource |
| `e` | Edit managed resource or configure database |
| `x` | Remove managed resource |
| `t` | Pin |
| `f` | Favorite |
| `v` | Favorites-only |
| `g` | Switch status or scope filter |
| `z` | Switch card/list view |
| `y` | Sort |
| `o` | Logs |
| `s` / `p` / `c` | Start, stop, restart |
| `r` | Refresh |
| `/` | Search |
| `q` / `Esc` | Back |

## App Deployment

Press `g` from the dashboard.

Deployment apps are global. Each app selects one target server, so running an app connects only to that configured server. See [Deployment](deployment.md) for fetch modes, credentials, queues, history, and rollback.

## Settings

Press `.` from the dashboard.

Settings include language, ASCII mode, refresh interval, SSH connect timeout, remote command timeout, warning thresholds, and transfer shortcut directories.
