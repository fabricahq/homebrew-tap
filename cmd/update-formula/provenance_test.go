// Exercise the attestation command boundary and reject unsigned releases before changing files.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyChecksums(t *testing.T) {
	directory := t.TempDir()
	log := filepath.Join(directory, "arguments")
	t.Setenv("PROVENANCE_TEST_LOG", log)
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := `#!/bin/sh
set -eu
printf '%s\n' "$@" > "$PROVENANCE_TEST_LOG"
cat "$3" > "$PROVENANCE_TEST_LOG.manifest"
exit "${PROVENANCE_TEST_EXIT:-0}"
`
	if err := os.WriteFile(filepath.Join(directory, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := []byte("exact manifest bytes\n")
	if err := verifyChecksums(t.Context(), testConfig, manifest); err != nil {
		t.Fatal(err)
	}
	arguments, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(arguments)), "\n")
	if len(args) != 10 {
		t.Fatalf("wrong verifier arguments: %q", args)
	}
	expected := []string{"attestation", "verify", args[2], "--repo", "fabricahq/code-rules", "--signer-workflow", "fabricahq/code-rules/.github/workflows/release.yml", "--source-ref", "refs/heads/main", "--deny-self-hosted-runners"}
	if strings.Join(args, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("wrong verifier policy: %q", args)
	}
	copied, err := os.ReadFile(log + ".manifest")
	if err != nil || !bytes.Equal(copied, manifest) {
		t.Fatal("verifier did not receive exact manifest", err)
	}
	if _, err := os.Stat(args[2]); !os.IsNotExist(err) {
		t.Fatal("temporary manifest was not removed", err)
	}
	t.Setenv("PROVENANCE_TEST_EXIT", "1")
	if err := verifyChecksums(t.Context(), testConfig, manifest); err == nil {
		t.Fatal("accepted failed signature verification")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := verifyChecksums(ctx, testConfig, manifest); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestUnverifiedReleasePreservesFormula(t *testing.T) {
	r, sums := releaseFixture("1.2.3")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	fetch := func(_ context.Context, url string) ([]byte, error) {
		if url == testConfig.latestURL() {
			return data, nil
		}
		return []byte(sums), nil
	}
	for _, existing := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "code-rules.rb")
		previous := []byte("existing trusted formula\n")
		if existing {
			if err := os.WriteFile(path, previous, 0644); err != nil {
				t.Fatal(err)
			}
		}
		refused := errors.New("untrusted signer")
		verified := false
		verify := func(_ context.Context, got []byte) error {
			verified = true
			if string(got) != sums {
				t.Fatal("verified different manifest")
			}
			return refused
		}
		var out bytes.Buffer
		if err := run(t.Context(), testConfig, fetch, verify, path, &out); !errors.Is(err, refused) {
			t.Fatal("ignored verification failure", err)
		}
		if !verified {
			t.Fatal("skipped verifier")
		}
		after, err := os.ReadFile(path)
		if existing {
			if err != nil || !bytes.Equal(after, previous) {
				t.Fatal("changed formula after failed verification", err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal("created formula after failed verification", err)
		}
	}
}
