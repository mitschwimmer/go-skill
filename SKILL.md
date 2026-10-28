---
name: archaic-go
description: Develop, review, maintain, and package Go projects using Archaic Go conventions on NixOS. Use for Go project scaffolding, Go code changes, development environment design, build/test workflows, Helix integration, project maintenance skills, and SOPS/age secrets handling when the user requests Archaic Go or works in a repository adopting these conventions.
---

# Archaic Go

Favor minimal, explicit, idiomatic Go projects. Keep Go responsible for modules, builds, tests, and developer feedback; use Nix for toolchain provisioning and reproducible packaging.

## Software Design

**Design principle:** Prefer the simplest design that satisfies current requirements while preserving clear boundaries for foreseeable change. Add complexity only to address demonstrated needs. When departing from these conventions, explain the tradeoff. Simplicity is not the same as minimizing lines, packages, or dependencies.

### Dependencies
- Prefer the Go standard library. Justify third-party dependencies against maintenance, security, and conceptual cost.
- Reuse suitable existing dependencies before adding alternatives.
- Avoid libraries that only wrap straightforward standard-library functionality.

### Package organization and abstractions
- Organize packages around cohesive capabilities, not prescribed architectural layers.
- Start with few packages; extract boundaries when responsibilities, ownership, or dependencies justify them.
- Keep public APIs small and implementation details unexported. Use `internal` when it enforces useful boundaries.
- Avoid speculative layers, generalized frameworks, and premature code generation.
- Define small interfaces near consumers when substitution is useful; return concrete types by default. Do not create interfaces solely for mocking.
- Use generics for genuine type-safe reuse, not for building an abstraction framework.

### Control flow
- Avoid nested loops and nested `if` clauses. Prefer guard clauses, early returns, and `continue` to keep the main path flat.
- Extract cohesive operations into small, clearly named functions when iteration or branching would otherwise nest. Keep ordinary error checks inside loops where needed; do not obscure the algorithm merely to eliminate indentation.

### Errors and state
- Return errors for expected failures; reserve panics for unrecoverable programming defects.
- Add operational context with `fmt.Errorf` and `%w`; use `errors.Is` and `errors.As` for meaningful classification.
- Expose sentinel or typed errors only as needed by callers. Emit failure logs at handling boundaries rather than repeating the same error at every layer; allow diagnostic events throughout a request to feed its flight recorder.
- Make ownership and lifecycle of mutable state explicit; avoid mutable package-level globals.
- Construct and validate configuration at startup, then pass the required values explicitly. Keep SOPS out of application logic.

### Concurrency and resources
- Start synchronous. Introduce concurrency only to satisfy a real requirement.
- Give every goroutine a clear owner, termination condition, and error policy; propagate cancellation using `context.Context`.
- Bound concurrent work. Use channels for communication and coordination, and mutexes where shared state needs protection.
- Close files, rows, responses, and other resources promptly; define graceful shutdown for long-lived services.

### Database access
- Prefer `database/sql` with explicit, handwritten SQL.
- Keep queries readable, reviewable, and close to their consumers; parameterize values.
- Define transaction boundaries deliberately; pass contexts and close rows and other resources.
- Add ORMs, query builders, or SQL generators only for demonstrated benefits.

### HTTP applications and observability
- Prefer `net/http` and its standard `http.ServeMux`, with standard handlers and middleware.
- Separate transport details from application behavior when it makes boundaries clearer.
- Configure appropriate timeouts and graceful shutdown; justify framework or router dependencies.
- Use `log/slog` for structured logging. Propagate request context and avoid leaking secrets.
- For HTTP server applications, use the flight recorder log pattern: collect structured events through debug level in a buffer owned by each request, starting at request entry and continuing through response completion. Discard the buffer on success; emit it only when something goes wrong during that lifecycle.
- Record failure explicitly, including application or dependency errors even when handled, error responses (4xx/5xx), panics, cancellation or deadline expiry, and response-write failures. Do not infer success solely from the HTTP status; a response may fail after headers have been sent.
- Finalize at the outer request boundary, after response handling and panic recovery. Flush failed requests once, in recorded order, with original levels, timestamps, attributes, request correlation, and the failure reason. Preserve the server's panic policy.
- Enable debug capture in the recorder's `slog.Handler.Enabled`; do not let the output handler's normal level threshold discard buffered debug records during replay. Preserve `WithAttrs` and `WithGroup` semantics and clone records before retaining them.
- Bound each request's buffer by records or bytes, report truncation when flushing, and synchronize access from concurrent request work. Keep buffers isolated and define ownership through completion so late goroutines cannot race finalization or silently lose failure events.
- Redact sensitive data before buffering and release retained data on completion. Keep startup and background logging outside the request recorder.
- Test that successful requests emit nothing, every failure path flushes diagnostics through debug exactly once, and concurrent requests remain isolated. Include buffer overflow, panic recovery, cancellation, and response-write failures.

### Testing
- Use the standard `testing` package and test observable behavior rather than implementation details.
- Prefer real components and lightweight fixtures to mocks; use test doubles only at meaningful boundaries.
- Keep tests deterministic with explicit setup and cleanup. Use table-driven tests when they improve readability.
- Run relevant unit and integration tests; use race detection when concurrency is involved.

### API protocols and compatibility
- Choose API protocols per project based on interoperability, performance, evolution, tooling, and operations.
- Follow established protocol conventions instead of inventing custom ones; impose no universal REST, gRPC, or GraphQL choice.
- Once APIs have consumers, define compatibility expectations explicitly and avoid accidental breaking changes.

## Project maintenance skills

- Create a dedicated skill for every project, teaching both human maintainers and coding agents how to maintain and extend it. Keep the skill versioned with the project and name it for that project.
- Use this skill as the primary project documentation in place of classic documentation. Write actionable workflows and decision guidance; avoid maintaining parallel narrative guides that repeat it.
- Cover purpose, architecture and invariants, code navigation, environment and configuration, build/test/run commands, common changes and extension points, debugging, and release or operational procedures as applicable.
- Keep `SKILL.md` concise with clear triggering metadata; put detailed procedures and reusable helpers in linked references or scripts. Make the entry point discoverable from the repository's existing agent instructions or a minimal link.
- Read the project's skill before changing the project and update it alongside changes to behavior, architecture, tooling, or maintenance procedures. For an existing project without one, create it from verified repository facts; do not invent commands or procedures.

## Task-specific guidance

- **New Go project or environment setup:** Read [references/new-project.md](references/new-project.md) before creating files, selecting tooling, or defining a development flake. Do not load it for routine code changes or reviews.
- **Existing project changes:** Read the project's maintenance skill and inspect current patterns first; apply the principles above incrementally. Use ordinary Go tools to validate the relevant changes and report any checks not run.

## Boundaries

- Avoid replacing Go modules with Nix-managed module imports or making `nix build` the interactive edit-test loop.
- Avoid treating `nix run` as a universal task runner. Prefer direct Go commands inside the shell; add flake apps for meaningful repeatable entry points only.
- Keep workstation-global shell, Helix, and Git configuration distinct from project-specific compiler and tooling.
- Keep third-party precompiled Linux binaries in mind: executables expecting FHS interpreter paths may fail on NixOS. Prefer native nixpkgs packages or build from source.
- When upstream code or the user's explicit requirements differ from these defaults, preserve compatibility and explain the tradeoff rather than imposing conventions blindly.
