# Fabrica Homebrew tap

A collection of [Homebrew](https://brew.sh/) formulas for installing Fabrica command-line tools on macOS and Linux. Each formula tells Homebrew how to download and install a tool.

## Usage

Install a tool directly from this tap:

```sh
brew install fabricahq/tap/code-rules
```

Or add the tap first, then install tools by name:

```sh
brew tap fabricahq/tap
brew install code-rules
```

To upgrade or remove a tool:

```sh
brew upgrade code-rules
brew uninstall code-rules
```

## Tools

| Tool | Description |
| --- | --- |
| [Code Rules](https://github.com/fabricahq/code-rules) | The package manager for engineering best practices |

## For maintainers

Each tool's release workflow triggers its updater in this repository. Formula updates run automatically after provenance verification and native install tests. A daily drift check opens an issue if an update is missed.

For updater commands, tests, and instructions for adding tools, see the [maintainer guide](maintainers/README.md).

## Licenses

Each tool has its own license. See the tool's repository for its license terms.
