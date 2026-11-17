package visualtest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const Format = 1

// Size is a Chromium viewport in CSS pixels.
type Size struct{ Width, Height int }

// Environment records parameters which must match before image pixels are compared.
type Environment struct {
	Browser           string  `json:"browser"`
	Platform          string  `json:"platform"`
	Locale            string  `json:"locale"`
	Timezone          string  `json:"timezone"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor"`
	ReducedMotion     string  `json:"reducedMotion"`
}

// Image identifies one captured checkpoint and its exact output file.
type Image struct {
	ID         string `json:"id"`
	Checkpoint string `json:"checkpoint"`
	Viewport   string `json:"viewport"`
	Theme      string `json:"theme"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	File       string `json:"file"`
}

// Manifest describes reproducible captures and per-scenario fixture contracts.
// Contracts includes all defined scenarios, even when a changed contract was
// intentionally not rendered against the baseline checkout.
type Manifest struct {
	Format      int               `json:"format"`
	Revision    string            `json:"revision"`
	Environment Environment       `json:"environment"`
	Contracts   map[string]string `json:"contracts"`
	Images      []Image           `json:"images"`
}

// Filename gives screenshots stable names independent of Git revisions.
func Filename(id, checkpoint, viewport, theme string) (string, error) {
	if !scenarioID.MatchString(id) || !segment.MatchString(checkpoint) || !segment.MatchString(viewport) || (theme != "light" && theme != "dark") {
		return "", errors.New("invalid screenshot identity")
	}
	return id + "--" + checkpoint + "--" + viewport + "--" + theme + ".png", nil
}

// Validate checks a capture manifest before writing or comparing it.
func (m Manifest) Validate() error {
	if m.Format != Format {
		return fmt.Errorf("unsupported capture manifest format %d", m.Format)
	}
	if m.Revision == "" {
		return errors.New("capture revision required")
	}
	if m.Contracts == nil {
		return errors.New("contracts must be present")
	}
	for id, hash := range m.Contracts {
		if !scenarioID.MatchString(id) || hash == "" {
			return fmt.Errorf("invalid contract for %s", id)
		}
	}
	seen := make(map[string]bool, len(m.Images))
	for _, image := range m.Images {
		name, err := Filename(image.ID, image.Checkpoint, image.Viewport, image.Theme)
		if err != nil {
			return err
		}
		if image.File != name || filepath.Base(image.File) != image.File {
			return fmt.Errorf("unsafe or unexpected screenshot file %q", image.File)
		}
		if image.Width <= 0 || image.Height <= 0 {
			return fmt.Errorf("invalid dimensions for %s", name)
		}
		if _, ok := m.Contracts[image.ID]; !ok {
			return fmt.Errorf("missing contract for %s", image.ID)
		}
		if seen[image.File] {
			return fmt.Errorf("duplicate screenshot %s", image.File)
		}
		seen[image.File] = true
	}
	if len(m.Images) > 0 && m.Environment.Browser == "" {
		return errors.New("browser version required for captures")
	}
	return nil
}

// Save writes manifest.json to an existing or new capture directory.
func (m Manifest) Save(dir string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(b, '\n'), 0o644)
}

// ReadManifest reads and validates a capture directory's manifest.json.
func ReadManifest(dir string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, m.Validate()
}
