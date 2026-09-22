// Load reviewed tool definitions; constrain paths, platforms, and generated Ruby inputs.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type toolConfig struct {
	Name           string   `json:"name"`
	Repository     string   `json:"repository"`
	Binary         string   `json:"binary"`
	Description    string   `json:"description"`
	Homepage       string   `json:"homepage"`
	License        string   `json:"license"`
	LicenseFile    string   `json:"license_file"`
	Platforms      []string `json:"platforms"`
	SignerWorkflow string   `json:"signer_workflow"`
	VersionArgs    []string `json:"version_args"`
	TestArgs       []string `json:"test_args"`
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
var filePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var workflowPattern = regexp.MustCompile(`^\.github/workflows/[A-Za-z0-9][A-Za-z0-9_-]*\.ya?ml$`)
var argPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.=-]*$`)
var platformRunners = map[string]string{
	"darwin_arm64": "macos-15", "darwin_amd64": "macos-15-intel",
	"linux_arm64": "ubuntu-24.04-arm", "linux_amd64": "ubuntu-24.04",
}

func loadConfig(root, name string) (toolConfig, error) {
	var c toolConfig
	if !namePattern.MatchString(name) {
		return c, fmt.Errorf("invalid tool name %q", name)
	}
	f, err := os.Open(filepath.Join(root, "tools", name+".json"))
	if err != nil {
		return c, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 64*1024))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return c, fmt.Errorf("unexpected trailing configuration data")
	}
	if c.Name != name {
		return c, fmt.Errorf("tool name must match configuration filename")
	}
	return c, c.validate()
}

func (c toolConfig) validate() error {
	if !namePattern.MatchString(c.Name) || !repoPattern.MatchString(c.Repository) || !filePattern.MatchString(c.Binary) || !filePattern.MatchString(c.LicenseFile) || !workflowPattern.MatchString(c.SignerWorkflow) {
		return fmt.Errorf("invalid tool identity, file, or signer workflow")
	}
	u, err := url.Parse(c.Homepage)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("homepage must be a public HTTPS URL")
	}
	for _, s := range []string{c.Description, c.Homepage, c.License} {
		if s == "" || strings.ContainsAny(s, "\r\n\x00") {
			return fmt.Errorf("metadata must be nonempty single-line text")
		}
	}
	seen := map[string]bool{}
	if len(c.Platforms) == 0 {
		return fmt.Errorf("at least one supported platform is required")
	}
	for _, p := range c.Platforms {
		if platformRunners[p] == "" || seen[p] {
			return fmt.Errorf("unsupported or duplicate platform %q", p)
		}
		seen[p] = true
	}
	for _, args := range [][]string{c.VersionArgs, c.TestArgs} {
		if len(args) == 0 {
			return fmt.Errorf("version_args and test_args are required")
		}
		for _, arg := range args {
			if !argPattern.MatchString(arg) {
				return fmt.Errorf("test arguments must be literal words or flags")
			}
		}
	}
	return nil
}

func (c toolConfig) repositoryURL() string { return "https://github.com/" + c.Repository }
func (c toolConfig) latestURL() string {
	return "https://api.github.com/repos/" + c.Repository + "/releases/latest"
}
func (c toolConfig) formulaPath(root string) string {
	return filepath.Join(root, "Formula", c.Name+".rb")
}
func (c toolConfig) className() string {
	var s strings.Builder
	for _, part := range strings.Split(c.Name, "-") {
		s.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return s.String()
}
func (c toolConfig) matrix() map[string]any {
	rows := []map[string]string{}
	for _, p := range c.Platforms {
		rows = append(rows, map[string]string{"platform": p, "runner": platformRunners[p]})
	}
	return map[string]any{"include": rows}
}
