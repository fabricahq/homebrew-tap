// Require the release workflow's signed checksum manifest before preparing a formula.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// verifyChecksums binds every archive hash to the protected release workflow on main.
// GitHub CLI verifies the Sigstore signature and workflow identity; no release code runs.
func verifyChecksums(ctx context.Context, checksums []byte) error {
	directory, err := os.MkdirTemp("", "code-rules-provenance-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "SHA256SUMS")
	if err := os.WriteFile(path, checksums, 0600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "gh", "attestation", "verify", path,
		"--repo", "fabricahq/code-rules",
		"--signer-workflow", "fabricahq/code-rules/.github/workflows/release.yml",
		"--source-ref", "refs/heads/main",
		"--deny-self-hosted-runners")
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return fmt.Errorf("checksum manifest must have a valid attestation from the Code Rules release workflow on main (GitHub CLI and GitHub authentication are required): %w", err)
	}
	return nil
}
