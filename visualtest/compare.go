package visualtest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Change describes the comparison status of one checkpoint variant.
type Change struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	Before        string `json:"beforeFile,omitempty"`
	After         string `json:"afterFile,omitempty"`
	Difference    string `json:"differenceFile,omitempty"`
	ChangedPixels int    `json:"changedPixels,omitempty"`
	TotalPixels   int    `json:"totalPixels,omitempty"`
}

// Summary contains the machine-readable result and only non-unchanged entries.
type Summary struct {
	BaseRevision      string         `json:"baseRevision"`
	CandidateRevision string         `json:"candidateRevision"`
	Browser           string         `json:"browser"`
	Counts            map[string]int `json:"counts"`
	Entries           []Change       `json:"entries"`
}

// Difference compares decoded pixel colors, ignoring PNG encoding and metadata.
// A dimension mismatch returns sameSize=false and is still a visual change.
func Difference(a, b image.Image) (sameSize bool, changed int, diff *image.RGBA) {
	if a.Bounds().Dx() != b.Bounds().Dx() || a.Bounds().Dy() != b.Bounds().Dy() {
		return false, 0, nil
	}
	bounds := image.Rect(0, 0, a.Bounds().Dx(), a.Bounds().Dy())
	out := image.NewRGBA(bounds)
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			c1 := a.At(a.Bounds().Min.X+x, a.Bounds().Min.Y+y)
			c2 := b.At(b.Bounds().Min.X+x, b.Bounds().Min.Y+y)
			r1, g1, b1, a1 := c1.RGBA()
			r2, g2, b2, a2 := c2.RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				changed++
				out.SetRGBA(x, y, color.RGBA{R: 245, G: 65, B: 70, A: 255})
				continue
			}
			c := color.NRGBAModel.Convert(c2).(color.NRGBA)
			gray := uint8((uint16(c.R) + uint16(c.G) + uint16(c.B)) / 3)
			out.SetRGBA(x, y, color.RGBA{R: gray / 2, G: gray / 2, B: gray / 2, A: 255})
		}
	}
	return true, changed, out
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return im, nil
}

func copyImage(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return cpErr
	}
	return closeErr
}

func imagesByName(m Manifest) map[string]Image {
	images := make(map[string]Image, len(m.Images))
	for _, image := range m.Images {
		images[image.File] = image
	}
	return images
}

// Compare compares two capture directories and writes a changed-only HTML gallery
// and summary.json. It rejects missing captures and incompatible environments;
// scenario-contract changes are never reported as like-for-like visual regressions.
func Compare(baseDir, candidateDir, outputDir string) (Summary, error) {
	var result Summary
	base, err := ReadManifest(baseDir)
	if err != nil {
		return result, fmt.Errorf("baseline: %w", err)
	}
	candidate, err := ReadManifest(candidateDir)
	if err != nil {
		return result, fmt.Errorf("candidate: %w", err)
	}
	compatible := false
	for id, contract := range candidate.Contracts {
		if base.Contracts[id] == contract {
			compatible = true
		}
	}
	if compatible && len(base.Images) > 0 && base.Environment != candidate.Environment {
		return result, errors.New("incompatible browser environment: baseline and candidate must use the same Chromium, locale, fonts, viewport settings and OS")
	}
	result = Summary{BaseRevision: base.Revision, CandidateRevision: candidate.Revision, Browser: candidate.Environment.Browser,
		Counts: map[string]int{"unchanged": 0, "changed": 0, "added": 0, "removed": 0, "contract-changed": 0}, Entries: []Change{}}
	for _, subdir := range []string{"images/main", "images/candidate", "images/diff"} {
		if err := os.MkdirAll(filepath.Join(outputDir, subdir), 0o755); err != nil {
			return result, err
		}
	}
	bases := imagesByName(base)
	candidates := imagesByName(candidate)
	ids := make([]string, 0, len(candidate.Contracts))
	for id := range candidate.Contracts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		contract := candidate.Contracts[id]
		baseContract, exists := base.Contracts[id]
		status := "added"
		if exists && baseContract != contract {
			status = "contract-changed"
		} else if exists {
			status = "unchanged"
		}
		found := false
		for _, image := range candidate.Images {
			if image.ID != id {
				continue
			}
			found = true
			row := Change{Name: image.File, Status: status}
			currentPath := filepath.Join(candidateDir, image.File)
			current, err := readPNG(currentPath)
			if err != nil {
				return result, err
			}
			old, oldExists := bases[image.File]
			_ = old
			if status == "unchanged" {
				if !oldExists {
					return result, fmt.Errorf("missing baseline screenshot %s for unchanged scenario contract", image.File)
				}
				before, err := readPNG(filepath.Join(baseDir, image.File))
				if err != nil {
					return result, err
				}
				sameSize, changed, diff := Difference(before, current)
				if !sameSize || changed != 0 {
					row.Status = "changed"
					row.ChangedPixels, row.TotalPixels = changed, current.Bounds().Dx()*current.Bounds().Dy()
					row.Before = filepath.ToSlash(filepath.Join("images/main", image.File))
					if err := copyImage(filepath.Join(baseDir, image.File), filepath.Join(outputDir, row.Before)); err != nil {
						return result, err
					}
					if sameSize {
						row.Difference = filepath.ToSlash(filepath.Join("images/diff", image.File))
						f, err := os.Create(filepath.Join(outputDir, row.Difference))
						if err != nil {
							return result, err
						}
						encErr := png.Encode(f, diff)
						closeErr := f.Close()
						if encErr != nil {
							return result, encErr
						}
						if closeErr != nil {
							return result, closeErr
						}
					}
				}
			} else if oldExists { // context without claiming an equivalent baseline
				row.Before = filepath.ToSlash(filepath.Join("images/main", image.File))
				if err := copyImage(filepath.Join(baseDir, image.File), filepath.Join(outputDir, row.Before)); err != nil {
					return result, err
				}
			}
			result.Counts[row.Status]++
			if row.Status == "unchanged" {
				continue
			}
			row.After = filepath.ToSlash(filepath.Join("images/candidate", image.File))
			if err := copyImage(currentPath, filepath.Join(outputDir, row.After)); err != nil {
				return result, err
			}
			result.Entries = append(result.Entries, row)
		}
		if !found {
			return result, fmt.Errorf("scenario %s has no candidate captures", id)
		}
	}
	for _, image := range base.Images {
		contract, exists := candidate.Contracts[image.ID]
		if !exists {
			row := Change{Name: image.File, Status: "removed", Before: filepath.ToSlash(filepath.Join("images/main", image.File))}
			if err := copyImage(filepath.Join(baseDir, image.File), filepath.Join(outputDir, row.Before)); err != nil {
				return result, err
			}
			result.Counts["removed"]++
			result.Entries = append(result.Entries, row)
		} else if contract == base.Contracts[image.ID] {
			if _, ok := candidates[image.File]; !ok {
				return result, fmt.Errorf("candidate missing screenshot %s despite unchanged scenario contract", image.File)
			}
		}
	}
	// Retired scenario contracts may have no captured images in their manifest.
	for id := range base.Contracts {
		if _, exists := candidate.Contracts[id]; exists {
			continue
		}
		hadImage := false
		for _, image := range base.Images {
			if image.ID == id {
				hadImage = true
				break
			}
		}
		if !hadImage {
			result.Counts["removed"]++
			result.Entries = append(result.Entries, Change{Name: id, Status: "removed"})
		}
	}
	if err := writeSummary(outputDir, result); err != nil {
		return result, err
	}
	return result, nil
}

func writeSummary(dir string, result Summary) error {
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	var out bytes.Buffer
	if err := galleryTemplate.Execute(&out, result); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "index.html"), out.Bytes(), 0o644)
}

var galleryTemplate = template.Must(template.New("gallery").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Visual scenarios</title><style>
body{margin:2rem auto;max-width:1100px;padding:0 1rem;font:16px system-ui;background:#171d1a;color:#f7f5ee}small,figcaption{color:#abb8ae}section{border-top:1px solid #526157;padding:1rem 0 2rem}.images{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,300px),1fr));gap:1rem}figure{margin:0;min-width:0}img{display:block;max-width:100%;height:auto;border:1px solid #526157}figcaption{padding:.5rem 0}
</style><main><h1>Visual scenario review</h1><p>Baseline {{.BaseRevision}} → candidate {{.CandidateRevision}}</p><p>{{range $name, $count := .Counts}}{{$name}}: {{$count}} · {{end}}</p><p>Only changed, added, removed, and contract-changed views are shown.</p>
{{if not .Entries}}<p>No visual changes.</p>{{end}}
{{range .Entries}}<section><h2>{{.Name}} <small>{{.Status}}</small></h2><div class="images">
{{if .Before}}<figure><img loading="lazy" src="{{.Before}}" alt="Baseline"><figcaption>Baseline</figcaption></figure>{{end}}
{{if .After}}<figure><img loading="lazy" src="{{.After}}" alt="Candidate"><figcaption>Candidate</figcaption></figure>{{end}}
{{if .Difference}}<figure><img loading="lazy" src="{{.Difference}}" alt="Pixel difference"><figcaption>Difference</figcaption></figure>{{end}}
</div></section>{{end}}</main></html>`))

// MarkdownSummary returns a compact GitHub Actions step summary without any
// platform dependency; callers can append it to GITHUB_STEP_SUMMARY.
func (s Summary) MarkdownSummary() string {
	names := []string{"unchanged", "changed", "added", "removed", "contract-changed"}
	var b strings.Builder
	b.WriteString("### Browser visual scenarios\n\n| Status | Views |\n|---|---:|\n")
	for _, name := range names {
		fmt.Fprintf(&b, "| %s | %d |\n", name, s.Counts[name])
	}
	b.WriteString("\nSee the changed-only HTML gallery in the uploaded artifact.\n")
	return b.String()
}
