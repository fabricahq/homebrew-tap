# Maintain the Fabrica Homebrew tap

This repository contains the formulas, updaters, and tests for distributing Fabrica tools through Homebrew. Each tool's source repository owns its binaries, checksums, and releases.

## How updates work

1. A tool's release workflow publishes a stable release with binaries and checksums.
2. The release workflow triggers that tool's updater workflow in this tap.
3. The updater validates the release and generates the formula.
4. The publishing job commits the changed formula to `main` automatically, without human approval.

Each updater changes only its tool's formula under `Formula/`. Updaters skip drafts and prereleases, reject downgrades and changed checksums at the same version, and preserve the existing formula when validation fails.

Updates run in response to releases, without polling. To retry a failed update, run the tool's updater workflow manually.

## Add a tool

1. In the tool's repository, publish release archives and checksums for the supported operating systems and processors.
2. In this tap, add a Go updater and tests under `cmd/update-<tool>/`. The updater should generate `Formula/<tool>.rb` from verified release metadata.
3. Add a workflow under `.github/workflows/` with a `workflow_dispatch` trigger. Validate and prepare the formula before passing it to the publishing job.
4. Configure the tool's release workflow to trigger its tap workflow after a successful stable release.
5. Test installation in a disposable environment and add the tool to the [README's tool list](../README.md#tools).

Keep updater implementations and tests in this tap. Product repositories should publish releases and trigger updates, without keeping copies of the tap's code.

## Configure release triggers

Trusted Fabrica product repositories share the [**Fabrica Homebrew Releaser**](https://github.com/apps/fabrica-homebrew-releaser) GitHub App. Install this App only on the tap, with **Actions: read and write** and **Metadata: read-only** permissions. It has no permission to edit repository contents.

In each product repository, store `HOMEBREW_APP_PRIVATE_KEY` in a release environment restricted to `main`. Set `HOMEBREW_APP_CLIENT_ID` to the shared App's client ID. The release job generates a short-lived token restricted to this tap, then dispatches the product's updater workflow on `main`.

The shared credential can trigger or disrupt any workflow in the tap. Share it only among trusted Fabrica repositories. Use a separate trigger App if a product needs a separate trust boundary.

For an example of the dispatch job, see the [Code Rules release workflow](https://github.com/fabricahq/code-rules/blob/main/.github/workflows/release.yml).

## Protect formula publication

A separate **Fabrica Homebrew Publisher** App owns formula commits. Install it only on this tap with **Contents: read and write** and **Metadata: read-only** permissions. Do not give product repositories its private key.

Store `FORMULA_APP_PRIVATE_KEY` and `FORMULA_APP_CLIENT_ID` in the tap's `formula-publish` environment. Restrict that environment to the `main` branch, with no required reviewers or wait timer.

Require pull requests for changes to `main`, with a bypass for the publishing App. Keep deletion and force-push protection in a separate rule without a bypass. Repository administrators may bypass review requirements through a pull request, but cannot push directly. The publishing credential has repository-wide Contents access; the trusted publishing job limits writes to the intended formula.

The preparation job validates releases without publishing credentials. The publishing job consumes only the prepared formula and checks that the existing formula has not changed. It does not check out or execute repository code.

### Verify release provenance

Each product's release workflow must attest its checksum manifest after its builds and tests pass. Keep signing and release publication in an environment restricted to `main`, and enable immutable releases in the product repository.

Before preparing a formula, the updater verifies the manifest's signature, expected repository, release workflow, and `main` source ref. This ties the archive checksums to the approved release workflow. An unsigned manifest or a different signer must stop publication.

The Code Rules updater uses `gh attestation verify` for this check. Local runs need GitHub CLI and authentication in addition to Go. Signing verifies the source of the manifest; it cannot protect against malicious changes approved into the release workflow itself.

### Activate the setup

Configure both Apps, their environment credentials, and the branch rules before releasing a tool. Merge the product's attestation support and the tap updater before publishing the first release. Adding the environment to a workflow does not create its branch restrictions automatically.

When moving an existing trigger key into an environment, remove the repository-level copy after the release workflow uses that environment. This prevents other branches from using the key.

## Prepare and validate changes

Use the Go version declared in `go.mod`. Run the updater for the tool you are changing. For example, to prepare the Code Rules formula:

```sh
go run ./cmd/update-code-rules
```

Inspect the generated formula, then run the shared checks:

```sh
go test -race ./...
go vet ./...
```

Also check the generated formula's Ruby syntax. For example:

```sh
ruby -c Formula/code-rules.rb
```

Test release validation, formula output, upgrades, and failures that must leave the existing formula unchanged. Updaters must not execute downloaded release code; run installation tests in a disposable environment.

Submit updater and workflow changes through pull requests. Once those changes are approved, routine formula updates run automatically.
