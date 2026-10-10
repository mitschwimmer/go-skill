# New Go project workflow

Use this procedure when creating a new Archaic Go repository or adding its development environment to a new project. For work on an established repository, inspect and preserve its existing conventions instead of automatically scaffolding new files.

## Setup checklist

1. Inspect the existing repository and its conventions before modifying it. Keep changes focused; do not introduce frameworks, wrappers, or additional tools without a concrete requirement.
2. Use standard `go.mod` and `go.sum` for module dependencies. Use `go fmt`, `go test ./...`, `go vet ./...`, and ordinary `go build` during development.
3. Manage the project toolchain with a flake pinned by `flake.lock`. Prefer a simple two-variable `let` block: `system` and `pkgs`. Do not create abstractions merely to shorten two or three repeated lines.
4. Activate the shell explicitly with `nix develop -c fish` or execute one command with `nix develop -c ...`. Do not add direnv, nix-direnv, `.envrc`, `just`, or Makefiles by default.
5. Assume Fish for interactive shell examples and Helix (`hx`) for editing. Put `gopls` in the development shell so Helix launched there can discover it on `PATH`; use `hx --health go` to diagnose integration. Avoid hard-coded store paths or project editor configuration unless necessary.
6. Keep encrypted secrets in SOPS-managed files and private age identities outside Git. Provide `sops` and `age` in the development shell but never decrypt inside Nix evaluation, build derivations, or `shellHook`. Prefer process-scoped `sops exec-env` or `sops exec-file` where the application can consume them. Never print or commit decrypted values.
7. Introduce `packages` using `buildGoModule` for reproducible release artifacts, `checks` for sandboxed CI validation, and `apps` only when genuinely useful as named commands. `nix flake check` does not automatically run Go tests without explicit checks.
8. Test the changes with the smallest relevant Go commands, then broader checks where available. State any tests that could not be run.

## Minimal development flake

Adapt package choices to project requirements and the target system. For cross-platform projects, extend the system handling only when needed.

```nix
{
  description = "Go development";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [
          go
          gopls
          delve
          golangci-lint
          sops
          age
        ];

        GOTOOLCHAIN = "local";
      };
    };
}
```

Commit both `flake.nix` and `flake.lock`. Pin a suitable stable nixpkgs revision when a project values stability over newer tooling. `GOTOOLCHAIN = "local"` deliberately prevents Go from downloading an alternate compiler; match the pinned compiler to the `go` directive in `go.mod`. Add `gcc`, `pkg-config`, and native libraries for projects that actually need CGO.

## Day-to-day commands (Fish)

```fish
nix develop -c fish
hx .
go test ./...
go test -race ./...
go vet ./...
```

Or without entering a persistent shell:

```fish
nix develop -c go test ./...
nix develop -c hx .
```

Go's default `GOCACHE` and `GOMODCACHE` should remain outside the Nix store and shared between projects; don't override them without a specific requirement.

For secrets, decrypt only at runtime:

```fish
sops exec-env secrets/development.enc.yaml 'go run ./cmd/my-service'
```

Use file-based secrets where more suitable. Environment variables and temporary files still require normal process/file permission safeguards.

