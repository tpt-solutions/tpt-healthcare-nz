# GitHub org setup checklist — tpt-solutions

Checklist for finishing the move of `tpt-healthcare-nz` from a personal account (`PhillipC05`) into the `tpt-solutions` GitHub org. These are all github.com settings — nothing here is enforced by code, so track completion manually.

Repo model: no external pull requests, contributions via issues only, maintainers/AI agents push directly to `master` once CI (`.github/workflows/ci.yml`) is green.

## Org security defaults

- [ ] Require two-factor authentication for all org members.
- [ ] If your GitHub plan supports it, enable SAML SSO for the org.
- [ ] Set base repository permissions to "No permission" or "Read" — grant write access per-team, not org-wide.
- [ ] Restrict who can create/transfer repos in the org to owners.
- [ ] Disable outside collaborators unless a specific one is needed.

## Teams

- [ ] Create a maintainers/admin team with write+admin access to this repo.
- [ ] If AI agents commit under dedicated bot accounts or PATs, create a bot/automation team scoped to just this repo (avoid granting org-wide access to automation credentials).

## Branch protection on `master`

Since there's no PR review step, protection should focus on process safety rather than review gates:

- [ ] Require the `CI` status checks (from `.github/workflows/ci.yml`) to pass before a push is accepted.
- [ ] Restrict who can push directly — only the maintainers team and any bot accounts.
- [ ] Block force-pushes and branch deletion on `master`.
- [ ] Consider requiring signed commits if you want provenance on direct pushes.

## Repo transfer follow-ups

- [ ] Confirm `github.com/PhillipC05/tpt-healthcare-nz` redirects to the new org URL (GitHub sets this up automatically on transfer, but verify it resolves).
- [ ] Re-check any external webhooks/integrations (issue trackers, deploy hooks, monitoring) that referenced the old owner — some third-party integrations need re-authorizing after a transfer.
- [ ] Regenerate any deploy keys, PATs, or SSH keys that were scoped to the old personal account.
- [ ] Update the `install.tpt.health` download endpoint (referenced in `installer/scripts/install.sh` and `install.ps1`) if it depends on GitHub Releases from the org — confirm release URLs resolve under `tpt-solutions/tpt-healthcare-nz`.

## Actions & secrets

- [ ] Enable GitHub Actions for the repo at the org level (Settings → Actions → General).
- [ ] The current `ci.yml` workflow (lint/test/build only) needs no secrets — Postgres/Redis for integration tests are provisioned by `testcontainers-go` at runtime, not via workflow services.
- [ ] When a deploy/publish step is added later, re-create any needed secrets here (e.g. container registry credentials, deploy tokens) — none exist today.

## Repo settings

- [ ] Confirm visibility (public/private) carried over correctly from the transfer.
- [ ] Set the org's social preview image/description for the repo.
- [ ] Disable unused features (Wikis, Projects, Discussions) if you want a lean repo — Issues stays enabled since it's the only contribution channel.
- [ ] Confirm the issue templates under `.github/ISSUE_TEMPLATE/` (bug report, feature request) render correctly in the "New issue" picker.

## Security policy

- [ ] Confirm `security@tpt.nz` (from `SECURITY.md`) is monitored under the new org.
