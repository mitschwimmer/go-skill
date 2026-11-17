# Archaic Go visualtest library

The Go module in this repository (`github.com/mitschwimmer/go-skill`) supplies a standard-library core at `visualtest` and a Rod adapter at `visualtest/rod`. Use it for applications that benefit from reproducible browser screenshots and PR visual review. Keep the dependency in test tooling, not production binaries.

## Division of responsibility

- `visualtest`: parse and validate stable `area.capability.case` definitions, record capture manifests, compare decoded PNG pixels and create changed-only HTML/JSON reports. No browser dependencies in this package.
- `visualtest/rod`: launch **installed** Chromium, isolate browser contexts, fix viewport/theme/locale/timezone/reduced-motion, run application-owned cases and capture every named checkpoint. This package imports Rod.
- Application code: synthetic HTTP fixtures, real UI assets, browser interaction/assertions, scenario files, and paths containing fixture/expectation code.
- CI: check out base commit and proposed merge revision, invoke the same test runner against both asset trees, compare reports, and upload artifacts. Do not elevate permissions for PR code.

## Minimal application adapter

Run under a test-only Go module or `tests/browser` command in the application. Register cases in ordinary Go. This example abbreviates the HTTP fixture:

```go
suite := rodtest.Runner{
    Browser: os.Getenv("VISUALTEST_BROWSER_EXECUTABLE"),
    Locale: "de-DE", Timezone: "Europe/Berlin",
    Viewports: map[string]visualtest.Size{
        "desktop": {Width: 1071, Height: 1579},
        "mobile": {Width: 390, Height: 844},
    },
    Cases: map[string]rodtest.Case{
        "chat.reply.streaming": {
            Fixture: "streaming-reply",
            ContractFiles: []string{"tests/browser/streaming_fixture.go", "tests/browser/streaming_case.go"},
            Serve: startStreamingFixture, // func(context.Context, assetsDir string) (origin string, cleanup func(), err error)
            Run: func(ctx context.Context, page *rod.Page, shot func(string) error) error {
                // Interact and assert using ordinary Rod methods.
                if err := page.Wait(rod.Eval(`() => !!document.querySelector('.response-loader')`)); err != nil { return err }
                return shot("waiting")
            },
        },
    },
}
manifest, err := suite.Capture(ctx, rodtest.Options{
    ProjectRoot: checkout, Assets: filepath.Join(checkout, "web"),
    Revision: commitSHA, Output: screenshotDir,
})
```

Import `github.com/mitschwimmer/go-skill/visualtest` as `visualtest`, `github.com/mitschwimmer/go-skill/visualtest/rod` as `rodtest`, and `github.com/go-rod/rod` as `rod`. `shot("waiting")` takes a real PNG after asserting the desired browser state. Multiple declared checkpoints can be taken in order; calling one twice or omitting one is an error. Browser options do not replace application assertions.

## Baselines and contracts

A scenario's contract is the canonical JSON definition **plus the contents of its `ContractFiles`**. Include all source files that determine its fixture data or test expectations, but do **not** include HTML/CSS/JS application assets: those are the intended subjects of visual comparison. Split scenario-specific fixtures if unrelated changes would otherwise invalidate the entire catalog.

Capture the candidate with the current checkout and save its returned `Manifest`. Capture `main` using the **same compiled candidate runner**, with `ProjectRoot`, `Definitions`, and `Assets` pointing at the exact base checkout, and `MatchContracts` set to `candidateManifest.Contracts`. The adapter skips incompatible contracts but retains their identities for the report. This prevents changed fixtures from being described as like-for-like regressions. A baseline with no catalog still yields a valid empty manifest, allowing new scenarios to be reported as **added**.

Compare with:

```sh
go run ./cmd/visualtest validate tests/scenarios
go run ./cmd/visualtest compare --base /tmp/main-shots --candidate /tmp/pr-shots --out /tmp/visual-report
```

The comparator rejects missing same-contract captures and incompatible browser environments. It classifies unchanged, changed, added, removed and contract-changed views, writes real PNG difference images where dimensions agree, and renders a static changed-only `index.html` and `summary.json`. `GITHUB_STEP_SUMMARY` is populated if available.

## Reproducibility limits

- Use the **same browser executable, fonts, OS image, screenshot settings and locale** for both checkouts. A change in fonts/OS can cause legitimate pixel differences; the manifest can check only declared environment settings, not all installed fonts.
- Pin the Go module and browser installation separately. On NixOS point `VISUALTEST_BROWSER_EXECUTABLE` at a suitable packaged browser; the adapter will not download a Linux binary.
- Fixture timestamps and randomness are the application's responsibility. Use `Case.InitScript` only for explicit deterministic browser-side setup.
- CI can set `Runner.NoSandbox` only when its sandbox/container configuration requires it. Keep Chromium sandboxing enabled on normal workstations.
- The library does **not** decide what to merge or automatically publish artifacts. Keep visual diffs reviewable; fail CI on assertion/capture errors.
- Existing Playwright suites need not be replaced until the native implementation has demonstrated equivalent coverage. Do not drop unrelated Node.js tests without verified replacements.

## Maintenance

Run `go test ./...` and `go vet ./...` from this repository. Real-browser smoke tests run when an installed Chromium is found. Keep public APIs small; build browser automation from Rod and HTTP fixtures from Go's `httptest`, not a second browser abstraction language. Change `SKILL.md` and this reference in the same PR when supported conventions change. After merging, tag a compatible Go module release before consuming it from independent projects.
