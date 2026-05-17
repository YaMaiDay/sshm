# Remote Script Security Notes

sshm intentionally runs remote commands. Monitoring, resource discovery, command templates, file transfer, deployment, and database metrics all rely on SSH or rsync. The project keeps these paths explicit so generated scripts and user-provided shell are not confused.

## Boundaries

| Boundary | Meaning |
| --- | --- |
| Generated script | Shell text built by sshm from structured values such as resource names, ports, paths, and database fields |
| User script | Shell text entered by the user, such as command templates, deployment stages, and managed resource custom commands |
| Argument list | Local process execution through `exec.Command` without passing through a local shell |
| Remote shell | A script sent to the target server through system `ssh` |

## Generated Scripts

Generated scripts must quote host, resource, path, and database identifiers before embedding them in shell.

Shared helpers live in `internal/remotescript`:

- `Quote`: quote shell values when needed.
- `SingleQuote`: always single-quote shell values.
- `EnvName`: validate environment variable names before use.
- `SudoFallback`: build the normal-command then `sudo -n` retry pattern.

Examples of generated script paths:

- Resource discovery and built-in resource actions in `internal/resource`.
- Database monitor scripts in `internal/dbmonitor`.
- Deployment fetch scripts in `internal/deployment`.
- Remote directory selection in `internal/fsselect`.

Built-in resource actions only accept the fixed actions `start`, `stop`, and `restart`. Unsafe action names are rejected instead of interpolated.

## User-Provided Scripts

These values are explicit user shell and are executed as-is:

- Command template body.
- Batch command body.
- Deployment pre-update, update, post-update, health-check, rollback, and custom fetch commands.
- Managed resource custom start, stop, restart, log, health, and delete commands.

The command service uses `remotescript.UserScript` to mark this boundary in code. The wrapper does not sanitize user shell; it documents that the caller is intentionally executing user-provided shell.

Users should not paste untrusted commands into sshm. Do not store token values, private-key contents, or production passwords inside command bodies.

## Local Process Execution

File transfer and SSH helpers build local processes through `exec.Command` argument lists. Local SSH, SCP, and rsync arguments do not pass through a local shell.

The remote side can still run shell scripts when sshm intentionally sends a script through SSH, such as monitoring, resource discovery, or deployment stages.

## Database Metrics

Database monitor connection fields are quoted before interpolation. This covers host, port, user, password, database, container name, and query-related command parameters.

Database passwords can be saved for managed database resources. Saved values are local plain text in `resources.toml` with restricted file permissions where practical.

## Execution Results

Remote command result shape is centralized in `internal/execresult`. This keeps output, exit code, and error semantics consistent across actions, command templates, deployment, resources, and TUI messages.

## Regression Coverage

- `internal/remotescript` tests cover quoting, environment name validation, and sudo fallback script construction.
- `internal/resource` tests cover shell quoting and reject unsafe built-in action command names.
- `internal/dbmonitor` tests cover quoting database connection fields.
- `internal/actions` tests cover SSH/SCP/rsync argument construction.
- `internal/smoke` contains gated real remote smoke for SSH, rsync, and deployment paths.
