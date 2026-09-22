// Exercise public metadata fetching and command outcomes without depending on GitHub availability.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadURL covers bounded reads, HTTPS enforcement, status handling, redirects, and cancellation.
func TestReadURL(t *testing.T) {
	insecureRequests := 0
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { insecureRequests++; w.WriteHeader(http.StatusOK) }))
	defer insecure.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.Write([]byte("metadata"))
		case "/limit":
			w.Write(bytes.Repeat([]byte("x"), metadataLimit))
		case "/large":
			w.Write(bytes.Repeat([]byte("x"), metadataLimit+1))
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/downgrade":
			http.Redirect(w, r, insecure.URL, http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		}
	}))
	defer server.Close()
	client := server.Client()
	for _, path := range []string{"/ok", "/limit", "/redirect"} {
		t.Run(path, func(t *testing.T) {
			if _, err := readURL(t.Context(), client, server.URL+path); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, path := range []string{"/large", "/error", "/downgrade", "/loop"} {
		t.Run(path, func(t *testing.T) {
			if _, err := readURL(t.Context(), client, server.URL+path); err == nil {
				t.Fatal("unsafe or failed fetch accepted")
			}
		})
	}
	if _, err := readURL(t.Context(), client, server.URL+"/missing"); !errors.Is(err, errNotFound) {
		t.Fatal(err)
	}
	if _, err := readURL(t.Context(), client, insecure.URL); err == nil {
		t.Fatal("accepted initial HTTP URL")
	}
	if insecureRequests != 0 {
		t.Fatal("made an insecure HTTP request")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := readURL(ctx, client, server.URL+"/ok"); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation", err)
	}
}

// TestRun verifies missing releases are no-ops while missing checksums and malformed metadata fail closed.
func TestRun(t *testing.T) {
	for _, scenario := range []string{"publish", "no release", "missing checksums", "malformed JSON", "missing flags", "network failure"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "Formula", "code-rules.rb")
			r, sums := releaseFixture("1.2.3")
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			fetch := func(_ context.Context, url string) ([]byte, error) {
				calls++
				if url == testConfig.latestURL() {
					switch scenario {
					case "no release":
						return nil, errNotFound
					case "malformed JSON":
						return []byte("{"), nil
					case "missing flags":
						return []byte(`{"tag_name":"v1.2.3","published_at":"2026-09-18"}`), nil
					case "network failure":
						return nil, errors.New("unreachable")
					}
					return data, nil
				}
				if url != testConfig.repositoryURL()+"/releases/download/v1.2.3/SHA256SUMS" {
					t.Fatalf("unexpected URL: %s", url)
				}
				if scenario == "missing checksums" {
					return nil, errNotFound
				}
				return []byte(sums), nil
			}
			var out bytes.Buffer
			err = run(t.Context(), testConfig, fetch, func(context.Context, []byte) error { return nil }, path, &out)
			switch scenario {
			case "publish":
				if err != nil || !strings.Contains(out.String(), "Prepared code-rules v1.2.3") {
					t.Fatal(err, out.String())
				}
				out.Reset()
				if err := run(t.Context(), testConfig, fetch, func(context.Context, []byte) error { return nil }, path, &out); err != nil || !strings.Contains(out.String(), "Already current:") {
					t.Fatal(err, out.String())
				}
			case "no release":
				if err != nil || calls != 1 || !strings.Contains(out.String(), "No published stable release") {
					t.Fatal(err, calls, out.String())
				}
			default:
				if err == nil {
					t.Fatal("failure was ignored")
				}
			}
			if scenario != "publish" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("failed fetch changed formula", err)
				}
			}
		})
	}
}
