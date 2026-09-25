# Maintain the Fabrica Homebrew tap

This tap distributes public Fabrica tools on macOS and Linux. Product repositories own releases; this repository owns formula generation, install tests, and publication. Private repositories and authenticated downloads are outside its scope.

A successful product release dispatches an update immediately. One shared Go updater validates the release and its provenance, then generates a formula from `tools/<name>.json`. A separate job tests installation on every configured platform. Only after all checks pass does the Publisher App commit the formula to `main`, without human approval.

## One-time setup: organization owners and tap maintainers

1. Configure the [Releaser App and organization credentials](#configure-release-triggers). Give access only to trusted product repositories.
2. Configure the [Publisher App and protected environment](#protect-formula-publication). Keep its key exclusively in the tap.
3. Require PRs for changes to the tap's `main`, with a bypass for the Publisher App. Keep deletion and force-push protection in a separate rule without a bypass. Admins can merge a PR without approval but cannot push to `main` directly.
4. Merge the shared updater, install-test, notification, and drift-check workflows. Enable repository issues and permit the notification jobs to create issues. Keep Actions failure notifications enabled for the maintainers responsible for this tap.
5. Follow the complete onboarding checklist below before a product's first Homebrew release.

## Add a public tool: product and tap maintainers

1. **Define the release contract.** Use stable tags in `vMAJOR.MINOR.PATCH` form. Publish `<binary>_<version>_<os>_<arch>.tar.gz` archives and `SHA256SUMS` with lowercase SHA-256 hashes and two spaces before each filename. Each archive must contain the executable and configured license file at its root. Supported targets are `darwin_arm64`, `darwin_amd64`, `linux_arm64`, and `linux_amd64`.
2. **Protect the product release.** Require reviewed changes to `main`, block force pushes and deletion, and enable immutable releases. Restrict the signing and publication environment to the `main` branch. After builds and tests pass, attest the exact `SHA256SUMS` file from the release workflow on `main`, using GitHub-hosted runners. Publish the stable release only after attestation succeeds.
3. **Add one tool definition.** Copy [`tools/code-rules.json`](../tools/code-rules.json) to `tools/<name>.json`. Set the repository, executable, homepage, description, SPDX license, license filename, supported platforms, signing workflow, version arguments, and smoke-test arguments. The filename and `name` must match. Tests invoke the configured executable with literal arguments; configuration cannot contain arbitrary Ruby or shell code.
4. **Request organization credential access.** Ask an organization owner to add the product repository to the selected-repository lists for both `HOMEBREW_APP_PRIVATE_KEY` and `HOMEBREW_APP_CLIENT_ID`. Product maintainers do not need a PEM copy. Confirm that the repository's maintainers and workflows are trusted before granting access.
5. **Connect the release trigger.** In the product repository, add a job that runs after a stable release is published. Use the shared Releaser App to create a short-lived installation token restricted to `fabricahq/homebrew-tap`, with only `permission-actions: write`. With that token, dispatch `update-tool.yml` on `main`, passing the tool name from `tools/<name>.json` as the `tool` input. Every product uses this shared workflow to update its formula.
6. **Review and merge setup before releasing.** Validate the configuration and updater changes with the commands below. Merge the tap configuration and product release integration before publishing the first release, so the first attested release can pass the complete update pipeline.
7. **Verify the first automatic update.** Confirm that style, online audit, installation, and formula tests pass on all configured native platforms, and that the Publisher commits the tested formula. Test installation from the public tap in a disposable environment, check the API-created commit's Verified status, and add the tool to the [README's tool list](../README.md#tools).

The shared updater and workflow handle all tools following this contract. A new tool normally needs only its JSON definition and product-side dispatch job. Add tool-specific code only when the release contract genuinely differs; do not copy the updater.

Example dispatch, using the short-lived Releaser token as `GH_TOKEN`:

```sh
gh workflow run update-tool.yml --repo fabricahq/homebrew-tap --ref main -f tool=example-tool
```

### Configure release triggers

Trusted products share the [Fabrica Homebrew Releaser](https://github.com/apps/fabrica-homebrew-releaser) App. Install it only on the tap, with **Actions: read and write** and **Metadata: read-only** permissions.

Store its credentials once in the [organization's Actions settings](https://github.com/organizations/fabricahq/settings/secrets/actions). An organization owner performs setup or rotation:

1. Open the [Releaser App settings](https://github.com/organizations/fabricahq/settings/apps/fabrica-homebrew-releaser). Copy **Client ID** from **About**. GitHub assigns this value; it is not the numeric App ID or a client secret.
2. Obtain the existing PEM key, or follow [Get or rotate an App private key](#get-or-rotate-an-app-private-key).
3. Under **Secrets and variables > Actions > Secrets**, set `HOMEBREW_APP_PRIVATE_KEY` to the entire PEM file, including its header, footer, and line breaks.
4. Under **Variables**, set `HOMEBREW_APP_CLIENT_ID` to the copied client ID.
5. Choose **Selected repositories** for both values and grant access only to trusted product repositories. Consult these access lists for the current set of authorized repositories.

Workflows use `secrets.HOMEBREW_APP_PRIVATE_KEY` and `vars.HOMEBREW_APP_CLIENT_ID`. Remove older repository and environment copies after verifying organization access, because those copies override organization values.

**Access tradeoff:** organization secrets are restricted by repository, not by branch or environment. Eligible workflows in allowed repositories can access the key without entering a protected environment. A `main`-only dispatch environment restricts the intended job, not access to the organization secret by other jobs.

A leaked Releaser key can dispatch, cancel, rerun, or disable tap workflows and delete run logs or caches. It does not grant Contents write access, the Publisher key, or the Publisher's branch-rule bypass. It cannot directly commit a formula; dispatched updates still pass provenance verification and install tests before publication. The principal risk is disruption of updates and their diagnostics.

[CR-7: OIDC dispatch service](https://linear.app/ohmygoshjosh/issue/CR-7/replace-shared-homebrew-trigger-keys-with-an-oidc-dispatch-service) tracks replacing the shared trigger key with a service that authorizes each product's repository, branch, and workflow. Formula integrity continues to depend on the protected publishing pipeline.

Code Rules starts its update with [Release Planner](https://release-planner.fabricahq.com/customize/downstream/), which reads the Releaser App's credentials from its `downstream` environment as `DOWNSTREAM_APP_CLIENT_ID` and `DOWNSTREAM_APP_PRIVATE_KEY`. Its workflow is [release-planner.yml](https://github.com/fabricahq/code-rules/blob/main/.github/workflows/release-planner.yml).

### Protect formula publication

The separate [Fabrica Homebrew Publisher](https://github.com/apps/fabrica-homebrew-publisher) App owns formula commits. Install it only on this tap with **Contents: read and write** and **Metadata: read-only** permissions. Product repositories must never receive its private key.

1. Open the [Publisher App settings](https://github.com/organizations/fabricahq/settings/apps/fabrica-homebrew-publisher) and copy **Client ID** from **About**.
2. Obtain this App's PEM key or follow the key-generation instructions below. The two Apps have different keys and client IDs.
3. In the tap's **Settings > Environments > formula-publish**, allow only branch `main` under **Deployment branches and tags**. Set no required reviewers or wait timer, so routine updates remain automatic.
4. Add the environment secret `FORMULA_APP_PRIVATE_KEY` containing the entire Publisher PEM. Add the environment variable `FORMULA_APP_CLIENT_ID` containing its client ID.

The shared publishing job declares `environment: formula-publish`. It does not check out the repository or run the updater or downloaded tool. It reads the prepared artifact as data, verifies the existing formula's SHA, and commits through GitHub's Contents API. Install-test jobs have no App secrets, protected environment, or repository-write permissions. No job holding App credentials or write permissions executes a downloaded release binary.

The Publisher credential has repository-wide Contents access. The reviewed publishing job limits writes to the selected formula, after configuration validation and native install tests. Workflow and configuration changes therefore require review.

### Get or rotate an App private key

An organization owner or authorized App manager performs these steps:

1. Open the correct App's settings using the links above.
2. Under **Private keys**, click **Generate a private key** and securely store the downloaded PEM. GitHub does not retain a downloadable private-key copy.
3. Upload the Releaser PEM to the organization secret, or the Publisher PEM to the tap's `formula-publish` environment secret. Never commit the PEM, paste it into a PR, or print it in logs. Actions secrets cannot be read back to recover a lost key.
4. Update the stored secret and verify authentication before deleting the old key in App settings. Check for older repository or environment copies. Revoke a compromised key promptly.

See GitHub's [private-key management instructions](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps) for fingerprint verification and revocation.

## Release verification and install tests

Every tool must use the same provenance policy. The shared updater verifies `SHA256SUMS` with `gh attestation verify`, pinning `--repo`, `--signer-workflow`, `--source-ref refs/heads/main`, and `--deny-self-hosted-runners`. Unsigned manifests, unexpected signers, incomplete archives, invalid checksums, downgrades, and changed same-version formulas stop the update without changing the existing formula.

Changed formulas then run `brew style`, `brew audit --strict --online`, `brew install`, and `brew test` on a fresh GitHub-hosted runner for each configured platform. A runner-architecture check prevents an ARM archive from passing only through Intel emulation. Publication waits for the entire matrix. Test jobs run release code without App secrets or write permissions; they never upload the artifact used for publishing.

The generator retains the approved Code Rules tagline, including its leading “The”, with a `FormulaAudit/Desc` exception only when the generated description exactly matches that tagline. The shared style script checks all other cops, including strict cops. Audit uses `--skip-style` to avoid repeating these style checks; online and other strict audits remain enabled.

## Detect and recover from failed updates

Release dispatch remains the immediate update trigger. **Check formula drift** runs once a day and compares every configured tool's latest public stable release with its formula. It does not publish updates or download executable code. A tool without a first release and without a formula is a clean no-op. Missing/stale formulas and check failures open one issue while the problem remains unresolved.

Update failures also open an issue with a link to the failed run. Notification jobs hold only Issues write permission and execute no updater or release code. Repeated failures reuse the open issue instead of creating more notifications. After a fix, rerun the affected workflow, confirm success, and close the issue manually.

GitHub can delay scheduled runs and disables schedules in public repositories after 60 days without repository activity. Maintainers should check the workflow's enabled state after inactivity and re-enable it if needed. The daily check is a backstop, not a guaranteed external uptime monitor. If issue creation itself fails, the notification job fails visibly in Actions.

Generated formulas also support `brew livecheck`, using GitHub's latest stable release:

```sh
brew livecheck fabricahq/tap/code-rules
```

Per-tool concurrency serializes updates with `cancel-in-progress: false`; the formula SHA check also prevents overwriting a concurrent human change. GitHub may replace an older pending run with a newer one. Each run resolves the latest release, so intermediate releases need not all produce commits.

### Bad releases and manual recovery

Prefer fixing forward: publish an attested patch release, let the automatic checks run, and verify the resulting installation. Immutable releases prevent replacing a published archive or retagging the same version.

If users need immediate relief, maintainers can submit a reviewed PR that restores a known-good formula or disables the broken formula. Explicitly review any version decrease and test installation on every supported platform. Users may need to uninstall and reinstall to move to an older version. Do not relax the updater's downgrade or same-version checks to automate recovery. Pause the affected update workflow while investigating a bad latest release; the drift check should continue reporting the mismatch.

Generated files contain a do-not-edit header. Routine changes belong in the tool definition or generator. An emergency formula edit is temporary; the next accepted release regenerates it. If a template-only change must apply at the same version, update the generator and reviewed formula together in a PR, preserving the attested archive hashes.

## Prepare and validate changes

Use the Go version in `go.mod` and an authenticated GitHub CLI for provenance verification:

```sh
go test -race ./...
go vet ./...
go run ./cmd/update-formula --tool code-rules --matrix
go run ./cmd/update-formula --tool code-rules
go run ./cmd/update-formula --check-all
```

For generator changes, regenerate the checked-in fixture with `UPDATE_GOLDEN=1 go test ./cmd/update-formula -run TestFormulaOutput`, then inspect its diff. CI checks the fixture's Homebrew style and validates all workflows.

Run installation checks in disposable environments. Never execute downloaded tools in jobs holding publishing or notification credentials. Keep updater implementations and tests in this tap; product repositories only publish releases and trigger updates.
