# Scenario-driven browser UI testing

Use this convention for browser-facing Archaic Go applications where screenshots help develop features and review pull requests. Do not impose browser tooling on API-only or CLI projects. A scenario defines reproducible behavior and state; a screenshot is evidence produced from that scenario, not its definition.

## Identity and definition

- Give each scenario a stable, descriptive ID with exactly three dot-separated lowercase ASCII segments: `<area>.<capability>.<case>`, e.g. `chat.reply.streaming`, `auth.login.required`, `chat.history.populated`. Within a segment, use letters, digits and internal hyphens. Describe observable behavior, not DOM selectors, implementation functions, issues, or branch names.
- Name an observable checkpoint with `#<checkpoint>` when one scenario has multiple meaningful states, e.g. `chat.reply.streaming#waiting` or `#partial`. Keep viewport, theme, browser and revision outside the scenario ID.
- Keep IDs stable through implementation or styling changes. Create or explicitly retire IDs when their behavioral meaning changes; do not silently reuse one for a different state.
- Store small, reviewable definitions in `tests/scenarios/<id>.json` (or the existing project's equivalent), one file per scenario. Prefer JSON so Go and Node.js can read it without another parser. Definitions specify intent, fixture and selected checkpoints/variants; keep browser actions and assertions in test code rather than creating a JSON testing DSL.
- Require at least one automated browser test to reference every scenario ID. Validate IDs, duplicate definitions, fixture references and declared checkpoints before capture. Do not require a screenshot for every assertion or test case.

Example `tests/scenarios/chat.reply.streaming.json`:

```json
{
  "id": "chat.reply.streaming",
  "description": "An assistant response is in progress",
  "fixture": "streaming-reply",
  "checkpoints": ["waiting", "partial"],
  "viewports": ["desktop", "mobile"],
  "themes": ["light", "dark"]
}
```

Resolve viewport names to explicitly sized, project-owned configurations. A screenshot can be named `chat.reply.streaming--waiting--mobile--dark.png`. Store the full Git revision, actual viewport dimensions, browser version and scenario/fixture revision in a capture manifest rather than embedding them in the stable ID.

## Execution and authenticity

- Run UI code from the checked-out revision through a **real headless Chromium browser**. Prefer Playwright (and Playwright Test for independently selectable scenarios) when already in use. Keep browser automation in development/CI only; do not add a frontend build or production JavaScript dependency just to take screenshots.
- Serve the actual HTML/CSS/JS assets over local HTTP. Use explicit, deterministic HTTP fixtures for remote services, including identity providers and model APIs; do not fabricate screenshots from HTML templates or add a production authentication bypass. Keep separate full-stack or live acceptance tests for integration boundaries that fixtures cannot establish.
- Test visible behavior and interactions before capture. Wait for an explicit ready state and fonts; use event/state-based waits instead of arbitrary sleeps. Make scenario setup independent and provide cleanup or fresh browser contexts so order does not affect results.
- Pin Chromium/Playwright versions together, operating system and fonts. Fix locale, timezone, viewport, device scale, theme, dates, test data and any relevant randomness. Freeze/hide the caret and disable incidental animations for still-image comparison. Capture animations in controlled, separately named checkpoints when motion itself matters.
- On NixOS, use a browser executable and libraries that match the development environment; do not assume a downloaded Linux Chromium runs unmodified. Reuse the project's pinned Nix toolchain where possible.
- Keep fixtures synthetic and safe for publication. Never put real user data, sessions, cookies, credentials or secrets into public CI screenshots or traces.

## Pull-request baseline and comparison

- Treat the target branch's **`main` commit at the PR base SHA** as the authoritative visual baseline. Render it and the PR's proposed merge result in the same CI environment, with the same browser/toolchain pins and capture configuration. Do not maintain a second, independently edited collection of golden PNGs by default.
- Match images by scenario ID, checkpoint and variant. Classify each as unchanged, changed, added or removed. If an existing scenario's definition or fixture changes across revisions, flag **scenario-contract changed** for explicit review; do not present its image difference as a like-for-like UI regression.
- Distinguish functional assertion failures, capture failures, missing images and unexecuted scenarios from ordinary visual changes. Fail required checks on test/capture errors; visual changes may be informational pending review. Never report a missing or failed capture as unchanged.
- Publish a PR-scoped report containing only changed, added, removed and contract-changed views, with before/after/diff images where comparisons are valid. Retain raw captures and a small machine-readable manifest as short-lived CI artifacts; record base/candidate SHA and capture environment. Do not commit generated screenshots on every PR.
- Optimize after measuring: reuse a Chromium process with isolated contexts, select only required scenario/variant combinations, and cache identical base captures by SHA and environment version only if re-rendering `main` becomes expensive. Real browser rendering remains non-negotiable.
- Keep untrusted PR execution read-only and free of secrets; do not run PR-supplied code in a privileged `pull_request_target` context.

## Developing and maintaining scenarios

- For a feature request naming `chat.reply.streaming#waiting`, locate the scenario definition, fixture, browser test and expected captures first; implement and test that behavior before comparing its image with `main`.
- Introduce or update scenario definitions and assertions in the same change as the behavior they describe. Report changed/added/retired scenarios and which checks were actually run.
- In each application-specific maintenance skill, document the scenario location, browser/test command, fixture boundaries, capture variants and CI comparison entry point. Adapt existing test infrastructure incrementally; replace Playwright with Go-native `chromedp` or `rod` only when a measured simplicity or performance benefit justifies owning the missing test/reporting features.
