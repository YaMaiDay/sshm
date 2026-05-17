# Contributing

Thanks for helping improve sshm.

sshm is a local-first terminal SSH manager. Please keep these principles intact:

- Default behavior stays local.
- Do not add telemetry, background update checks, or project-owned runtime network requests.
- Do not upload SSH keys, passwords, server lists, command history, or transfer history.
- Prefer system `ssh` and `rsync` instead of reimplementing protocol details.
- English is the primary language for GitHub-facing docs and metadata. Chinese is supported as a secondary UI language.

## Development Environment

Requirements:

- Go 1.24 or newer.
- `ssh`, used for login and monitoring collection.
- `rsync`, used for file transfer.
- `sshpass`, optional and only used for automated password login.

Run checks:

```sh
go test ./...
go vet ./...
git diff --check
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

Before opening a PR, confirm:

1. The change is scoped and does not include unrelated edits.
2. `go test ./...` passes.
3. `go vet ./...` passes.
4. `git diff --check` passes.
5. User-visible behavior changes are reflected in README, `docs/`, or `CHANGELOG.md`.
6. The PR avoids unrelated formatting churn.
7. The user-visible impact is clearly described.

Prefer small, reviewable commits. Keep documentation, tests, refactors, and behavior changes separate when practical so maintainers can review or revert them independently.

## Bug Reports

Please include:

- sshm version.
- Operating system and CPU architecture.
- Terminal application.
- Reproduction steps.
- Expected behavior.
- Actual behavior.
- Redacted logs or screenshots.

Do not publicly post passwords, private keys, private server IPs, or production hostnames.

## Feature Requests

Please describe:

- The workflow you want to improve.
- Why the current behavior is not enough.
- How you expect sshm to work.
- Whether it affects SSH login, remote commands, file transfer, deployment, or local configuration.

## Security Issues

Do not disclose sensitive security details in public issues. Use GitHub private vulnerability reporting or Security Advisories when available. See `SECURITY.md` for details.
