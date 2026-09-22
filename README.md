# Fabrica Homebrew tap

Install Code Rules on macOS or Linux with Homebrew:

```sh
brew install fabricahq/tap/code-rules
brew upgrade code-rules
brew uninstall code-rules
```

The formula installs the official executable for your operating system and processor, verifies the release archive's SHA-256, and retains its MIT license. Go, Node.js, and Bun are not required. Git is needed for `code-rules sync`.

After publishing a stable release, the Code Rules release workflow triggers **Update Code Rules** through the Fabrica Homebrew Releaser GitHub App. The tap validates the release and commits its formula directly to `main`; routine formula updates do not require a pull request or human approval. The updater can also run manually to retry an update. It does not poll for releases. Until the first stable release is published and the workflow succeeds, the formula is unavailable. Prereleases and drafts are never selected; failed updates leave the existing formula intact.

Maintainers need the Go version declared in `go.mod` to prepare an update locally:

```sh
go run ./cmd/update-code-rules
```

Review `Formula/code-rules.rb`, then commit it. The updater refuses downgrades and changed checksums at the same version. Formula downloads come directly from `fabricahq/code-rules` release assets.

This repository owns Homebrew formula generation, validation, and publication for Fabrica tools. Each product repository owns its release binaries and checksums and triggers the corresponding tap workflow after publishing a stable release. Keep updater implementations and tests here, rather than copying them into product repositories.

This repository can hold other Fabrica tools under `Formula/`. The Code Rules updater changes only `Formula/code-rules.rb`.

## Validate updater changes

```sh
go test -race ./...
go vet ./...
ruby -c cmd/update-code-rules/testdata/code-rules.rb
```

Tests cover formula output, metadata validation, downgrade and same-version protection, HTTPS redirects, download limits, and failures that must leave the formula unchanged. The updater fetches only metadata and checksums; it never executes downloaded release code. Actual installation tests belong in a disposable environment.

Updater code and workflow changes go through pull requests. After those changes are approved, stable product releases trigger automatic formula publication.

## Reuse the release app for another tool

Fabrica Homebrew Releaser is a private GitHub App owned by `fabricahq`. Install it only on the tap repositories it needs to trigger, with **Actions: read and write** and the required **Metadata: read-only** permission. It needs no repository contents permission: each tap's workflow uses its own `GITHUB_TOKEN` to publish formula updates.

For another Fabrica tool, add a dedicated updater workflow to its tap with `workflow_dispatch`, then trigger it after that tool publishes a stable release. Configure the product repository with `HOMEBREW_APP_CLIENT_ID` as an Actions variable and `HOMEBREW_APP_PRIVATE_KEY` as an Actions secret. Generate a short-lived installation token restricted to the intended tap and `permission-actions: write`, then dispatch that updater on `main`. For tools sharing this tap, the existing app installation can be reused.

Access to the app's private key allows creating tokens for any repository in its installation. Share that credential only with trusted release repositories and jobs; use separate apps when products need separate trust boundaries. Never commit the key. The Code Rules release workflow provides an example of a separate dispatch job with no source checkout or build tools.
