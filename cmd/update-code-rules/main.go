// Fetch bounded public release metadata and prepare a formula; publication belongs to the workflow.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const latestRelease = "https://api.github.com/repos/fabricahq/code-rules/releases/latest"
const metadataLimit = 1024 * 1024

var errNotFound = errors.New("release metadata not found")

// readURL enforces HTTPS on the initial request and every redirect, a timeout, and a 1 MiB body limit.
func readURL(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if request.URL.Scheme != "https" {
		return nil, fmt.Errorf("release metadata requires HTTPS")
	}
	bounded := *client
	bounded.Timeout = 30 * time.Second
	bounded.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("release metadata redirected away from HTTPS")
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many release metadata redirects")
		}
		return nil
	}
	response, err := bounded.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release metadata HTTP status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, metadataLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > metadataLimit {
		return nil, fmt.Errorf("release metadata exceeds 1 MiB")
	}
	return data, nil
}

// run leaves the formula untouched when no release exists; other fetch or validation failures are errors.
func run(ctx context.Context, fetch func(context.Context, string) ([]byte, error), path string, out io.Writer) error {
	data, err := fetch(ctx, latestRelease)
	if errors.Is(err, errNotFound) {
		_, err = fmt.Fprintln(out, "No published stable release yet; leaving the tap unchanged.")
		return err
	}
	if err != nil {
		return err
	}
	var r release
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("decode release metadata: %w", err)
	}
	if err := validateRelease(r); err != nil {
		return err
	}
	sums, err := fetch(ctx, repository+"/releases/download/"+r.Tag+"/SHA256SUMS")
	if err != nil {
		return err
	}
	changed, err := updateFormula(path, r, string(sums))
	if err != nil {
		return err
	}
	message := "Already current:"
	if changed {
		message = "Prepared"
	}
	_, err = fmt.Fprintf(out, "%s Code Rules %s\n", message, r.Tag)
	return err
}

func main() {
	fetch := func(ctx context.Context, url string) ([]byte, error) { return readURL(ctx, http.DefaultClient, url) }
	if err := run(context.Background(), fetch, "Formula/code-rules.rb", os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Homebrew update failed: %v\n", err)
		os.Exit(1)
	}
}
