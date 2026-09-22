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

Use a dedicated GitHub App for each product's release trigger. Install it only on the tap it needs to trigger, with **Actions: read and write** and **Metadata: read-only** permissions.

Store the App's private key in the product's protected release environment. Generate a short-lived installation token restricted to the tap, then dispatch the product's updater workflow on `main`. Trigger credentials do not need Contents write permission.

Keep credentials for publishing formulas in the tap. Do not share a trigger App's private key across product repositories: possession of that key grants access to every installation of that App.

For an example of the dispatch job, see the [Code Rules release workflow](https://github.com/fabricahq/code-rules/blob/main/.github/workflows/release.yml).

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
