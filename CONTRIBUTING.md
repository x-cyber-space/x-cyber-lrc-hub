# 贡献指南

感谢你有兴趣改进 `x-cyber-lrc-hub`。这份文档只讲实用的部分：怎么构建、要过哪些检查，以及几条**容易搞错的项目专属规矩**。

---

## 环境准备

```bash
git clone https://github.com/x-cyber-space/x-cyber-lrc-hub.git
cd x-cyber-lrc-hub

make help     # 列出所有目标
make build    # 产物在 bin/x-cyber-lrc-hub
make check    # 一次跑完 CI 的全部检查：fmt-check、vet、test、lint
```

依赖：`Go 1.25+`；建议再装 `golangci-lint` v2：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

`make lint` 会先找 PATH，找不到则回退到 `$(go env GOPATH)/bin`，所以不把 `~/go/bin` 加进 PATH 也能用。Docker 只在 `make docker` 和构建发布产物时才需要。

---

## 日常开发循环

`main` 有规则集保护，**`git push origin main` 会被直接拒绝**。所有改动都要走分支 + PR：

```bash
# 1. 开工前同步
git switch main
git pull

# 2. 建分支
git switch -c feat/narrow-search-query

# 3. 改代码，本地自测
make check

# 4. 提交
git commit -am "feat: ..."

# 5. 推送（新分支直接 push 即可，会自动建立跟踪关系）
git push

# 6. 开 PR
gh pr create --fill

# 7. 等检查并合并
gh pr checks --watch
gh pr merge --squash --auto

# 8. 回到 main
git switch main
git pull
git branch -d feat/narrow-search-query
```

`gh pr merge --squash --auto` 开启自动合并：9 项必需检查一绿就自动合并，你不必回来点。

几条容易踩的：

- **只允许 squash 合并**，所以分支里有多少个「改错了」的提交都无所谓，落到 `main` 上永远是一条干净提交。
- **分支合并后远端会自动删除**，本地那份要自己删（上面第 8 步）。一次性清理：
  ```bash
  git branch --merged main | grep -v '^\*\|main' | xargs -r git branch -d
  ```
- **不要在分支上 `git merge main`**；要同步就用 rebase：`git pull --rebase origin main`。
- **同一 PR 连续推送时，上一次还在跑的 CI 会被取消**（`concurrency` 配置），这是有意的，不用心疼 CI 时间。
- **Dependabot 的 PR 不需要你建分支**，直接合即可：`gh pr merge <编号> --squash`。

### 推荐的 git 配置

```bash
git config --global fetch.prune true           # 自动清理已删除远端分支的跟踪引用
git config --global push.autoSetupRemote true  # 新分支直接 git push，不必再写 -u origin <名>
git config pull.ff only                        # 仅本仓库：分叉时明确报错，而不是静默造一个 merge commit
```

前两条全局生效，但只会把「原本就报错的情况」变成「成功」——任何原本能用的 `git push` / `git fetch` 行为不变。
第三条会改变**所有仓库**的 `git pull` 行为（分叉时从静默合并变成报错），所以建议只在本仓库设置。

---

## 测试

```bash
make test       # 离线，带 -race。CI 跑的就是这个
make test-all   # 额外包含真实平台冒烟测试，需要外网
```

`internal/provider` 里那条冒烟测试会访问四个真实的音乐平台，所以它在 `-short` 下被跳过，**不能**依赖它在 CI 里把关。

所有决定行为的逻辑——匹配、缓存、协议格式、调度器——都有离线覆盖，由 stub provider 驱动。**改了行为就要补离线测试。**

---

## 本项目的几条硬规矩

下面每一条都是真实 bug 留下的教训。

### `/api/get` 绝不做猜测

`/api/get` 走的是硬过滤，从来不是加权评分。一旦歌词被播放器渲染出来，**错的和对的看起来完全一样**；而 `404` 是可恢复的——客户端可以退回到 `/api/search`，用自身的排序模型加人工判断来选。所以如果你冒出「干脆返回分数最高的那个」的念头，不要。

打分（`internal/scoring`）是**排序**函数，只服务于 `/api/search`；匹配（`internal/matching`）是**闸门**，只服务于 `/api/get`。歌名满分一项就有 45 分，这正是它**绝不能**把完全不符的歌手拖过阈值的原因。

### 缓存不能回答冷路径会拒绝的查询

每一次缓存命中都会用**与冷启动完全相同的谓词**重新校验（`cache.Matches`）。新增任何读缓存的代码路径，都必须跑这个谓词。否则一条模糊搜索的结果，只要躺在数据库里就能变成权威的 `/api/get` 答案。

身份（identity）只在 `matching.IdentityKey` 里定义**一次**，缓存键和结果去重共用它。不要在第二个地方重新计算。

### 线上格式兼容性靠实测，不靠假设

README 和代码注释里关于 `lrclib.net` 行为的描述，都是**对着线上服务实测**得出的。改动响应结构、错误体或匹配规则之前，先探测真实端点，并把这个事实写进提交信息。`docs/SPEC.md` 讲架构，README 的兼容性表格是对外契约。

### 报错不等于「没有歌词」

如果所有提供方都失败，那是 `503`，不是 `404`。把上游故障伪装成「没有这首歌」，会让坏掉的服务看起来像缺失的资源。

---

## 提交与 Pull Request

- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)：`feat:`、`fix:`、`docs:`、`refactor:`、`test:`、`chore:`。
- **正文写清「为什么」**。diff 已经说明了「改了什么」。
- `go.mod`、`go.sum` 和 `CHANGELOG.md` 要跟代码改动保持一致。
- 开 PR 之前跑一遍 `make check`，CI 跑的是同一套。

`main` 有规则集保护：必须走 PR、9 项必需检查全绿、线性历史、只允许 squash 合并。详见 [`docs/AUTOMATION.md`](docs/AUTOMATION.md)。

---

## 代码风格

- `gofmt` 强制。`golangci-lint` 的检查集**刻意保持很小**——目的是抓错误，不是堆积个人偏好。
- **包名要描述它所属的领域。** 项目里没有 `util`、`common`、`helpers` 这类包，也不应该新增。
- 优先写「解释某个不显然的约束」的注释，而不是「复述代码」的注释。

---

## 仓库自动化

工作流清单、Dependabot 配置，以及那些**只能网页端设置、无法版本化**的项目（Actions 白名单、分支规则集、Code security 开关），全部记录在 [`docs/AUTOMATION.md`](docs/AUTOMATION.md)。
