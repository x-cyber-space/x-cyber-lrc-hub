# 更新日志

本文件记录项目的所有重要变更。

格式遵循 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/spec/v2.0.0.html)。

## [未发布]

### 新增

- 响应中增加 `lyricsfile` 字段，与 `lrclib.net` 在 `plainLyrics`、`syncedLyrics` 之外提供的那个字段对齐。
- `GET /api/get/{id}`：按 ID 获取缓存记录。
- `-cache-ttl`：缓存有效期（默认 30 天，`0` 表示永不过期）。
- 服务端匹配策略文档，以及 `docs/SPEC.md` 架构规范。

### 变更

- **`/api/get` 不再用加权总分决定是否接受候选。** 改为对齐 `lrclib.net` 的语义：对请求提供的字段做归一化精确匹配，加 ±3 秒时长窗口并取最近的记录；之后还有一级受控放宽（多歌手署名、±5 秒、平台标题后缀）。原来的 `>= 40` 分阈值会让满分的歌名把完全不符的歌手拖过线。
- `/api/search` 的加权分只用于**排序**，并保留 40 分下限，刻意低于客户端侧的 60 分线。
- `-log-level` 现在真的生效了；之前它被解析、打印，然后被忽略。
- 错误响应、响应字段集与 JSON 空值语义现在与 `lrclib.net` 逐字节一致。`plainLyrics` / `syncedLyrics` / `lyricsfile` 输出 `null` 而不是 `""`。
- `album_name` 参与匹配，不再被丢弃。它只在第 1 级（精确匹配）强制，因为平台的专辑名与用户自己的标签经常对不上。
- 缓存命中会用与冷启动**相同的谓词**重新校验，因此模糊搜索的结果不再可能变成一个权威的 `/api/get` 答案。
- 缓存键加入 2 秒时长分桶，同一首歌的不同版本不再互相覆盖。
- 要求 Go 1.25+；此前 go.mod 声明 `go 1.26.5`，导致更早的工具链直接拒绝构建。

### 修复

- 歌名相同、歌手不同时，可能返回另一位歌手的歌词。
- `duration` 缺失曾等于白送 20 分，把不匹配的候选推过了阈值。
- `/api/search` 会返回重复的行 ID，且会返回远低于其标称下限的候选。
- `name` 字段只在缓存命中时存在，导致响应结构随缓存状态变化。
- 平台不可达时，提供方冒烟测试会静默通过。

### 工程

- CI：gofmt、build、vet、`go test -short -race`、golangci-lint，以及 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0` 交叉编译。
- CI 还会**构建并运行**容器镜像：`push: false`（PR 不能发布任何东西），但会真正启动容器并断言 health 端点、`VERSION` 构建参数是否经 ldflags 到达二进制、以及非特权运行用户能否写入缓存卷。
- `Dockerfile`（多阶段、静态、非 root）、`Makefile`、`.golangci.yml`、`.editorconfig`、`.gitattributes`。
- CodeQL、依赖审查、OpenSSF Scorecard 工作流；Dependabot 覆盖 Go 模块、GitHub Actions 与 Docker 基础镜像。第三方 action 一律按 commit SHA 固定。
- 发布流程产出 GoReleaser 归档，并把多架构镜像推送到 GHCR（`ghcr.io/x-cyber-space/x-cyber-lrc-hub`）。
- SQLite 启用 WAL 与 `synchronous=NORMAL`；服务端设置 `ReadHeaderTimeout`；panic 会记录调用栈。
- 文档全部改为中文，新增 `docs/AUTOMATION.md` 记录工作流与那些无法版本化的仓库设置。

[未发布]: https://github.com/x-cyber-space/x-cyber-lrc-hub/commits/main
