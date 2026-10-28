---
name: archaic-go
description: Develop, review, maintain, and package Go projects using Archaic Go conventions on NixOS. Use for Go project scaffolding, Go code changes, development environment design, build/test workflows, Helix integration, Fish shell usage, and SOPS/age secrets handling when the user requests Archaic Go or works in a repository adopting these conventions.
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

### Errors and state
- Return errors for expected failures; reserve panics for unrecoverable programming defects.
- Add operational context with `fmt.Errorf` and `%w`; use `errors.Is` and `errors.As` for meaningful classification.
- Expose sentinel or typed errors only as needed by callers. Log at handling boundaries rather than at every layer.
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
- Use `log/slog` for structured logging when logging is needed. Propagate request context and avoid leaking secrets.

### Testing
- Use the standard `testing` package and test observable behavior rather than implementation details.
- Prefer real components and lightweight fixtures to mocks; use test doubles only at meaningful boundaries.
- Keep tests deterministic with explicit setup and cleanup. Use table-driven tests when they improve readability.
- Run relevant unit and integration tests; use race detection when concurrency is involved.

### API protocols and compatibility
- Choose API protocols per project based on interoperability, performance, evolution, tooling, and operations.
- Follow established protocol conventions instead of inventing custom ones; impose no universal REST, gRPC, or GraphQL choice.
- Once APIs have consumers, define compatibility expectations explicitly and avoid accidental breaking changes.

## Task-specific guidance

- **New Go project or environment setup:** Read [references/new-project.md](references/new-project.md) before creating files, selecting tooling, or defining a development flake. Do not load it for routine code changes or reviews.
- **Existing project changes:** Inspect current patterns first; apply the principles above incrementally. Use ordinary Go tools to validate the relevant changes and report any checks not run.

## Boundaries

- Avoid replacing Go modules with Nix-managed module imports or making `nix build` the interactive edit-test loop.
- Avoid treating `nix run` as a universal task runner. Prefer direct Go commands inside the shell; add flake apps for meaningful repeatable entry points only.
- Keep workstation-global Helix, and Git configuration distinct from project-specific compiler and tooling.
- Keep third-party precompiled Linux binaries in mind: executables expecting FHS interpreter paths may fail on NixOS. Prefer native nixpkgs packages or build from source.
- When upstream code or the user's explicit requirements differ from these defaults, preserve compatibility and explain the tradeoff rather than imposing conventions blindly.
