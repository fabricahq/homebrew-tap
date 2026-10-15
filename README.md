# Fabrica Homebrew tap

A collection of [Homebrew](https://brew.sh/) formulas for installing Fabrica command-line tools on macOS and Linux. Each formula tells Homebrew how to download and install a tool.

## Usage

Install a tool directly from this tap:

```sh
brew install fabricahq/tap/syllabus
```

Or add the tap first, then install tools by name:

```sh
brew tap fabricahq/tap
brew install syllabus
```

To upgrade or remove a tool:

```sh
brew upgrade syllabus
brew uninstall syllabus
```

## Tools

| Tool | Description |
| --- | --- |
| [Syllabus](https://github.com/fabricahq/syllabus) | The package manager for your agent skills and rules |

## For maintainers

Each tool's release workflow triggers its updater in this repository. Formula updates run automatically after provenance verification and native install tests. A daily drift check opens an issue if an update is missed.

For updater commands, tests, and instructions for adding tools, see the [maintainer guide](maintainers/README.md).

## Licenses

Each tool has its own license. See the tool's repository for its license terms.
