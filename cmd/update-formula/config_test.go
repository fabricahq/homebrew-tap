package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testConfig = toolConfig{
	Name: "code-rules", Repository: "fabricahq/code-rules", Binary: "code-rules",
	Description: "The package manager for engineering best practices", Homepage: "https://code-rules.fabricahq.com", License: "MIT", LicenseFile: "LICENSE.md",
	Platforms: []string{"darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64"}, SignerWorkflow: ".github/workflows/release.yml", VersionArgs: []string{"--version"}, TestArgs: []string{"--help"},
}

func TestLoadConfig(t *testing.T) {
	c, err := loadConfig("../..", "code-rules")
	if err != nil {
		t.Fatal(err)
	}
	if c.Repository != testConfig.Repository || len(c.matrix()["include"].([]map[string]string)) != 4 {
		t.Fatal("incorrect config", c)
	}
	for _, name := range []string{"../code-rules", "CODE", "code.rules", "bad\nname", ""} {
		if _, err := loadConfig("../..", name); err == nil {
			t.Fatal("accepted unsafe name", name)
		}
	}
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "tools"), 0755)
	data, _ := json.Marshal(testConfig)
	for _, value := range []string{string(data) + " {}", strings.Replace(string(data), `"name":`, `"unknown":1,"name":`, 1), strings.Replace(string(data), `"name":"code-rules"`, `"name":"different"`, 1)} {
		os.WriteFile(filepath.Join(root, "tools", "code-rules.json"), []byte(value), 0600)
		if _, err := loadConfig(root, "code-rules"); err == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
}

func TestConfigRejectsUnsafeInputs(t *testing.T) {
	for _, change := range []func(*toolConfig){
		func(c *toolConfig) { c.Name = "../escape" }, func(c *toolConfig) { c.Repository = "owner/repo/other" },
		func(c *toolConfig) { c.Binary = "$(whoami)" }, func(c *toolConfig) { c.LicenseFile = "../LICENSE" },
		func(c *toolConfig) { c.SignerWorkflow = ".github/workflows/../../bad" }, func(c *toolConfig) { c.Homepage = "http://example.com" },
		func(c *toolConfig) { c.Platforms = []string{"windows_amd64"} }, func(c *toolConfig) { c.Platforms = []string{"linux_arm64", "linux_arm64"} },
		func(c *toolConfig) { c.Platforms = nil }, func(c *toolConfig) { c.VersionArgs = []string{"--version; touch bad"} }, func(c *toolConfig) { c.TestArgs = nil },
		func(c *toolConfig) { c.Description = "bad\nclass Inject" },
	} {
		c := testConfig
		change(&c)
		if err := c.validate(); err == nil {
			t.Fatal("accepted invalid config", c)
		}
	}
}

// A second tool must use its own repository, binary, class, license, platforms, and test arguments.
func TestSecondToolFormula(t *testing.T) {
	c := testConfig
	c.Name = "sample-tool"
	c.Repository = "example/sample"
	c.Binary = "sample"
	c.License = "Apache-2.0"
	c.Homepage = "https://example.com"
	c.Description = `Tool #{raise "injection"}`
	c.VersionArgs = []string{"version"}
	c.TestArgs = []string{"help"}
	c.Platforms = []string{"linux_amd64"}
	r, sums := releaseFixture("1.2.3")
	for i := range r.Assets {
		r.Assets[i].Name = strings.ReplaceAll(r.Assets[i].Name, "code-rules_", "sample_")
		r.Assets[i].URL = c.repositoryURL() + "/releases/download/v1.2.3/" + r.Assets[i].Name
	}
	sums = strings.ReplaceAll(sums, "code-rules_", "sample_")
	formula, err := renderFormula(c, r, sums)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"class SampleTool", `license "Apache-2.0"`, "https://github.com/example/sample/releases/download", `bin.install "sample"`, `shell_output("#{bin}/sample version")`, `system bin/"sample", "help"`, `\#{raise`} {
		if !strings.Contains(formula, want) {
			t.Fatalf("missing %q in %s", want, formula)
		}
	}
	if strings.Contains(formula, "on_macos") || strings.Contains(formula, "code-rules") {
		t.Fatal("leaked Code Rules configuration", formula)
	}
}
