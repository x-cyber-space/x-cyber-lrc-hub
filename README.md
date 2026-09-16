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
- **智能加权契合度打分**：基于歌曲名、歌手名与歌曲时长的多维加权打分模型（45% 歌名 + 30% 歌手 + 25% 时长），保证歌词精准度。
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
                        │           智能多维加权契合度打分引擎         │
                        │    Title(45%) + Artist(30%) + Dur(25%)    │
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

系统要求：`Go 1.22+`（零 CGO/GCC 工具链依赖）

```bash
# 克隆仓库
git clone https://github.com/x-cyber-space/x-cyber-lrc-hub.git
cd x-cyber-lrc-hub

# 编译为单一静态二进制程序
go build -o x-cyber-lrc-hub ./cmd/server
```

### 2. 启动服务

```bash
# 默认端口 3300，缓存数据库路径 ./data/lyrics.db
./x-cyber-lrc-hub

# 自定义端口与缓存路径
./x-cyber-lrc-hub -port 3300 -cache ./data/lyrics.db -log-level info
```

命令行参数支持：
- `-port` (int): 服务监听端口，默认 `3300`。
- `-cache` (string): SQLite 缓存文件存储路径，默认 `./data/lyrics.db`。
- `-log-level` (string): 日志输出等级，默认 `info`。

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
  "trackName": "晴天",
  "artistName": "周杰伦",
  "albumName": "叶惠美",
  "duration": 269,
  "instrumental": false,
  "plainLyrics": "晴天 - 周杰伦 (Jay Chou)\n词：周杰伦\n曲：周杰伦...",
  "syncedLyrics": "[00:00.00] 晴天 - 周杰伦 (Jay Chou)\n[00:02.25]词：周杰伦\n[00:04.50]曲：周杰伦..."
}
```

未命中时返回 `404 Not Found`：
```json
{
  "error": "Lyrics not found"
}
```

### 2. 模糊候选搜索

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
    "trackName": "青花瓷",
    "artistName": "周杰伦",
    "albumName": "我很忙",
    "duration": 239,
    "instrumental": false,
    "plainLyrics": "素胚勾勒出青花笔锋浓转淡...",
    "syncedLyrics": "[00:00.00] 青花瓷 - 周杰伦..."
  }
]
```

---

## 契合度打分算法

系统对各源返回的候选歌曲进行智能打分（满分 100 分）：
1. **歌名契合度 (最高 45 分)**：文本归一化（去小写、去 `(Live)`/`[Remix]` 等各类中英文修饰词），完全一致得 45 分，子串得 38 分，其余按 Bigram Jaccard 相似度折算。
2. **歌手契合度 (最高 30 分)**：未知歌手得基础分 20 分，完全一致得 30 分，子串 28 分，相似度折算。
3. **时长契合度 (最高 25 分)**：误差 $<1.0\text{s}$ 得 25 分，$<2.0\text{s}$ 得 24 分，$<3.0\text{s}$ 得 22 分，$<5.0\text{s}$ 得 15 分，$\ge 5.0\text{s}$ 得 0 分。

---

## 免责声明 (Disclaimer)

1. 本项目（`x-cyber-lrc-hub`）为一个遵循 AGPL-3.0 协议的开源技术研究与学习项目，旨在为局域网媒体服务器及个人音乐播放器提供兼容 LRCLIB 标准协议的本地中转与格式适配功能。
2. 本项目不提供任何在线服务器，不存储任何非个人授权音频或歌词版权内容，不进行任何商业牟利行为。
3. 歌词数据版权归属于原词曲权利人或原始版权方所有。本软件所检索的数据仅供个人学习、研究与欣赏，严禁用于任何商业营利活动。
4. 使用者在部署与运行本程序时，应自行遵守所在地区的著作权法律法规及第三方网络平台的服务协议。开发者对使用者因不正当使用所引发的任何直接或间接纠纷不承担任何法律责任。

---

## 开源协议

本项目采用 [GNU Affero General Public License v3.0 (AGPL-3.0)](./LICENSE) 协议开源。