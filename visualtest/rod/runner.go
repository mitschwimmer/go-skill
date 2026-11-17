// Package rod executes visualtest scenarios in real Chromium using Rod. It
// supplies browser lifecycle, reproducible settings and named screenshot
// checkpoints, but never mocks application APIs or defines product assertions.
package rod

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	browserrod "github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/mitschwimmer/go-skill/visualtest"
)

// Case is application-owned behavior. ContractFiles identifies project files
// that define its fixture and assertions, excluding the real UI assets.
// Include enough inputs to detect changes in behavioral setup across revisions.
type Case struct {
	Fixture       string
	ContractFiles []string
	InitScript    string
	Serve         func(context.Context, string) (origin string, cleanup func(), err error)
	Run           func(context.Context, *browserrod.Page, func(checkpoint string) error) error
}

// Runner configures one real Chromium process reused across isolated contexts.
type Runner struct {
	Cases     map[string]Case
	Viewports map[string]visualtest.Size
	Browser   string // executable path; otherwise use VISUALTEST_BROWSER_EXECUTABLE or PATH
	Locale    string
	Timezone  string
	NoSandbox bool // enable only where the CI environment requires it
	Timeout   time.Duration
}

// Options selects the checked-out source tree and capture destination.
// MatchContracts optionally skips baseline captures for contracts that changed
// on the PR branch; such scenarios remain present in manifest.Contracts.
type Options struct {
	ProjectRoot    string
	Definitions    string
	Assets         string
	Output         string
	Revision       string
	Only           string
	MatchContracts map[string]string
}

func contract(root string, def visualtest.Definition, files []string) (string, error) {
	h := sha256.New()
	_, _ = h.Write([]byte(def.Fingerprint()))
	for _, name := range files {
		clean := filepath.Clean(name)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("contract path must be relative to project root: %s", name)
		}
		_, _ = h.Write([]byte("\x00" + filepath.ToSlash(clean) + "\x00"))
		data, err := os.ReadFile(filepath.Join(root, clean))
		if errors.Is(err, os.ErrNotExist) { // old commits may lack the test implementation
			_, _ = h.Write([]byte("MISSING"))
			continue
		}
		if err != nil {
			return "", err
		}
		_, _ = h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r Runner) defaults() Runner {
	if r.Locale == "" {
		r.Locale = "en-US"
	}
	if r.Timezone == "" {
		r.Timezone = "UTC"
	}
	if r.Timeout <= 0 {
		r.Timeout = 15 * time.Second
	}
	return r
}

func (r Runner) connect(ctx context.Context) (*browserrod.Browser, func(), error) {
	binary := r.Browser
	if binary == "" {
		binary = os.Getenv("VISUALTEST_BROWSER_EXECUTABLE")
	}
	if binary == "" {
		if path, ok := launcher.LookPath(); ok {
			binary = path
		}
	}
	if binary == "" {
		return nil, nil, errors.New("Chromium not found; configure the browser executable (automatic downloads are disabled)")
	}
	l := launcher.New().Bin(binary).Headless(true).NoSandbox(r.NoSandbox).Set("disable-dev-shm-usage")
	url, err := l.Launch()
	if err != nil {
		return nil, nil, err
	}
	browser := browserrod.New().ControlURL(url).NoDefaultDevice().Context(ctx)
	if err := browser.Connect(); err != nil {
		l.Kill()
		return nil, nil, err
	}
	return browser, func() { _ = browser.Close(); l.Kill() }, nil
}

// Capture validates every declared scenario, then runs each selected viewport
// and color scheme in a clean browser context. Scenario code must call snapshot
// once for every declared checkpoint, in the desired sequence.
func (r Runner) Capture(ctx context.Context, opts Options) (visualtest.Manifest, error) {
	r = r.defaults()
	var m visualtest.Manifest
	if opts.ProjectRoot == "" || opts.Revision == "" {
		return m, errors.New("ProjectRoot and Revision are required")
	}
	if opts.Definitions == "" {
		opts.Definitions = filepath.Join(opts.ProjectRoot, "tests", "scenarios")
	}
	if opts.Assets == "" {
		opts.Assets = filepath.Join(opts.ProjectRoot, "web")
	}
	defs, err := visualtest.Load(opts.Definitions)
	if err != nil {
		return m, err
	}
	m = visualtest.Manifest{Format: visualtest.Format, Revision: opts.Revision,
		Environment: visualtest.Environment{Platform: runtime.GOOS + "/" + runtime.GOARCH, Locale: r.Locale,
			Timezone: r.Timezone, DeviceScaleFactor: 1, ReducedMotion: "reduce"},
		Contracts: make(map[string]string), Images: []visualtest.Image{}}
	selected := make([]visualtest.Definition, 0, len(defs))
	for _, def := range defs {
		if opts.Only != "" && def.ID != opts.Only {
			continue
		}
		c, exists := r.Cases[def.ID]
		if !exists {
			if opts.MatchContracts == nil {
				return m, fmt.Errorf("scenario %s has no registered browser case", def.ID)
			}
			m.Contracts[def.ID] = visualtest.Hash([]byte(def.Fingerprint() + "NO_CASE"))
			continue // an old, now-retired case
		}
		if c.Fixture != def.Fixture || c.Serve == nil || c.Run == nil {
			return m, fmt.Errorf("scenario %s has incompatible fixture or missing Serve/Run", def.ID)
		}
		fingerprint, err := contract(opts.ProjectRoot, def, c.ContractFiles)
		if err != nil {
			return m, err
		}
		m.Contracts[def.ID] = fingerprint
		if opts.MatchContracts != nil && opts.MatchContracts[def.ID] != fingerprint {
			continue
		}
		for _, viewport := range def.Viewports {
			size, ok := r.Viewports[viewport]
			if !ok || size.Width <= 0 || size.Height <= 0 {
				return m, fmt.Errorf("scenario %s: unknown viewport %s", def.ID, viewport)
			}
		}
		selected = append(selected, def)
	}
	if opts.Only != "" {
		if _, exists := m.Contracts[opts.Only]; !exists {
			return m, fmt.Errorf("unknown scenario %s", opts.Only)
		}
	}
	if opts.Output != "" {
		if err := os.MkdirAll(opts.Output, 0o755); err != nil {
			return m, err
		}
	}
	if len(selected) > 0 {
		browser, closeBrowser, err := r.connect(ctx)
		if err != nil {
			return m, err
		}
		defer closeBrowser()
		version, err := browser.Version()
		if err != nil {
			return m, err
		}
		m.Environment.Browser = version.Product
		for _, def := range selected {
			for _, viewport := range def.Viewports {
				for _, theme := range def.Themes {
					records, err := r.captureVariant(ctx, browser, def, viewport, theme, opts.Assets, opts.Output)
					if err != nil {
						return m, fmt.Errorf("%s/%s/%s: %w", def.ID, viewport, theme, err)
					}
					m.Images = append(m.Images, records...)
				}
			}
		}
	}
	if opts.Output != "" {
		if err := m.Save(opts.Output); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (r Runner) captureVariant(ctx context.Context, browser *browserrod.Browser, def visualtest.Definition, viewport, theme, assets, out string) ([]visualtest.Image, error) {
	c := r.Cases[def.ID]
	url, stop, err := c.Serve(ctx, assets)
	if err != nil {
		return nil, err
	}
	if stop == nil {
		return nil, fmt.Errorf("scenario %s did not provide server cleanup", def.ID)
	}
	defer stop()
	incognito, err := browser.Incognito()
	if err != nil {
		return nil, err
	}
	defer incognito.Close()
	page, err := incognito.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, err
	}
	defer page.Close()
	page = page.Timeout(r.Timeout)
	size := r.Viewports[viewport]
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: size.Width, Height: size.Height, DeviceScaleFactor: 1, Mobile: false}); err != nil {
		return nil, err
	}
	media := proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "prefers-color-scheme", Value: theme}, {Name: "prefers-reduced-motion", Value: "reduce"},
	}}
	if err := media.Call(page); err != nil {
		return nil, err
	}
	if err := (proto.EmulationSetLocaleOverride{Locale: r.Locale}).Call(page); err != nil {
		return nil, err
	}
	if err := (proto.EmulationSetTimezoneOverride{TimezoneID: r.Timezone}).Call(page); err != nil {
		return nil, err
	}
	if c.InitScript != "" {
		if _, err := page.EvalOnNewDocument(c.InitScript); err != nil {
			return nil, err
		}
	}
	if err := page.Navigate(url); err != nil {
		return nil, err
	}
	checkpoints := make(map[string]bool, len(def.Checkpoints))
	for _, name := range def.Checkpoints {
		checkpoints[name] = true
	}
	var images []visualtest.Image
	shot := func(checkpoint string) error {
		if !checkpoints[checkpoint] {
			return fmt.Errorf("undeclared checkpoint %s#%s", def.ID, checkpoint)
		}
		for _, image := range images {
			if image.Checkpoint == checkpoint {
				return fmt.Errorf("duplicate checkpoint %s#%s", def.ID, checkpoint)
			}
		}
		if _, err := page.Eval(`() => document.fonts.ready.then(() => true)`); err != nil {
			return err
		}
		// Keep incidental animations/caret out of reproducible still images.
		if _, err := page.Eval(`() => {
          if (!document.querySelector('#visualtest-freeze')) {
            const s = document.createElement('style'); s.id = 'visualtest-freeze';
            s.textContent = '*,*::before,*::after{animation-play-state:paused!important;transition-duration:0s!important;caret-color:transparent!important}';
            document.head.appendChild(s);
          }
          document.activeElement?.blur(); return true;
        }`); err != nil {
			return err
		}
		name, err := visualtest.Filename(def.ID, checkpoint, viewport, theme)
		if err != nil {
			return err
		}
		if out != "" {
			data, err := page.Screenshot(true, nil)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
				return err
			}
		}
		images = append(images, visualtest.Image{ID: def.ID, Checkpoint: checkpoint, Viewport: viewport,
			Theme: theme, Width: size.Width, Height: size.Height, File: name})
		return nil
	}
	if err := c.Run(ctx, page, shot); err != nil {
		return nil, err
	}
	if len(images) != len(checkpoints) {
		return nil, fmt.Errorf("scenario %s: captured %d of %d checkpoints", def.ID, len(images), len(checkpoints))
	}
	return images, nil
}
