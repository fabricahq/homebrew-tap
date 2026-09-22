// Validate published release metadata and render the Code Rules formula without downloading executable code.

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const repository = "https://github.com/fabricahq/code-rules"
const formulaHeader = "# Generated from a published Code Rules release; update with go run ./cmd/update-code-rules."

var targets = []string{"darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64"}
var stablePattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var checksumPattern = regexp.MustCompile(`^([0-9a-f]{64})  (\S+)$`)
var formulaVersionPattern = regexp.MustCompile(`(?m)^  version "([^"]+)"$`)

type release struct {
	Tag         string  `json:"tag_name"`
	Draft       *bool   `json:"draft"`
	Prerelease  *bool   `json:"prerelease"`
	PublishedAt string  `json:"published_at"`
	Assets      []asset `json:"assets"`
}

type asset struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Size   int64  `json:"size"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

// stableVersion returns canonical decimal components, retaining arbitrarily large SemVer numbers.
func stableVersion(tag string) ([3]string, error) {
	match := stablePattern.FindStringSubmatch(tag)
	if match == nil {
		return [3]string{}, fmt.Errorf("expected a stable vMAJOR.MINOR.PATCH release for Homebrew")
	}
	return [3]string{match[1], match[2], match[3]}, nil
}

func compareVersions(a, b [3]string) int {
	for i := range a {
		if len(a[i]) < len(b[i]) {
			return -1
		}
		if len(a[i]) > len(b[i]) {
			return 1
		}
		if cmp := strings.Compare(a[i], b[i]); cmp != 0 {
			return cmp
		}
	}
	return 0
}

func validateRelease(r release) error {
	if _, err := stableVersion(r.Tag); err != nil {
		return err
	}
	if r.Draft == nil || r.Prerelease == nil || *r.Draft || *r.Prerelease || r.PublishedAt == "" {
		return fmt.Errorf("release must be published and stable")
	}
	return nil
}

// renderFormula accepts only complete uploaded archives with matching checksums and canonical release URLs.
func renderFormula(r release, checksums string) (string, error) {
	if err := validateRelease(r); err != nil {
		return "", err
	}
	sums := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(checksums))
	for scanner.Scan() {
		match := checksumPattern.FindStringSubmatch(scanner.Text())
		if match == nil {
			return "", fmt.Errorf("invalid release checksum")
		}
		if _, exists := sums[match[2]]; exists {
			return "", fmt.Errorf("duplicate release checksum")
		}
		sums[match[2]] = match[1]
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read release checksums: %w", err)
	}
	assets := make(map[string]asset)
	for _, a := range r.Assets {
		if _, exists := assets[a.Name]; exists {
			return "", fmt.Errorf("duplicate release asset")
		}
		assets[a.Name] = a
	}
	version := r.Tag[1:]
	var out strings.Builder
	fmt.Fprintf(&out, "%s\nclass CodeRules < Formula\n  desc \"The package manager for engineering best practices\"\n  homepage \"https://code-rules.fabricahq.com\"\n  version %q\n  license \"MIT\"\n\n", formulaHeader, version)
	for index, target := range targets {
		if index%2 == 0 {
			osName := "macos"
			if index == 2 {
				osName = "linux"
			}
			fmt.Fprintf(&out, "  on_%s do\n", osName)
		}
		filename := fmt.Sprintf("code-rules_%s_%s.tar.gz", version, target)
		a := assets[filename]
		digest := sums[filename]
		url := repository + "/releases/download/" + r.Tag + "/" + filename
		if digest == "" || a.State != "uploaded" || a.Size <= 0 || a.URL != url {
			return "", fmt.Errorf("missing or invalid published archive: %s", filename)
		}
		if a.Digest != "" && a.Digest != "sha256:"+digest {
			return "", fmt.Errorf("GitHub asset digest disagrees with SHA256SUMS: %s", filename)
		}
		arch := "intel"
		if strings.HasSuffix(target, "arm64") {
			arch = "arm"
		}
		fmt.Fprintf(&out, "    on_%s do\n      url %q\n      sha256 %q\n    end\n", arch, url, digest)
		if index%2 == 1 {
			out.WriteString("  end\n\n")
		}
	}
	out.WriteString("  def install\n    bin.install \"code-rules\"\n    prefix.install \"LICENSE.md\"\n  end\n\n  test do\n    assert_match version.to_s, shell_output(\"#{bin}/code-rules --version\")\n    system bin/\"code-rules\", \"--help\"\n  end\nend\n")
	return out.String(), nil
}

// updateFormula preserves existing content on validation failure and rejects changed same-version assets or downgrades.
func updateFormula(path string, r release, checksums string) (bool, error) {
	formula, err := renderFormula(r, checksums)
	if err != nil {
		return false, err
	}
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err == nil {
		if string(current) == formula {
			return false, nil
		}
		match := formulaVersionPattern.FindStringSubmatch(string(current))
		if match == nil {
			return false, fmt.Errorf("cannot identify the current formula version")
		}
		previous, err := stableVersion("v" + match[1])
		if err != nil {
			return false, err
		}
		next, err := stableVersion(r.Tag)
		if err != nil {
			return false, err
		}
		if compareVersions(next, previous) <= 0 {
			return false, fmt.Errorf("refusing to replace the same or a newer formula version")
		}
	}
	if err := writeFormula(path, []byte(formula)); err != nil {
		return false, err
	}
	return true, nil
}

// writeFormula stages the complete file beside its destination before replacing it atomically.
func writeFormula(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".code-rules-*.rb")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
