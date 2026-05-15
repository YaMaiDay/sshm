# App Deployment

sshm deployment helps you release applications without logging into servers manually. It can fetch Git repositories or GitHub Release assets, run command stages, record results, and run rollback commands when needed.

It does not install a remote agent. Deployment reuses the same SSH, bastion, and rsync connection logic used by monitoring, command execution, and file transfer.

## Entry Point

Press `g` on the dashboard to open app deployments.

Common keys:

| Key | Action |
| --- | --- |
| `a` | Add deployment app |
| `e` | Edit deployment app |
| `x` | Delete deployment app, with confirmation |
| `Space` | Show details |
| `Enter` | Open deploy confirmation |
| `s` | Select multiple apps and deploy them serially in selection order |
| `t` | Pin |
| `f` | Favorite |
| `v` | Show favorites only |
| `z` | Switch card/list view |
| `Tab` | Switch between server categories that contain deployment apps |
| `Esc` | Go back |

Deployment apps are global. Each app is bound to one target server, so pressing `g` shows all deployment apps, and running one app only connects to the server configured for that app.

## Deploy Flow

A normal deployment runs these stages in order:

1. Pre-update
2. Fetch resource
3. Upload resource, only when using local fetch then upload
4. Update
5. Post-update
6. Health check

Stages without commands are skipped. The fetch-resource stage is generated from source, fetch mode, repository, version, asset, path, and credential settings by default. It can also be customized in the deployment flow editor.

Rollback is not part of the deploy flow. Press `r` on the deployment output page to open rollback confirmation. Confirming rollback only runs the rollback commands.

## Resource Source

| Source | Best For | Behavior |
| --- | --- | --- |
| Git | A server directory that is the source repository | Clone or pull a branch |
| Release | Built artifacts, binaries, or archives | Download a Release asset, extract it into a version directory, and switch `current` |

Git example:

| Field | Example |
| --- | --- |
| Source | Git |
| Repository | `git@github.com:owner/api.git` |
| Branch | `main` |
| Project path | `/opt/api` |

Release example:

| Field | Example |
| --- | --- |
| Source | Release |
| Repository | `owner/api` |
| Version | `latest` or `v1.2.3` |
| Asset / match | `api-linux-amd64.tar.gz` or `api-linux-amd64-*` |
| Project path | `/opt/api` |

If a full download URL is configured, sshm uses that URL directly and does not build a GitHub Release URL.

## Fetch Mode

| Fetch Mode | GitHub Access Happens On | Credential Lives On |
| --- | --- | --- |
| Local fetch then upload | Local machine | Local machine |
| Remote fetch | Target server | Target server |

Local fetch then upload:

1. Target server runs pre-update commands.
2. Local machine clones the repository or downloads the Release asset into a temporary directory.
3. Local machine uploads the resource to the target server project path with rsync.
4. Target server runs update, post-update, and health-check commands.

Remote fetch:

1. Target server runs pre-update commands.
2. Target server clones, pulls, or downloads the Release asset.
3. Target server runs update, post-update, and health-check commands.

If the target server cannot access GitHub, use local fetch then upload. If the target server can access GitHub and already has the right credentials, remote fetch is simpler.

## GitHub Credentials

sshm stores only credential references. It does not store private key contents or token values.

| Credential Type | Local Fetch Then Upload | Remote Fetch |
| --- | --- | --- |
| None | Public repository, or local environment already has access | Public repository, or target-server environment already has access |
| SSH Key | Local private key path, for example `~/.ssh/api_deploy_key` | Target-server private key path, for example `/home/deploy/.ssh/api_deploy_key` |
| Token | Local environment variable name, for example `GITHUB_TOKEN` | Target-server environment variable name, for example `GITHUB_TOKEN` |

Recommendations:

- Prefer GitHub Deploy Keys for private Git repositories.
- Use minimal tokens for private Release assets.
- Do not put token values or private key contents in deployment commands.
- Do not use long-lived personal account tokens for server deployment.

## Command Stage Examples

Pre-update:

```sh
systemctl stop api || true
cp -a current backup/$(date +%Y%m%d%H%M%S)
```

Update:

```sh
npm ci --omit=dev
npm run build
```

Post-update:

```sh
systemctl restart api
```

Health check:

```sh
curl -fsS http://127.0.0.1:8080/health
```

Rollback:

```sh
ln -sfn releases/previous current
systemctl restart api
```

These commands run inside the project path. If root privileges are required, use `sudo` explicitly and make sure the target server account can run the command.

## Serial Deployment

In the deployment list, press `s` to select multiple apps, then press `Enter` to open queue confirmation.

Queue rules:

- Apps run serially in the order selected.
- If one app fails, the whole queue stops.
- After a failure, press `r` to retry the failed app.
- After a failure, press `a` to redeploy from the first app.
- After each successful app, sshm waits for that app's configured wait time before starting the next app.

Wait time is useful for rolling releases. For example, deploy API first, wait 10 seconds, then deploy Web.

## History

Each deploy or rollback records:

- App
- Server
- Action: deploy or rollback
- Status: success or failed
- Previous version
- Current version
- Exit code
- Output

Deploy confirmation and detail pages show up to the latest 50 related records. The global deployment record list is capped; newer records replace older ones when the limit is exceeded.

## Bastion Hosts

If the target server is configured with a bastion host, deployment automatically reuses the same connection logic.

From the user's perspective, a target server behind a bastion behaves like a normal server:

- Monitoring collects target-server metrics.
- Commands run on the target server.
- File transfers go to the target server.
- App deployment deploys to the target server.

The bastion is only one hop in the SSH path. It is not the deployment target unless the deployment app explicitly selects that bastion server.
