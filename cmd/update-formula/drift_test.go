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

func TestCheckDrift(t *testing.T) {
	for _, scenario := range []string{"current", "missing formula", "stale", "no release", "removed release", "network failure", "unrecognized formula"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			path := testConfig.formulaPath(root)
			r, sums := releaseFixture("1.2.3")
			data, _ := json.Marshal(r)
			if scenario != "no release" && scenario != "missing formula" {
				if _, err := updateFormula(testConfig, path, r, sums); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "unrecognized formula" {
				os.WriteFile(path, []byte("broken formula"), 0600)
			}
			before, _ := os.ReadFile(path)
			fetch := func(_ context.Context, url string) ([]byte, error) {
				if url != testConfig.latestURL() {
					t.Fatal("unexpected download", url)
				}
				switch scenario {
				case "no release", "removed release":
					return nil, errNotFound
				case "network failure":
					return nil, errors.New("offline")
				case "stale":
					return bytes.ReplaceAll(data, []byte("v1.2.3"), []byte("v1.2.4")), nil
				}
				return data, nil
			}
			err := checkDrift(t.Context(), testConfig, root, fetch)
			if (err == nil) != (scenario == "current" || scenario == "no release") {
				t.Fatal("incorrect drift result", err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("drift check changed formula")
			}
		})
	}
}

func TestCheckAllReportsEveryTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "tools"), 0755)
	for _, name := range []string{"first", "second"} {
		c := testConfig
		c.Name = name
		data, _ := json.Marshal(c)
		os.WriteFile(filepath.Join(root, "tools", name+".json"), data, 0600)
	}
	r, _ := releaseFixture("1.2.3")
	data, _ := json.Marshal(r)
	var out bytes.Buffer
	calls := 0
	err := checkAll(t.Context(), root, func(context.Context, string) ([]byte, error) { calls++; return data, nil }, &out)
	if err == nil || calls != 2 || !strings.Contains(out.String(), "first:") || !strings.Contains(out.String(), "second:") {
		t.Fatal("lost drift findings", err, calls, out.String())
	}
}
