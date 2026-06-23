# Releasing glazier

This document codifies how work flows through branches and how a tagged release is cut. It is the authoritative description of the process the [release workflow](.github/workflows/release.yaml) depends on.

## Branching model

glazier uses a two-trunk model (Git Flow, minus the optional `release/*` branches):

- **`main`** is production. Every commit on `main` is a known-good, releasable state. Nothing lands here except reviewed merges from `develop` (or a `hotfix/*` branch - see below).
- **`develop`** is the integration branch. Day-to-day work merges here first and stabilizes before it is promoted to `main`.

```
feature/fix branch ──PR──▶ develop ──PR──▶ main ──tag──▶ release workflow
        ▲                                    │
        └──────────── hotfix/* ──────────────┘
```

### The normal flow

1. Cut a working branch off `develop`.
2. Open a PR back into `develop`. CI (`make test`, `race`, `lint`, `vet`) must be green; get the review.
3. Periodically, open a PR to merge `develop` into `main`.
4. On `main`, cut a semver tag and push it. The release workflow does the rest (see [Cutting a release](#cutting-a-release)).
5. After the release, merge `main` back into `develop` so the two trunks stay reconciled.

### Hotfixes

When production needs a fix that can't wait for whatever is currently stabilizing on `develop`:

1. Cut `hotfix/<description>` **off `main`**, not `develop`.
2. PR it into `main`, tag, and release as usual.
3. Merge `main` back into `develop` so the fix isn't lost on the next promotion.

This is the one path that bypasses `develop`, and it exists precisely so an urgent fix never has to ship half-finished `develop` work alongside it.

The whole sequence, end to end:

```sh
# 1. Branch off the current production tip.
git checkout main
git pull
git checkout -b hotfix/19-panic-on-empty-profile

# 2. Make the fix, with a test, then verify locally
#    (a green local run is a green CI run).
make test && make race && make lint && make vet

# 3. Land it on main. Prefer a PR for the review/CI gate; if it is a true
#    can't-wait emergency, merge directly and let the tag re-verify:
git checkout main
git merge --no-ff hotfix/19-panic-on-empty-profile
git push origin main

# 4. Tag and push - this triggers the release workflow.
git tag v0.1.4
git push origin v0.1.4

# 5. Reconcile develop so the fix survives the next promotion.
git checkout develop
git merge --no-ff main
git push origin develop
```

A hotfix is almost always a **patch** bump (`v0.1.3` → `v0.1.4`).

## Branch naming

Use `<type>/<short-description>`: a type prefix, a slash, then a concise kebab-case description. The prefixes mirror our Conventional-Commits commit types, so branch and commit vocabulary match.

| Prefix      | Use for                                            |
| ----------- | -------------------------------------------------- |
| `feature/`  | New functionality                                  |
| `fix/`      | Bug fixes                                          |
| `hotfix/`   | Urgent production fixes (cut off `main`)           |
| `chore/`    | Tooling, deps, config - no behavior change         |
| `refactor/` | Internal restructuring - no behavior change        |
| `docs/`     | Documentation only                                 |
| `test/`     | Test-only changes                                  |

Rules:

- **kebab-case, lowercase, ASCII** - no spaces, no underscores, no mixed case (macOS's case-insensitive filesystem gets weird with the latter).
- **Reference the issue** when there is one, and add a content hint so the branch is self-describing in a list:

  ```
  fix/15-glaze-down-decode
  feature/12-down-flow-skip
  docs/cover-releases
  ```

  A bare `fix/issue-15` is acceptable but `fix/15-glaze-down-decode` tells you what it does without an issue lookup.
- **Keep it short** - 2–4 words of description.

## Cutting a release

Releases are driven entirely by pushing a semver tag to a commit on `main`. There is no manual upload step.

1. Make sure `main` is at the commit you intend to ship and is pushed to the remote - the Forgejo mirror forwards tags to GitHub, and the tag must point at a commit the remote already has.
2. Tag and push:

   ```sh
   git checkout main
   git pull
   git tag v0.2.0
   git push origin v0.2.0
   ```

   Use [semantic versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`.

3. The [release workflow](.github/workflows/release.yaml) triggers on the `v*` tag and:
   - **re-verifies** the tagged commit (`make vet`, `make test`, `make race`);
   - **cross-compiles** version-stamped binaries via `make release STAGE=production` - linux/darwin × amd64/arm64 - zipped with a `SHA256SUMS` manifest;
   - **signs** build provenance for every zip (sigstore attestation), attaching the `intoto.jsonl` bundle to the release so the provenance survives independently of GitHub's attestation API;
   - **publishes** a GitHub Release titled with the tag, with notes auto-generated from the PRs merged since the previous tag.

   The version stamp comes from `git describe --tags`, which is why the release job checks out with full history (`fetch-depth: 0`).

4. Verify a published artifact if you want offline assurance:

   ```sh
   gh attestation verify glaze-<os>-<arch>.zip --repo wilhelm-murdoch/glazier
   ```

### Tagging notes

- **Tag the actual `develop`→`main` merge commit.** Don't tag a local state that hasn't been pushed, or the release will be cut from something the remote doesn't have.
- **One tag, one release.** If a release fails after the tag is pushed, fix forward with a new patch tag rather than moving or reusing a tag - moved tags break provenance and the generated notes.

## See also

- [CONTRIBUTING.md](CONTRIBUTING.md) - how to build, test, and open PRs.
- [.github/workflows/release.yaml](.github/workflows/release.yaml) - the workflow this document describes.
- [SECURITY.md](SECURITY.md) - vulnerability reporting.
