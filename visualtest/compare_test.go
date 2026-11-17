package visualtest

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func pngFile(t *testing.T, path string, c color.Color) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	im := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			im.Set(x, y, c)
		}
	}
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
func captureManifest(t *testing.T, dir, revision, contract string) string {
	t.Helper()
	name, _ := Filename("chat.reply.streaming", "waiting", "mobile", "dark")
	m := Manifest{Format: Format, Revision: revision, Environment: Environment{
		Browser: "Chrome/135", Platform: "linux/amd64", Locale: "de-DE", Timezone: "Europe/Berlin", DeviceScaleFactor: 1, ReducedMotion: "reduce",
	}, Contracts: map[string]string{"chat.reply.streaming": contract}, Images: []Image{{ID: "chat.reply.streaming", Checkpoint: "waiting", Viewport: "mobile", Theme: "dark", Width: 390, Height: 844, File: name}}}
	if err := m.Save(dir); err != nil {
		t.Fatal(err)
	}
	return name
}
func TestCompare(t *testing.T) {
	for _, tc := range []struct {
		name, contract                  string
		changed, added, contractChanged int
	}{
		{"equal", "v1", 0, 0, 0}, {"different pixels", "v1", 1, 0, 0}, {"changed contract", "v2", 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			a := filepath.Join(root, "base")
			b := filepath.Join(root, "candidate")
			out := filepath.Join(root, "report")
			name := captureManifest(t, a, "old", "v1")
			captureManifest(t, b, "new", tc.contract)
			pngFile(t, filepath.Join(a, name), color.RGBA{200, 80, 40, 255})
			c := color.RGBA{200, 80, 40, 255}
			if tc.name == "different pixels" {
				c = color.RGBA{190, 80, 40, 255}
			}
			pngFile(t, filepath.Join(b, name), c)
			summary, err := Compare(a, b, out)
			if err != nil {
				t.Fatal(err)
			}
			if summary.Counts["changed"] != tc.changed || summary.Counts["contract-changed"] != tc.contractChanged {
				t.Fatalf("wrong statuses: %+v", summary.Counts)
			}
			if tc.name == "different pixels" {
				if len(summary.Entries) != 1 || summary.Entries[0].ChangedPixels != 6 || summary.Entries[0].Difference == "" {
					t.Fatalf("bad pixel diff: %+v", summary.Entries)
				}
				if _, err := os.Stat(filepath.Join(out, summary.Entries[0].Difference)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestCompareMissingCapture(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "base")
	b := filepath.Join(root, "candidate")
	name := captureManifest(t, a, "old", "v1")
	captureManifest(t, b, "new", "v1")
	pngFile(t, filepath.Join(a, name), color.White)
	if _, err := Compare(a, b, filepath.Join(root, "report")); err == nil {
		t.Fatal("missing candidate capture accepted")
	}
}
func TestCompareNewAndRemoved(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "base")
	b := filepath.Join(root, "candidate")
	name := captureManifest(t, a, "old", "v1")
	pngFile(t, filepath.Join(a, name), color.White)
	m := Manifest{Format: Format, Revision: "new", Contracts: map[string]string{}, Images: []Image{}}
	if err := m.Save(b); err != nil {
		t.Fatal(err)
	}
	s, err := Compare(a, b, filepath.Join(root, "report"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Counts["removed"] != 1 {
		t.Fatalf("removed: %+v", s.Counts)
	}
	s, err = Compare(b, a, filepath.Join(root, "report2"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Counts["added"] != 1 {
		t.Fatalf("added: %+v", s.Counts)
	}
}
func TestDifferenceDimensions(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 1, 1))
	b := image.NewRGBA(image.Rect(0, 0, 2, 2))
	same, _, diff := Difference(a, b)
	if same || diff != nil {
		t.Fatal("dimension mismatch accepted")
	}
}
