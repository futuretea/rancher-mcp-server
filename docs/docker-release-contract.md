# Docker Release Contract

## 支持矩阵

当前仓库明确支持两条 Docker 构建路径：

1. GoReleaser release path
   - 使用 `Dockerfile` 中的 `goreleaser-release` target。
   - 前提是 Docker build context 根目录已经包含预构建二进制 `rancher-mcp-server`。
   - 当前真实入口是 `.goreleaser.yml` 的 `dockers` 配置，以及 `.github/workflows/release-docker.yaml` 的 tag release workflow。

2. direct local build path
   - 使用 `docker build --target local .`、`docker build --target dev .`，或显式传入 build args 的 `docker build .`。
   - 二进制来源是 `Dockerfile` 内部的 `builder` stage。
   - 这条路径面向本地开发和手动验证，不依赖 GoReleaser 的 staged binary context。
   - 若需要与 release path 可比的版本元数据，必须传入以下 build args：
     - `VERSION`
     - `GIT_COMMIT`
     - `BUILD_DATE`
   - `docker build .` 当前默认落到 `dev` target；`dev` 现在只是 `local` 的别名。

## 不支持的路径

以下路径不属于仓库承诺支持的 contract：

- `docker build --target goreleaser-release .`
  - 原因：仓库根目录默认不包含 release workflow 所需的预构建二进制。
  - 如果需要验证该 target，必须先准备与 GoReleaser 等价的 staged binary context。

## 一致性约束

- GoReleaser release path 和 release workflow 都必须指向 `goreleaser-release` target。
- direct local build path 只应依赖 `builder -> local/dev` 这条链路。
- Docker 的 direct local build path 通过 `Dockerfile` builder stage 调用 `make build`，直接复用 `pkg/core/version` + `make build` 的版本元数据注入契约。

## 当前验证方式

- `.github/workflows/release-docker.yaml`
  - 先运行 `goreleaser build --clean --single-target` 生成 staged binary。
  - 再基于 staged binary 构造临时 Docker context，执行 `docker build --target goreleaser-release` 做 release image preflight。
  - 最后运行真正的 `goreleaser release --clean`。

- direct local build path
  - `release-docker` workflow 会执行默认 `docker build .` 的 smoke check，并复用现有版本输出校验脚本。
