# Contributing to sshm

Thank you for taking the time to improve sshm.

sshm is a local-first terminal SSH manager. Contributions should preserve these project principles:

- Keep runtime behavior local by default.
- Do not add telemetry, background update checks, or project-owned network calls.
- Do not upload SSH keys, passwords, server lists, command history, or transfer history.
- Prefer system `ssh` and `rsync` behavior over custom protocol implementations.
- Keep the TUI Chinese-first and terminal-friendly.

## Development Setup

Requirements:

- Go 1.24 or newer.
- `ssh` for login and monitor collection.
- `rsync` for file transfer.
- `sshpass` is optional and only needed for password-based automation.

Run tests:

```sh
go test ./...
```

Build locally:

```sh
go build -o sshm ./cmd/sshm
```

Run:

```sh
./sshm
```

## Pull Requests

Before opening a pull request:

1. Keep the change focused.
2. Run `go test ./...`.
3. Update README or Wiki text when behavior changes.
4. Avoid unrelated formatting churn.
5. Include a clear explanation of user-facing impact.

## Bug Reports

Good bug reports include:

- sshm version.
- Operating system and CPU architecture.
- Terminal app.
- Reproduction steps.
- Expected behavior.
- Actual behavior.
- Relevant logs or screenshots with secrets removed.

Do not post passwords, private keys, private server IPs, or production hostnames publicly.

## Feature Requests

Feature requests should explain:

- The workflow you are trying to improve.
- Why current behavior is insufficient.
- How the proposed behavior should work.
- Whether it affects SSH login, remote commands, file transfer, or local config.

## Security Issues

Please do not disclose sensitive security details in public issues. Use GitHub private vulnerability reporting or Security Advisories when available. See `SECURITY.md`.
