# X-Cyber-LRC-Hub

```
__  __   ____ y b e r   _      ____    ____     _   _       _     
\ \/ /  / ___|         | |    |  _ \  / ___|   | | | | _   | |__  
 \  /  | |      _____  | |    | |_) || |       | |_| || |  | '_ \ 
 /  \  | |___  |_____| | |___ |  _ < | |___    |  _  || |_ | |_) |
/_/\_\  \____|         |_____||_| \_\ \____|   |_| |_| \___||_.__/ 
             :: X-Cyber Lyrics Hub :: [v1.0.0]
             :: LRCLIB Compatible API Server ::
```

`x-cyber-lrc-hub` 是一个专为音乐爱好者、自建 NAS 玩家及极客开发者打造的**轻量级、自建式（Self-Hosted）、兼容 LRCLIB 标准协议的歌词聚合与中转服务**。

- **去中心化与隐私安全**：服务运行于本地主机、NAS 或软路由，所有检索流量由自身网络发起，无中心化服务器监控与封禁风险。
- **协议标准化**：对外提供 100% 兼容 [LRCLIB](https://lrclib.net/) 的公开标准 REST API，支持 ZZMusicPlayer、Symfonium 等现代播放器无缝接入。
- **并发多源聚合**：网易云音乐、QQ音乐、酷狗音乐、酷我音乐 4 大主流平台毫秒级并发检索。
- **两级匹配策略**：`/api/get` 采用与 LRCLIB 同构的**确定性硬过滤**（归一化精确匹配 + 时长窗口），`/api/search` 才使用加权打分做模糊排序。服务器不做猜测，判断留给播放器。
- **零 CGO 本地持久化**：基于纯 Go 实现的 SQLite（`modernc.org/sqlite`），编译为单静态二进制，开箱即用。

---

## 总体架构

```
                               ┌───────────────────────────┐
                               │  ZZMusicPlayer / 第三方播放器 │
                               └─────────────┬─────────────┘
                                             │ HTTP (LRCLIB 标准协议: /api/get, /api/search)
                                             ▼
                        ┌───────────────────────────────────────────┐
                        │             x-cyber-lrc-hub               │
                        │           (本地 Go 单二进制服务)            │
                        ├─────────────────────┬─────────────────────┤
                        │  LRCLIB 协议适配层   │  本地持久化缓存层     │
                        │  (Router & DTO)     │  (SQLite 纯 Go 实现) │
                        ├─────────────────────┴─────────────────────┤
                        │      两级匹配引擎（零模型依赖，确定性）        │
                        │  /api/get   : 归一化精确 + 时长 ±3s 窗口      │
                        │  /api/search: 加权排序 45/30/25 + 下限过滤    │
                        ├───────────────────────────────────────────┤
                        │           并发多源歌词爬取调度器            │
                        └───────┬─────────┬─────────┬─────────┬─────┘
                                │         │         │         │
                                ▼         ▼         ▼         ▼
                             [网易云]   [QQ音乐]   [酷狗]    [酷我]
```

---

## 快速上手

### 1. 源码编译

系统要求：`Go 1.25+`（零 CGO/GCC 工具链依赖）。

> 这个下限来自依赖本身：`modernc.org/sqlite v1.59.0` 及其依赖要求 `go 1.25.0`。

```bash
# 克隆仓库
git clone https://github.com/x-cyber-space/x-cyber-lrc-hub.git
cd x-cyber-lrc-hub

# 编译到 bin/（推荐，Makefile 会自动注入版本号）
make build

# 或者手动编译
go build -o bin/x-cyber-lrc-hub ./cmd/server
```

`make build` 等价于：

```bash
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=$(git describe --tags --always)" \
  -o bin/x-cyber-lrc-hub ./cmd/server
```

版本号只在 `cmd/server/main.go` 里声明一次，通过 `-ldflags -X` 注入，不再硬编码在多处。

### 2. 启动服务

```bash
# 默认端口 3300，缓存数据库路径 ./data/lyrics.db，缓存有效期 30 天
./bin/x-cyber-lrc-hub

# 自定义端口、缓存路径与缓存有效期
./bin/x-cyber-lrc-hub -port 3300 -cache ./data/lyrics.db -cache-ttl 720h -log-level info
```

命令行参数支持：

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `-port` | int | `3300` | 服务监听端口 |
| `-cache` | string | `./data/lyrics.db` | SQLite 缓存文件路径 |
| `-cache-ttl` | duration | `720h`（30 天） | 缓存有效期；`0` 表示永不过期 |
| `-log-level` | string | `info` | 日志等级：`debug` / `info` / `warn` / `error` |

### 3. Docker

```bash
make docker
docker run -d --name lrc-hub -p 3300:3300 -v lrc-data:/data x-cyber-lrc-hub:latest
```

镜像基于 `alpine`，以非 root 用户运行，缓存持久化在 `/data` 卷上。

### 4. 预编译二进制

每个 `v*` 标签都会由 GitHub Actions 构建并发布 linux / macOS / Windows（amd64、arm64）的静态二进制包，附带 `checksums.txt`。见 [Releases](https://github.com/x-cyber-space/x-cyber-lrc-hub/releases)。

---

## 项目结构

```
cmd/server/          程序入口（flag 解析、启动、优雅退出）
internal/
  api/               LRCLIB 协议层：路由、中间件、两个端点
  matching/          匹配闸门：归一化、相似度、歌手集合、身份、MatchTrack
  scoring/           加权打分：只用于 /api/search 的排序
  provider/          四个平台的适配器 + 并发调度器
  cache/             SQLite 缓存（键、TTL、复核谓词）
  lyrics/            LRC 解析/生成、lyricsfile 渲染
  model/             LRCLIB 响应、候选、查询的数据结构
  config/            flag → Config
  logging/           slog 分级日志
docs/SPEC.md         架构与开发规范
```

两条关键的分层约定：

- **`matching` 与 `scoring` 是分开的。** `matching` 回答"这是不是同一首歌"（`/api/get` 的闸门），`scoring` 回答"哪个候选更好"（`/api/search` 的排序）。把两者混在一起，正是"满分歌名掩盖错误歌手"这类 bug 的来源。
- **平台的线上格式归平台自己。** 例如酷我把行时间戳表示成字符串秒数，`kuwoLrcItem` 就定义在 `provider/kuwo.go`；`lyrics` 只接受中性的 `TimedLine`。项目里没有 `util` / `common` / `helpers` 这类包。

---

## 接口文档 (LRCLIB 兼容)

### 1. 单曲精准匹配获取

`GET /api/get`

**Query 参数：**
- `track_name` (string, 必需): 歌曲名称
- `artist_name` (string, 必需): 歌手名称
- `album_name` (string, 可选): 专辑名称
- `duration` (number, 可选): 歌曲时长（秒）

**响应示例 (200 OK)：**
```json
{
  "id": 1,
  "name": "晴天",
  "trackName": "晴天",
  "artistName": "周杰伦",
  "albumName": "叶惠美",
  "duration": 269,
  "instrumental": false,
  "plainLyrics": "晴天 - 周杰伦 (Jay Chou)\n词：周杰伦\n曲：周杰伦...",
  "syncedLyrics": "[00:00.00] 晴天 - 周杰伦 (Jay Chou)\n[00:02.25]词：周杰伦\n[00:04.50]曲：周杰伦...",
  "lyricsfile": "version: '1.0'\nmetadata:\n  title: 晴天\n..."
}
```

未命中时返回 `404 Not Found`：
```json
{"message":"Failed to find specified track","name":"TrackNotFound","statusCode":404}
```

缺少必需参数时返回 `400 Bad Request`，响应体为 `text/plain`：
```
Failed to deserialize query string: missing field `track_name`
```

### 2. 按 ID 获取

`GET /api/get/{id}`

返回对应 ID 的记录；不存在时返回与上面相同的 404 响应体。

### 3. 模糊候选搜索

`GET /api/search`

**Query 参数：**
- `q` (string, 可选): 综合搜索关键词
- `track_name` (string, 可选): 歌曲名过滤
- `artist_name` (string, 可选): 歌手过滤

**响应示例 (200 OK)：**
```json
[
  {
    "id": 1,
    "name": "青花瓷",
    "trackName": "青花瓷",
    "artistName": "周杰伦",
    "albumName": "我很忙",
    "duration": 239,
    "instrumental": false,
    "plainLyrics": "素胚勾勒出青花笔锋浓转淡...",
    "syncedLyrics": "[00:00.00] 青花瓷 - 周杰伦...",
    "lyricsfile": "version: '1.0'\nmetadata:\n  title: 青花瓷\n..."
  }
]
```

---

`q` / `track_name` / `artist_name` 全为空，或检索无结果时，均返回 `[]` 与 `200`（而不是 404）。

---

## 匹配策略

项目刻意把"精确获取"与"模糊搜索"拆成两套策略，对应 LRCLIB 的两个端点。依据是**两类错误的代价并不对称**：

- **假 404 可恢复**：客户端可以退回到 `/api/search`，由播放器自身的模型和用户来挑。
- **假命中不可恢复**：一旦返回了错歌的歌词，播放器就直接渲染，用户无从分辨。

所以：**服务器只做确定性匹配，智能判断留给下游。**

### `GET /api/get`：确定性硬过滤（零模型依赖）

对齐 `lrclib.net` 的实测行为：

1. **归一化**：转小写、剥离括号修饰词、折叠空白。因此 `七里香 (Live)` 与 `七里香` 属于同一身份，`Wildest Dreams (Taylor's Version)` 也能命中原版记录 —— 这与 LRCLIB 一致。
2. **硬条件**：歌名归一化后必须相等；歌手与专辑**仅在请求提供时**才要求相等，未提供则不构成约束。
3. **时长窗口**：请求带 `duration` 时，取 ±3 秒内**最接近**的记录。（实测：220s 的记录对 220/221/222s 的请求均命中；225s 会改投更近的 226s 记录；230s 则未命中。）
4. **受控放宽**（第二级，仍然零模型依赖）：
   - 多歌手集合匹配 —— 请求 `周杰伦` 可匹配平台署名 `周杰伦 / 费玉清`
   - 时长窗口放宽到 ±5 秒
   - 平台给标题加后缀 —— `晴天` 可匹配 `晴天 现场版`（要求候选更长，且请求至少 2 个字符，避免 `红` 吞掉 `红玫瑰`）
   - **不再要求专辑相等** —— 专辑只在第 1 级做硬过滤。LRCLIB 的专辑列是它自己规整过的数据，而我们抓取的平台专辑名与用户文件标签经常对不上（版本后缀、简繁差异、再版合集），把它一路硬性执行会把确实存在的歌变成 404；而且专辑对"歌词内容"几乎没有信息量。
5. 仍无命中 → `404`。**绝不退而求其次返回"分数最高"的候选。**

> 第 1 级的语义与 `lrclib.net` 逐条对齐；第 2 级是 LRCLIB 本身没有的兜底层，只用于挽救脏元数据，不影响协议兼容性。

> **为什么不用加权总分当门槛**：加法模型会让满分的歌名（45）掩盖完全不符的歌手（0）。请求 `童话 / 光良` 时，候选 `童话 / 王菲` 仍能拿到 45 分并越过 40 分门槛，从而返回另一位歌手的歌词。**歌手不符是硬矛盾，不是扣分项。**

### `GET /api/search`：模糊加权排序

这里才使用加权打分做**排序**（满分 100 分）：

1. **歌名契合度 (最高 45 分)**：文本归一化（转小写、去 `(Live)`/`[Remix]` 等各类中英文修饰词），完全一致得 45 分，子串得 38 分，其余按 Bigram Jaccard 相似度折算。
2. **歌手契合度 (最高 30 分)**：未知歌手给中立分 20 分，完全一致得 30 分，子串 28 分，其余按相似度折算。
3. **时长契合度 (最高 25 分)**：误差 $<1.0\text{s}$ 得 25 分，$<2.0\text{s}$ 得 24 分，$<3.0\text{s}$ 得 22 分，$<5.0\text{s}$ 得 15 分，$\ge 5.0\text{s}$ 得 0 分。

排序后保留 40 分的下限。这个下限刻意**低于**客户端侧的 60 分线：服务器负责召回，判断留给下游 —— 拥有自身模型、并且有用户看着候选列表的播放器。

### 搜索结果不会污染精确获取

`/api/search` 的每条结果按**它自己的身份**写入缓存，而 `/api/get` 命中缓存时，会使用与冷启动**完全相同**的匹配谓词重新校验。因此一条模糊搜索的结果不可能"混成"一个权威的 `/api/get` 答案。

缓存键由「归一化歌名 + 归一化歌手 + 时长分桶（2 秒）」构成，且分桶宽度**有意窄于** ±3 秒的匹配窗口：这既保证缓存命中永远不会因分桶而被复核拒绝，也保证一首歌的不同版本不会互相覆盖。

---

## 缓存机制

缓存落在 **SQLite** 文件里（`-cache` 指定的路径，默认 `./data/lyrics.db`），表名 `lyrics`，一行代表"一首歌的某一个版本"。

### 什么时候会写入

只有两个地方会写库，**且都只在真正拿到歌词之后**：

| 触发点 | 写入用的键 |
|---|---|
| `/api/get` 冷启动抓到歌词 | **请求的**身份（请求里的 track / artist / duration） |
| `/api/search` 返回的每一条结果 | **结果自己的**身份（结果自身的 track / artist / duration） |

**不会写入的情况**：

- 未命中（`404`）**不做负缓存**。一次失败之后，下次请求仍会重新打 4 个平台。这是有意为之：上游限流或临时故障不应该被记成"这首歌没有歌词"。
- 抓不到歌词（且不是纯音乐）的候选会被丢弃，不写库。
- 4 个平台全部失败时返回 `503`，不写库。

### 什么时候会读出

| 路径 | 行为 |
|---|---|
| `/api/get` | 按请求身份查缓存 → 用**与冷启动完全相同的匹配谓词**复核 → 通过才返回 |
| `/api/get/{id}` | 按 ID 直读 |
| `/api/search` | **不读缓存**，每次都实时聚合（搜索路径只写不读） |

### 缓存键

```
sha256( 归一化歌名 \0 归一化歌手 \0 时长分桶 )[0:16]
时长分桶 = round(duration / 2)     // duration 未知时为 -1
```

时长参与分桶，是为了让同一首歌的**不同版本**（原版 / Live / 伴奏 / DJ 版）各自占一行、互不覆盖。分桶宽度 2 秒**有意窄于** ±3 秒的匹配窗口，这保证缓存命中不会因为分桶本身而被复核拒绝。

> **建议客户端务必带上 `duration`。** 不带时长时所有请求都落进 `-1` 桶，同一歌名 + 歌手的各个版本会共用一行。虽有匹配复核兜底、不会返回错歌，但版本之间会互相覆盖刷新。

### 过期与回收

`-cache-ttl` 控制有效期（默认 30 天，`0` 表示永不过期）：

- **读取时即判定过期** —— 过期行等同于未命中，不必等待清理任务；
- 后台每小时（且不低于每分钟）清理一次过期行以回收磁盘；
- 每次写入 / 更新都会刷新 `updated_at`。

没有这一层时，第一次请求抓到的歌词会被永久提供下去，即使上游后来修正了错别字或替换了时间轴。

### 并发与持久性

数据库连接数固定为 **1**，这是刻意的：SQLite 的写入本来就串行，而这里每次查询都是本地小表上的索引查找（微秒级），请求真正的开销在 4 次外部 HTTP 调用上 —— 加大连接池没有可测量的收益，却要求把 PRAGMA 逐连接重新应用（PRAGMA 是连接级的，不是库级的）。

同时启用了 `journal_mode=WAL` 与 `synchronous=NORMAL`，把默认回滚日志的「每次提交都 fsync」降为「每次 checkpoint 一次 fsync」，这对 NAS 上的机械盘 / SD 卡是有实际意义的。

---

## 与 LRCLIB 的兼容性

以下行为均已对 `lrclib.net` 实测逐条核对：

| 项目 | 行为 |
|---|---|
| 响应字段 | `id, name, trackName, artistName, albumName, duration, instrumental, plainLyrics, syncedLyrics, lyricsfile`，共 10 个，恒定存在 |
| 空歌词 | `plainLyrics` / `syncedLyrics` / `lyricsfile` 用 JSON `null`，而不是空字符串 |
| `404` | `{"message":"Failed to find specified track","name":"TrackNotFound","statusCode":404}` |
| `400` 缺参数 | `text/plain; charset=utf-8`，内容为 ``Failed to deserialize query string: missing field `track_name` `` |
| `405` | 空响应体，且不带 `Content-Type` |
| 未知路由 | 空响应体 `404` |
| `/api/get` 必填参数 | 仅 `track_name` 与 `artist_name`；`album_name` / `duration` 可选，提供时作为硬过滤条件（专辑见上文：第 1 级硬过滤，第 2 级放开） |
| `/api/search` 空查询 / 无结果 | `[]` 与 `200` |
| CORS | `Access-Control-Allow-Origin: *`，并 expose `retry-after` |

**已知的唯一差异**：`duration` 输出为 `302` 而非 `302.0`。JSON 中两者是同一个数字，任何符合标准的解析器都会得到相同结果；只有逐字节比对才会察觉。

> 上游平台被限流时，LRCLIB 会返回 `503` 与 `{"message":"The server is busy...","name":"ServerOverloaded",...}`。本项目在**所有**提供方都失败时返回同样的 503，而不是把它伪装成"没有这首歌"的 404。

---

## 开发

```bash
make help        # 列出所有目标
make check       # 一次跑完 CI 的全部检查：fmt-check + vet + test + lint
make fmt         # gofmt
make vet         # go vet
make test        # 离线测试（含 -race），不需要网络
make test-all    # 全部测试，含真实平台 smoke test（需要网络）
make lint        # golangci-lint
make build       # 编译到 bin/ 并注入版本号
```

`make test` 使用 `-short`，会跳过需要访问网易云 / QQ / 酷狗 / 酷我的 smoke test —— 那条测试依赖第三方平台可用性，放进 CI 会让构建变得不稳定。匹配、缓存、协议层的行为全部由离线测试覆盖（含 stub provider 驱动的 dispatcher 测试）。

`make lint` 会先找 PATH 里的 `golangci-lint`，找不到则回退到 `$(go env GOPATH)/bin/golangci-lint`，因此不把 `~/go/bin` 加进 PATH 也能用。需要 v2 版本：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

CI（`.github/workflows/ci.yml`）在每次 push / PR 上执行：`gofmt` 校验、`go build`、`go vet`、`go test -short -race`、`golangci-lint`，并在 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 上做 `CGO_ENABLED=0` 交叉编译验证 —— 因为"单一静态二进制"是这个项目的核心承诺，必须每个目标都能真的构建出来。

推送 `v*` 标签会触发 `.github/workflows/release.yml`：先跑 `make check-core`，再用 GoReleaser 构建各平台产物并创建 draft release，最后把 `linux/amd64` + `linux/arm64` 的多架构镜像推到 GHCR（`ghcr.io/x-cyber-space/x-cyber-lrc-hub`）。

其余自动化：`codeql.yml`（Go 安全与质量分析，push + 每周）、`dependency-review.yml`（PR 引入中危及以上依赖时直接失败）、`scorecard.yml`（OpenSSF Scorecard）、`dependabot.yml`（Go 模块 / Actions / Docker 基础镜像）。第三方 action 全部按 commit SHA 固定，由 Dependabot 连带版本注释一起更新。

> 仓库的 branch protection / ruleset、Dependabot security updates、secret scanning、private vulnerability reporting 等设置位于 GitHub 网页端，无法随代码版本化 —— 清单见 [CONTRIBUTING.md](./CONTRIBUTING.md#settings-that-are-not-files)。

其余文件：`CHANGELOG.md`、`CONTRIBUTING.md`、`SECURITY.md`、`docs/SPEC.md`。

发布配置（`.goreleaser.yml`）可以在本地校验：`make release-check`。注意用 docker 跑 `goreleaser release --snapshot` 会在 `dist/` 留下 **root 属主**的文件，所以本地一般只做校验，真正的打包交给 CI。

---

## 免责声明 (Disclaimer)

1. 本项目（`x-cyber-lrc-hub`）为一个遵循 AGPL-3.0 协议的开源技术研究与学习项目，旨在为局域网媒体服务器及个人音乐播放器提供兼容 LRCLIB 标准协议的本地中转与格式适配功能。
2. 本项目不提供任何在线服务器，不存储任何非个人授权音频或歌词版权内容，不进行任何商业牟利行为。
3. 歌词数据版权归属于原词曲权利人或原始版权方所有。本软件所检索的数据仅供个人学习、研究与欣赏，严禁用于任何商业营利活动。
4. 使用者在部署与运行本程序时，应自行遵守所在地区的著作权法律法规及第三方网络平台的服务协议。开发者对使用者因不正当使用所引发的任何直接或间接纠纷不承担任何法律责任。

---

## 开源协议

本项目采用 [GNU Affero General Public License v3.0 (AGPL-3.0)](./LICENSE) 协议开源。