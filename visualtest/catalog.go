// Package visualtest defines reproducible browser UI scenarios and compares their
// captured screenshots. Browser interaction is supplied by visualtest/rod;
// application fixtures and assertions belong to the consuming project.
package visualtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Definition describes product behavior, not the browser actions that establish it.
// Its checkpoints are captured explicitly by the application's scenario function.
type Definition struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Fixture     string   `json:"fixture"`
	Checkpoints []string `json:"checkpoints"`
	Viewports   []string `json:"viewports"`
	Themes      []string `json:"themes"`
}

var scenarioID = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var segment = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

func unique(items []string, label string, valid func(string) bool) error {
	if len(items) == 0 {
		return fmt.Errorf("at least one %s is required", label)
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !valid(item) {
			return fmt.Errorf("invalid %s %q", label, item)
		}
		if seen[item] {
			return fmt.Errorf("duplicate %s %q", label, item)
		}
		seen[item] = true
	}
	return nil
}

// Validate rejects ambiguous scenario names and duplicate or invalid variants.
func (d Definition) Validate() error {
	if !scenarioID.MatchString(d.ID) {
		return fmt.Errorf("invalid scenario ID %q: want area.capability.case", d.ID)
	}
	if strings.TrimSpace(d.Description) == "" {
		return fmt.Errorf("scenario %s: description required", d.ID)
	}
	if !segment.MatchString(d.Fixture) {
		return fmt.Errorf("scenario %s: invalid fixture %q", d.ID, d.Fixture)
	}
	if err := unique(d.Checkpoints, "checkpoint", segment.MatchString); err != nil {
		return fmt.Errorf("%s: %w", d.ID, err)
	}
	if err := unique(d.Viewports, "viewport", segment.MatchString); err != nil {
		return fmt.Errorf("%s: %w", d.ID, err)
	}
	if err := unique(d.Themes, "theme", func(theme string) bool { return theme == "light" || theme == "dark" }); err != nil {
		return fmt.Errorf("%s: %w", d.ID, err)
	}
	return nil
}

// Fingerprint hashes normalized JSON so formatting-only edits do not change a contract.
func (d Definition) Fingerprint() string {
	data, _ := json.Marshal(d)
	return Hash(data)
}

// Hash returns a lowercase SHA-256 digest suitable for stable contract fingerprints.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Load reads one <area>.<capability>.<case>.json file per scenario. A missing
// directory is treated as an empty catalog to support comparisons with older main.
func Load(dir string) ([]Definition, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Definition{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read scenarios: %w", err)
	}
	defs := make([]Definition, 0, len(entries))
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		var def Definition
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&def)
		if err == nil {
			var extra any
			if next := decoder.Decode(&extra); next == nil {
				err = errors.New("multiple JSON values")
			} else if !errors.Is(next, io.EOF) {
				err = next
			}
		}
		closeErr := file.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if err := def.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if entry.Name() != def.ID+".json" {
			return nil, fmt.Errorf("%s: file must be named %s.json", path, def.ID)
		}
		if seen[def.ID] {
			return nil, fmt.Errorf("duplicate scenario ID %s", def.ID)
		}
		seen[def.ID] = true
		defs = append(defs, def)
	}
	slices.SortFunc(defs, func(a, b Definition) int { return strings.Compare(a.ID, b.ID) })
	return defs, nil
}
