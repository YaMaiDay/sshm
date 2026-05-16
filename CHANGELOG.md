# Changelog

This file records user-visible changes. Historical release notes are available in GitHub Releases.

## v0.1.48 - 2026-05-16

### Changed

- Document where the installed executable and user data files live after installation.
- Use lighter resource type symbols for services, processes, and ports to avoid conflicting with the pinned marker.

## v0.1.47 - 2026-05-16

### Added

- Add resource pinning with `t`, using the same pinned-first behavior as the dashboard.
- Add resource sorting with `y`, cycling through default, status, name, CPU, memory, and port ordering.

## v0.1.46 - 2026-05-16

### Changed

- Make resource action shortcuts consistent between the resource list and detail pages: start `s`, stop `p`, restart `c`, and refresh `r`.
- Resource start, stop, and restart actions still require confirmation before running.

## v0.1.45 - 2026-05-16

### Added

- Add a dedicated resource manager for the selected server, covering Docker containers, systemd services, processes, and listening ports with card/list views, search, filters, details, logs, and confirmed start/stop/restart actions.
- Resource discovery now keeps a local cache per server and resource type so reopening the page can show the last known resources before refreshing remote data.
- Resource actions automatically retry with `sudo -n` when the normal command fails, then report permission errors clearly if sudo also fails.
- Container cards show the configured CPU limit from Docker inspect data during list loading, so users do not need to open container details first.

### Changed

- Server details now show service and container summary counts inside resource monitoring instead of duplicating the full service/container pages.
- Docker containers are treated as auto-discovered resources: they can be favorited or unfavorited, but removal is blocked with a clear message.
- Resource lists now show localized status labels alongside the original raw state for clearer scanning.

## v0.1.44 - 2026-05-15

### Changed

- Complete English-by-default UI coverage across dashboard, server details, transfers, command templates, deployment, settings, and anomaly views.
- Make settings fully active: ASCII mode changes progress bars, command timeout applies to commands and deployment, and warning thresholds drive dashboard colors, risks, and problem filters.
- Keep dashboard search in the current view instead of switching cards to list view.
- Clarify custom transfer directory behavior: disabled or empty custom directories list `/` entries; enabled values become upload/download shortcuts.
- Treat local symlink directories as expandable directories in transfer pickers and deduplicate entries by real path.

## v0.1.43 - 2026-05-15

### Changed

- Convert CLI flag descriptions, probe output, and SVG demo assets to English for GitHub-facing presentation.

## v0.1.42 - 2026-05-15

### Added

- Add a settings page for language, refresh interval, timeouts, ASCII mode, warning thresholds, and common transfer directories.
- Improve server detail service rendering with systemd service status groups, failed-first ordering, and clearer status colors.

### Changed

- Use `.` as the dashboard settings shortcut, including fullwidth `。` input normalization.
- Render the dashboard shortcut help and settings page in English by default, with Chinese shown when the language is set to `zh`.
- Remove app deployment from standalone GitHub documentation navigation while keeping it documented as a feature.

## v0.1.40 - 2026-05-15

### Added

- Add app deployment with Git and GitHub Release resource sources.
- Support local fetch then upload and remote fetch deployment modes.
- Support SSH Key, Token, and no-credential GitHub access modes.
- Support pre-update, fetch resource, update, post-update, health check, and rollback command stages.
- Support serial deployment queues ordered by user selection.
- Support per-app wait time after successful deployment.
- Support deployment history with up to 50 related records on confirmation and detail pages.
- Support card/list views, category filtering, pinning, favorites, and favorites-only mode for app deployments.
- Show Docker raw status in container details so users can distinguish running, abnormal, restarting, and stopped containers.

### Changed

- Bastion configuration now selects an existing bastion server; bastions are kept in a fixed separate category.
- Bastion documentation now clarifies that target-server private keys stay on the local machine and are not copied to the bastion.
- Disk monitoring now shows real mount points and lists device, filesystem type, capacity, available space, and usage in details.
- Deploy confirmation and queue pages now use a unified execution-stage and current-output display.
- Deployment app deletion now requires confirmation.
- Deployment forms, lists, cards, and details now match dashboard styling.

### Engineering

- Split large TUI files to reduce maintenance cost around deployment, rendering, and type definitions.
- Add regression tests for SSH options, ProxyJump, file picking, deployment queues, deployment rendering, and deployment execution helpers.
- Add `go vet` and whitespace checks to CI and release workflows.

## v0.1.39 And Earlier

See GitHub Releases.
