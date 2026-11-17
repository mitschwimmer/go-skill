package rod

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	browserrod "github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/mitschwimmer/go-skill/visualtest"
)

func TestContractFingerprint(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture.go")
	if err := os.WriteFile(path, []byte("version one"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := visualtest.Definition{ID: "ui.button.clicked", Description: "Clicked button", Fixture: "sample", Checkpoints: []string{"ready"}, Viewports: []string{"desktop"}, Themes: []string{"light"}}
	a, err := contract(root, d, []string{"fixture.go"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version two"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := contract(root, d, []string{"fixture.go"})
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("fixture changes did not invalidate contract")
	}
	if _, err := contract(root, d, []string{"../secrets.txt"}); err == nil {
		t.Fatal("unsafe fixture path accepted")
	}
}

func TestCaptureRealChromium(t *testing.T) {
	binary := os.Getenv("VISUALTEST_BROWSER_EXECUTABLE")
	if binary == "" {
		if value, ok := launcher.LookPath(); ok {
			binary = value
		}
	}
	if binary == "" {
		t.Skip("Chromium is not installed; real browser test runs in CI")
	}
	root := t.TempDir()
	defs := filepath.Join(root, "tests", "scenarios")
	if err := os.MkdirAll(defs, 0o755); err != nil {
		t.Fatal(err)
	}
	definition := `{"id":"ui.button.clicked","description":"Clicking changes a label","fixture":"button","checkpoints":["ready","clicked"],"viewports":["desktop"],"themes":["light"]}`
	if err := os.WriteFile(filepath.Join(defs, "ui.button.clicked.json"), []byte(definition), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte("synthetic fixture v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	suite := Runner{Browser: binary, NoSandbox: os.Getenv("VISUALTEST_NO_SANDBOX") == "1", Viewports: map[string]visualtest.Size{"desktop": {Width: 640, Height: 480}}, Cases: map[string]Case{
		"ui.button.clicked": {Fixture: "button", ContractFiles: []string{"fixture.go"}, Serve: func(_ context.Context, _ string) (string, func(), error) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(`<!doctype html><html><body><button onclick="document.querySelector('p').textContent='Clicked'">Change</button><p>Ready</p></body></html>`))
			}))
			return srv.URL, srv.Close, nil
		}, Run: func(_ context.Context, page *browserrod.Page, shot func(string) error) error {
			if err := page.Wait(browserrod.Eval(`() => document.querySelector('p')?.textContent === 'Ready'`)); err != nil {
				return err
			}
			if err := shot("ready"); err != nil {
				return err
			}
			page.MustElement("button").MustClick()
			if err := page.Wait(browserrod.Eval(`() => document.querySelector('p')?.textContent === 'Clicked'`)); err != nil {
				return err
			}
			return shot("clicked")
		}},
	}}
	result, err := suite.Capture(context.Background(), Options{ProjectRoot: root, Output: filepath.Join(root, "capture"), Revision: "example-revision"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Images) != 2 {
		t.Fatalf("expected two checkpoints, got %d", len(result.Images))
	}
	if _, err := visualtest.Compare(filepath.Join(root, "capture"), filepath.Join(root, "capture"), filepath.Join(root, "report")); err != nil {
		t.Fatal(err)
	}
}
