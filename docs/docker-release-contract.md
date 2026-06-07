# Docker Release Contract

## Supported Paths

This repository explicitly supports two Docker build paths:

1. GoReleaser release path
   - Use the `goreleaser-release` target in `Dockerfile`.
   - The Docker build context must already contain a prebuilt `rancher-mcp-server` binary at its root.
   - The real entry points are the `dockers` section in `.goreleaser.yml` and the tag release workflow in `.github/workflows/release-docker.yaml`.

2. direct local build path
   - Use `docker build --target local .`, `docker build --target dev .`, or `docker build .` with explicit build args.
   - The binary is produced by the `builder` stage inside `Dockerfile`.
   - This path is intended for local development and manual verification. It does not depend on GoReleaser's staged binary context.
   - To produce version metadata comparable to the release path, pass these build args:
     - `VERSION`
     - `GIT_COMMIT`
     - `BUILD_DATE`
   - `docker build .` currently resolves to the `dev` target, and `dev` is now only an alias of `local`.

## Unsupported Paths

The following path is outside the supported contract:

- `docker build --target goreleaser-release .`
  - The repository root does not normally contain the prebuilt binary required by the release workflow.
  - If this target must be validated directly, prepare a staged binary context equivalent to the one GoReleaser provides.

## Consistency Rules

- The GoReleaser release path and the release workflow must both target `goreleaser-release`.
- The direct local build path must only depend on the `builder -> local/dev` chain.
- The direct local Docker build path calls `make build` from the `Dockerfile` builder stage so it reuses the version metadata injection contract defined by `pkg/core/version` and `make build`.

## Current Validation Flow

- `.github/workflows/release-docker.yaml`
  - First run `goreleaser build --clean --single-target` to produce the staged binary.
  - Then create a temporary Docker context from that staged binary and run `docker build --target goreleaser-release` as a release image preflight.
  - Finally run the real `goreleaser release --clean`.

- direct local build path
  - The `release-docker` workflow runs a smoke check with the default `docker build .` path and reuses the existing version output verification script.
