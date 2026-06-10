# jdan

Go 编写的常用小工具集合（单二进制）。

## 构建

```bash
# 若上级目录存在 go.work 导致构建报错，可临时：
# Linux/macOS: GOWORK=off go build -o jdan .
# Windows PowerShell:
$env:GOWORK="off"; go build -o jdan.exe .
```

也可以直接安装：

```bash
go install github.com/xunull/jdan@latest
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

### `jdan dns lookup`

并发查询域名的多个 DNS 记录类型，一发命令拿到 A / AAAA / MX / TXT / CNAME / NS 的完整诊断信息。相比 `dig` 默认仅查 A 记录，`jdan dns lookup` 默认一次查 6 个最常用 type，并发送出，总耗时 ≈ 最慢单 type。

```bash
jdan dns lookup example.com                       # 默认查询 6 个 type
jdan dns lookup example.com -t A                  # 仅查 A 记录
jdan dns lookup example.com -t A,MX,TXT           # 指定多个 type，逗号分隔
jdan dns lookup example.com -t all                # 查询 9 个 type（含 SOA / CAA / SRV）
jdan dns lookup example.com -s 8.8.8.8            # 指定 DNS server（绕过本地 resolver）
jdan dns lookup example.com --json                # JSON 输出，便于脚本消费
jdan dns lookup example.com --short -t A          # 仅输出值，dig +short 风格
jdan dns lookup example.com --verbose             # 顶部追加 query time、rcode 列
jdan dns lookup example.com --strict              # 任一 type 失败即 exit 1
jdan dns lookup example.com --timeout 2s          # 调整整体查询超时（默认 5s）
```

| 参数 | 说明 |
|------|------|
| `-t` / `--type` | 查询的 record type，逗号分隔；`all` 表示 9 个；空表示默认 6 个（A / AAAA / MX / TXT / CNAME / NS） |
| `-s` / `--server` | DNS server（如 `8.8.8.8` 或 `8.8.8.8:53`）；空表示从 `/etc/resolv.conf` 读取系统配置 |
| `-j` / `--json` | 以 JSON 格式输出（含 TTL / rcode / query_time_ms 等完整 metadata） |
| `--short` | 仅输出值，每行一条（适合脚本：`IP=$(jdan dns lookup example.com --short -t A)`） |
| `-v` / `--verbose` | 顶部追加 query time，rcode 单独列 |
| `--strict` | 任一 type 失败（NXDOMAIN / SERVFAIL / TIMEOUT）即 `exit 1`；默认宽容（任一成功即 `exit 0`） |
| `--timeout` | 整体查询超时（默认 5s） |

退出码：默认宽容模式下，只要任一 type 返回 NOERROR（含空记录）就 `exit 0`；所有 type 都失败才 `exit 1`。`--strict` 切换为严格模式，任一 type 失败立即 `exit 1`。

默认从 `/etc/resolv.conf` 读取系统 DNS server，读不到时 fallback 到 `8.8.8.8:53`。顶部一行会打印 `domain — via X.X.X.X:53` 说明实际查询源，便于在 VPN / 公司内网 / DNS 劫持环境下确认查询路径。

**通过 DoH (DNS-over-HTTPS, RFC 8484) 绕过本地 DNS 劫持：**

```bash
jdan dns lookup example.com --doh google         # 使用 Google DoH (8.8.8.8)
jdan dns lookup example.com --doh cloudflare     # 使用 Cloudflare DoH (1.1.1.1)
jdan dns lookup example.com --doh quad9          # 使用 Quad9 DoH (9.9.9.9)
jdan dns lookup example.com --doh dns.google     # 主机名形式（自动补 /dns-query）
jdan dns lookup example.com --doh https://dns.alidns.com/dns-query  # 自定义完整 URL
```

支持的内置别名（共 6 个）：

| 别名 | DoH endpoint | Bootstrap IPs |
|------|--------------|----------------|
| `google` | `https://dns.google/dns-query` | `8.8.8.8` / `8.8.4.4` |
| `cloudflare` | `https://cloudflare-dns.com/dns-query` | `1.1.1.1` / `1.0.0.1` |
| `quad9` | `https://dns.quad9.net/dns-query` | `9.9.9.9` / `149.112.112.112` |
| `opendns` | `https://doh.opendns.com/dns-query` | `208.67.222.222` / `208.67.220.220` |
| `ali` | `https://dns.alidns.com/dns-query` | `223.5.5.5` / `223.6.6.6` |
| `360` | `https://doh.360.cn/dns-query` | `101.226.4.6` / `218.30.118.6` |

**别名形式**会用内置的 Bootstrap IPs 直连对应的 DoH 服务器，**完全绕过本地 resolver**——这是 jdan dns lookup 在 DNS 被劫持环境下的"看真相"模式。TLS SNI 仍是 endpoint 的 host 名（`dns.google` 等），证书验证不变。机制与 `curl --resolve` 一致。

**主机名 / 完整 URL 形式**走 OS resolver 解析 DoH host，适合非劫持环境或自定义 DoH 服务器（含 NextDNS 等带 UUID path 的私有 endpoint）。

`--doh` 与 `--server` 互斥；默认验证 TLS 证书（DoH 不提供 `--insecure-tls`）。

> 仅支持 macOS 和 Linux；Windows 暂不在 first release 范围内（resolver 自动检测的 Windows 路径需单独实现）。

### `jdan dns reverse`

把 IP 反向解析为域名（PTR 查询）。`jdan dns lookup` 的对偶——前者"域名 → 信息"，后者"IP → 域名"。

```bash
jdan dns reverse 8.8.8.8                    # 默认走系统 resolver
jdan dns reverse 8.8.8.8 --doh cloudflare   # 通过 DoH 绕过本地劫持
jdan dns reverse 1.1.1.1 --doh google       # 任意内置别名（与 dns lookup 一致）
jdan dns reverse 2001:4860:4860::8888       # IPv6 自动用 ip6.arpa
jdan dns reverse 8.8.8.8 --short            # 仅输出 PTR 值（脚本友好）
jdan dns reverse 8.8.8.8 --json             # 完整 metadata（含 display_name 字段）
```

支持与 `jdan dns lookup` 完全相同的 flag：`--server` / `--doh` / `--json` / `--short` / `--verbose` / `--strict` / `--timeout`。**唯一不同**是没有 `--type`——reverse 只查 PTR 一种 record type。`--doh` 别名（`google` / `cloudflare` / `quad9` / `opendns` / `ali` / `360`）依然走内置 IP 直连，劫持环境下也能拿到真实 PTR。

**输入要求**：只接受单一 IP 字面量（IPv4 或 IPv6）。以下输入会被拒绝并提示正确用法：

| 输入 | 错误提示 |
|------|----------|
| 域名（如 `google.com`） | "请用 `jdan dns lookup`" |
| CIDR（如 `8.8.8.8/32`） | "请传单一 IP" |
| host:port（如 `8.8.8.8:53`） | "不要传端口" |
| 带 zone-id 的链路本地（如 `fe80::1%en0`） | "不是合法 IP" |

`0.0.0.0` / `127.0.0.1` / 私网 IP 等不拦截——按"DNS 真相"原则透传查询（多数返回 NXDOMAIN），与命令的诊断定位一致。

**输出顶部**显示原始 IP（`8.8.8.8 — via …`），不是 `8.8.8.8.in-addr.arpa.` 形式。JSON 输出含 `display_name` 字段（原始 IP）+ `domain` 字段（实际查询的 arpa 域名），方便脚本根据需要消费。

### `jdan dns trace`

从根 DNS 服务器开始**迭代解析**，展示每一跳的委派路径（`dig +trace` 的 jdan 同款）。`jdan dns lookup` 是"问 recursive resolver 拿最终答案"，`jdan dns trace` 是"自己一跳一跳走完全程，看每个 NS 怎么把你交给下一跳"。

```bash
jdan dns trace example.com                  # 从 13 个根开始追，默认查 A
jdan dns trace example.com -t NS            # --type 覆盖（dig +trace 风格）
jdan dns trace example.com --doh google     # glueless NS 走 DoH bootstrap（绕本地劫持）
jdan dns trace example.com --short          # 仅最终答案
jdan dns trace example.com --json | jq '.hops | length'  # 脚本消费
jdan dns trace example.com --verbose        # 每跳含 NS referrals 与 glue 详情
jdan dns trace example.com -s 1.1.1.1       # 用 recursive resolver 作起步 server
jdan dns trace example.com --hop-timeout 2s --timeout 15s
```

**与 `jdan dns lookup` 的核心差异**：

| | `dns lookup` | `dns trace` |
|---|--------------|-------------|
| 查询模型 | 单次问 recursive resolver | 多跳从根迭代追 auth NS |
| 走 DoH | `--doh` 把整条查询切到 HTTPS | `--doh` **仅**用于 glueless NS bootstrap，主跳路径仍直接 UDP/TCP 查权威 NS |
| `--server` | DNS resolver IP | 起步 NS IP（覆盖 13 个根） |
| 默认 type | 6 个 type 并发 | 仅 A，`--type` 覆盖（dig 风格；多 type 会让 chain ×6） |
| 适用场景 | "这个域名 / IP 现在解析到哪" | "委派链路是怎么走的、哪一跳慢、NS 委派对了吗、本地是否被劫持" |

**Hijack detection（重要）**：trace 自带一个 sanity check——根服务器对非根域名查询本应返回 REFERRAL 而非 ANSWER。在被网关拦截 UDP-53 的网络下（连发往根服务器 IP 的流量都被伪造响应），第一跳直接给 ANSWER 会被识别为"可疑响应"并标 ERROR，提示用户改走 `jdan dns lookup --doh google` 走 HTTPS 加密查询。这是 trace 在污染网络下保持**不撒谎**的关键。

**`--strict` 在 trace 中的语义**：默认拿到 final answer 即 `exit 0`（即使中途某个 root server 超时被 fallback）。`--strict` 切换为"任一 hop 出错即 `exit 1`"——用于诊断"哪一跳不稳"。

> 仅支持 macOS 和 Linux；与 `dns lookup` / `reverse` 一致。

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

### `jdan readme`

输出指定目录（默认当前目录）下的 `README.md` 内容。文件名大小写不敏感，`README.md` / `readme.md` / `Readme.md` 等均可识别。

```bash
jdan readme                      # 输出当前目录的 README.md
jdan readme ./internal/cli       # 相对路径
jdan readme /path/to/project     # 绝对路径
jdan readme ~/code/myrepo        # 支持 ~ 展开
jdan readme --paging             # 强制启用 bat 分页器（可按空格/回车翻页，q 退出）
```

| 参数 | 说明 |
|------|------|
| `dir` | 目录路径（可选，默认当前目录） |
| `--paging` | 使用 bat 时强制启用分页（等同于 `bat --paging=always`）；默认不分页 |

渲染方式按以下优先级选择：

1. 若 `PATH` 中存在 `bat`，使用 `bat` 输出（带语法高亮）。默认追加 `--paging=never` 一次性输出；加 `--paging` 后追加 `--paging=always` 进入 less 等分页器。
2. 否则若存在 `cat`，使用 `cat` 输出（`--paging` 对 `cat` 无效）。
3. 两者都不可用时（如 Windows 默认环境），直接读取文件内容写到标准输出。

若目录中没有任何大小写形式的 `README.md`，会以非零退出码报错。

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
