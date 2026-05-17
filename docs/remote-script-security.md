# Remote Script Security Notes

sshm intentionally runs remote shell commands for monitoring, resources, transfers, and deployments. The boundary is:

- Generated scripts must quote host/resource/database identifiers before embedding them in shell.
- Built-in resource actions only allow `start`, `stop`, and `restart`.
- Deployment commands and managed resource custom commands are explicit user-provided shell and are executed as-is.
- Database monitor connection fields are shell-quoted before script interpolation.
- File transfer and SSH arguments are passed through `exec.Command` argument lists rather than a local shell.

Regression coverage:

- `internal/resource` tests cover shell quoting and reject unsafe built-in action command names.
- `internal/dbmonitor` tests cover quoting database connection fields.
- `internal/smoke` contains gated real remote smoke for SSH, rsync, and deployment paths.
