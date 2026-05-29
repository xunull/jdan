# jdan

Go 编写的常用小工具集合（单二进制）。

## 构建

```bash
# 若上级目录存在 go.work 导致构建报错，可临时：
# Linux/macOS: GOWORK=off go build -o jdan ./cmd/jdan
# Windows PowerShell:
$env:GOWORK="off"; go build -o jdan.exe ./cmd/jdan
```

## 命令

### `jdan file bak`

将**普通文件**复制到**同目录**下的备份文件，命名规则：

- 无 `--desc`（或 trim 后为空）：`{原完整文件名}.bak.{YYYYMMDD-HHMMSS}`
- 有 `--desc`：`{原完整文件名}.bak.{YYYYMMDD-HHMMSS}-{描述}`  
  描述：仅允许 **英文字母、ASCII 数字、汉字、ASCII 空格**；空格会变为 `_`。其它字符（标点、制表符等）会 **拒绝执行** 并打日志说明。
- 若目标备份路径已存在（同一时间戳）：**不复制**，报错提示「已存在相同时间戳的备份」。

```bash
jdan file bak ./report.pdf
jdan file bak ./report.pdf --desc "before edit"
```

### `jdan http timing`

测量 HTTP 请求各阶段耗时：DNS 查询、TCP 连接、TLS 握手、服务端处理、内容传输、总耗时，以及 HTTP 状态码。

```bash
jdan http timing https://github.com
jdan http timing https://github.com -n 3        # 请求 3 次，输出每次结果与平均值
jdan http timing https://github.com --json       # JSON 格式输出
jdan http timing https://github.com -n 3 --json  # 3 次 + JSON
jdan http timing https://example.com -k          # 跳过 TLS 证书验证
```

| 参数 | 说明 |
|------|------|
| `-n` | 请求次数（默认 1；大于 1 时追加平均值） |
| `--json` | 以 JSON 格式输出（Duration 以毫秒浮点数表示） |
| `-k` / `--insecure` | 跳过 TLS 证书验证 |

### `jdan pubip4` / `jdan pubip6`

查询本机当前出口的公网 IP 地址。

```bash
jdan pubip4                   # 输出公网 IPv4 地址（默认使用 ipify）
jdan pubip6                   # 输出公网 IPv6 地址（默认使用 ipify）
jdan pubip4 -p ipip           # 使用 ipip.net 查询 IPv4
jdan pubip6 -p ipip           # 使用 ipip.net 查询 IPv6
```

| 参数 | 说明 |
|------|------|
| `-p` / `--provider` | IP 查询服务：`ipify`（默认）或 `ipip` |

内部自动重试至多 3 次，全部失败后输出提示信息并以非零退出码退出。

### `jdan macgpu`

实时监控 Apple Silicon Mac 的 GPU 使用率、功耗、频率和散热压力等级。
以 htop/glances 风格的 TUI 界面展示：顶部带颜色的 ASCII 柱状图 + 底部详情表格。

> **要求：** 仅支持 Apple Silicon（arm64）Mac，需要 `sudo` 权限运行。

```bash
sudo jdan macgpu                # 默认每 2 秒采样一次
sudo jdan macgpu -i 1000        # 每 1 秒采样一次（最小 500ms）
```

| 参数 | 说明 |
|------|------|
| `-i` / `--interval` | 采样间隔（ms，默认 2000，最小 500） |

按 `q` 退出 TUI 界面。

### `jdan tree2`

按当前终端宽度多列显示两层目录结构，默认只显示目录。适合在宽终端中快速扫视项目结构，减少 `tree -L 2` 的纵向滚动。

```bash
jdan tree2                         # 查看当前目录，两层，自动推断列数
jdan tree2 ./internal --width 120   # 指定宽度，便于脚本或测试复现
jdan tree2 --cols 1                 # 强制单列输出
jdan tree2 --files                  # 包含文件
jdan tree2 --all                    # 包含隐藏文件和目录
jdan tree2 --limit 0                # 不限制每个一级目录显示的子项数量
```

| 参数 | 说明 |
|------|------|
| `--cols` | 指定输出列数（默认根据终端宽度自动推断） |
| `--width` | 指定终端宽度（默认自动检测，失败时使用 80） |
| `--files` | 包含文件（默认只显示目录） |
| `--all` | 包含隐藏文件和目录 |
| `--limit` | 每个一级目录最多显示的子项数量，默认 50；`0` 表示不限制 |

### `jdan unix-time`

将 Unix 时间戳（秒或毫秒）转换为本地时区可读时间。

```bash
jdan unix-time 1711843200000
echo 1711843200 | jdan unix-time
```

| 规则 | 说明 |
|------|------|
| 输入长度 10 | 按秒级时间戳解析 |
| 输入长度 13 | 按毫秒级时间戳解析 |
| 输出时区 | 本机本地时区 |

### 全局

### `jdan obsidian install-claudian`

从 GitHub 最新 Release 下载 [Claudian](https://github.com/YishenTu/claudian) 插件文件，并安装到指定 Obsidian Vault。

```bash
jdan obsidian install-claudian ./my-vault       # 安装到指定 vault 目录
jdan obsidian install-claudian                  # 安装到当前目录
jdan obsidian install-claudian ~/Documents/vault --force  # 覆盖已安装版本
```

| 参数 | 说明 |
|------|------|
| `vault-path` | Vault 目录路径（可选，默认当前目录） |
| `--force` / `-f` | 若插件已安装则强制覆盖 |

安装成功后会在 `{vault}/.obsidian/plugins/claudian/` 下创建 `main.js`、`manifest.json`、`styles.css`，之后在 Obsidian 的 Settings → Community plugins 中启用即可。

### 全局

## 开发

```bash
go test ./...
```
