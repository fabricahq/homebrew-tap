// Verify formula output and fail-closed updates against public release metadata fixtures.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func releaseFixture(version string) (release, string) {
	no := false
	r := release{Tag: "v" + version, Draft: &no, Prerelease: &no, PublishedAt: "2026-09-18T00:00:00Z"}
	var sums strings.Builder
	for _, target := range targets {
		name := fmt.Sprintf("code-rules_%s_%s.tar.gz", version, target)
		digest := strings.Repeat("a", 64)
		r.Assets = append(r.Assets, asset{Name: name, Size: 1234, State: "uploaded", Digest: "sha256:" + digest, URL: repository + "/releases/download/" + r.Tag + "/" + name})
		fmt.Fprintf(&sums, "%s  %s\n", digest, name)
	}
	return r, sums.String()
}

// TestFormulaOutput pins the generated formula to the reference fixture.
func TestFormulaOutput(t *testing.T) {
	r, sums := releaseFixture("1.2.3")
	formula, err := renderFormula(r, sums)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/code-rules.rb")
	if err != nil {
		t.Fatal(err)
	}
	if formula != string(expected) {
		t.Fatalf("formula differs from reference:\n%s", formula)
	}
	for i := range r.Assets {
		r.Assets[i].Digest = ""
	}
	if _, err := renderFormula(r, strings.ReplaceAll(sums, "\n", "\r\n")); err != nil {
		t.Fatal("optional API digests and CRLF checksums must work", err)
	}
}

// TestRejectInvalidReleases keeps incomplete, ambiguous, or unpublished metadata out of the formula.
func TestRejectInvalidReleases(t *testing.T) {
	yes := true
	for _, test := range []struct {
		name   string
		change func(*release)
	}{
		{"draft", func(r *release) { r.Draft = &yes }},
		{"prerelease", func(r *release) { r.Prerelease = &yes }},
		{"unpublished", func(r *release) { r.PublishedAt = "" }},
		{"missing draft flag", func(r *release) { r.Draft = nil }},
		{"missing prerelease flag", func(r *release) { r.Prerelease = nil }},
		{"prerelease tag", func(r *release) { r.Tag = "v1.2.3-rc.1" }},
		{"tag injection", func(r *release) { r.Tag = "v1.2.3\"; system(\"bad\")" }},
		{"missing archive", func(r *release) { r.Assets = r.Assets[:3] }},
		{"duplicate asset", func(r *release) { r.Assets = append(r.Assets, r.Assets[0]) }},
		{"digest mismatch", func(r *release) { r.Assets[0].Digest = "sha256:" + strings.Repeat("b", 64) }},
		{"empty asset", func(r *release) { r.Assets[0].Size = 0 }},
		{"not uploaded", func(r *release) { r.Assets[0].State = "new" }},
		{"wrong URL", func(r *release) { r.Assets[0].URL = "https://example.com/archive" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, sums := releaseFixture("1.2.3")
			test.change(&r)
			if _, err := renderFormula(r, sums); err == nil {
				t.Fatal("invalid release accepted")
			}
		})
	}
	r, sums := releaseFixture("1.2.3")
	for _, test := range []struct{ name, sums string }{
		{"empty", ""}, {"missing", strings.Split(sums, "\n")[0]},
		{"duplicate", sums + sums}, {"invalid digest", strings.ReplaceAll(sums, strings.Repeat("a", 64), "invalid")},
		{"blank line", sums + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := renderFormula(r, test.sums); err == nil {
				t.Fatal("invalid checksums accepted")
			}
		})
	}
}

// TestUpdateFormula verifies idempotence, upgrades, immutable version assets, and migration of the generator comment.
func TestUpdateFormula(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Formula", "code-rules.rb")
	r, sums := releaseFixture("1.2.3")
	if changed, err := updateFormula(path, r, sums); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if changed, err := updateFormula(path, r, sums); err != nil || changed {
		t.Fatal(changed, err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := strings.Replace(string(original), formulaHeader, "# Generated from a published Code Rules release; update with scripts/update_code_rules.py.", 1)
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	if changed, err := updateFormula(path, r, sums); err != nil || !changed {
		t.Fatal("generator comment migration", changed, err)
	}
	for _, kind := range []string{"downgrade", "changed same version", "missing asset"} {
		t.Run(kind, func(t *testing.T) {
			candidate, checksums := releaseFixture("1.2.3")
			switch kind {
			case "downgrade":
				candidate, checksums = releaseFixture("1.2.2")
			case "changed same version":
				for i := range candidate.Assets {
					candidate.Assets[i].Digest = "sha256:" + strings.Repeat("b", 64)
				}
				checksums = strings.ReplaceAll(checksums, strings.Repeat("a", 64), strings.Repeat("b", 64))
			case "missing asset":
				candidate.Assets = nil
			}
			if _, err := updateFormula(path, candidate, checksums); err == nil {
				t.Fatal("unsafe update accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(original) {
				t.Fatal("failed update changed existing file", err)
			}
		})
	}
	newer, checksums := releaseFixture("1.3.0")
	if changed, err := updateFormula(path, newer, checksums); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if err := os.WriteFile(path, []byte("unknown formula"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := updateFormula(path, newer, checksums); err == nil {
		t.Fatal("overwrote unrecognized formula")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".code-rules-*.rb"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("staging files remain", leftovers, err)
	}
}

// TestStableVersionOrdering preserves numeric ordering without restricting version numbers to machine integers.
func TestStableVersionOrdering(t *testing.T) {
	for _, tag := range []string{"1.2.3", "v01.2.3", "v1.2.3\n", "v1.2.3+build", "v1.2.3-rc.1"} {
		if _, err := stableVersion(tag); err == nil {
			t.Fatal("invalid stable tag accepted", tag)
		}
	}
	for _, pair := range [][2]string{{"v1.9.0", "v1.10.0"}, {"v9.0.0", "v10.0.0"}, {"v1.0.99999999999999999999999999", "v1.1.0"}} {
		a, err := stableVersion(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		b, err := stableVersion(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if compareVersions(a, b) >= 0 || compareVersions(b, a) <= 0 || compareVersions(a, a) != 0 {
			t.Fatal("incorrect ordering", pair)
		}
	}
}
