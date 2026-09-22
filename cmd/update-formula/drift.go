// Detect missed dispatches without changing formulas or downloading release executables.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func checkAll(ctx context.Context, root string, fetch func(context.Context, string) ([]byte, error), out io.Writer) error {
	paths, err := filepath.Glob(filepath.Join(root, "tools", "*.json"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no tool definitions found")
	}
	var failures []error
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		config, err := loadConfig(root, name)
		if err == nil {
			err = checkDrift(ctx, config, root, fetch)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", name, err))
			fmt.Fprintf(out, "%s: %v\n", name, err)
		} else {
			fmt.Fprintf(out, "%s: current or awaiting its first release\n", name)
		}
	}
	return errors.Join(failures...)
}

func checkDrift(ctx context.Context, config toolConfig, root string, fetch func(context.Context, string) ([]byte, error)) error {
	current, readErr := os.ReadFile(config.formulaPath(root))
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	data, err := fetch(ctx, config.latestURL())
	if errors.Is(err, errNotFound) && os.IsNotExist(readErr) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot check latest public release: %w", err)
	}
	var r release
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	if err := validateRelease(r); err != nil {
		return err
	}
	match := formulaVersionPattern.FindSubmatch(current)
	if match == nil {
		return fmt.Errorf("published %s has no recognized formula; run its update workflow", r.Tag)
	}
	if string(match[1]) != strings.TrimPrefix(r.Tag, "v") {
		return fmt.Errorf("formula %s differs from published %s; run its update workflow", match[1], r.Tag)
	}
	return nil
}
