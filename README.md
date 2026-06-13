# jdan

Go 编写的常用小工具集合（单二进制）。定位：每个子命令解决一个**系统自带工具行为不一致 / 输出难看 / 跨平台缺失**的小痛点，组合在一起避免装一堆小工具。设计倾向：默认聪明（合理 default + 自动检测），但不剥夺用户控制权（所有自动行为都能通过 flag 覆盖）；text 默认友好，`--json` 始终可被脚本消费。

## 安装

### 方式 1：下载预编译二进制（推荐）

从 [GitHub Releases](https://github.com/xunull/jdan/releases) 下载对应平台的 archive：

| 平台 | Archive |
|------|---------|
| macOS Apple Silicon | `jdan_<VERSION>_darwin_arm64.tar.gz` |
| macOS Intel | `jdan_<VERSION>_darwin_amd64.tar.gz` |
| Linux x86_64 | `jdan_<VERSION>_linux_amd64.tar.gz` |
| Linux ARM64 | `jdan_<VERSION>_linux_arm64.tar.gz` |

```bash
# 例：macOS Apple Silicon，把 <VERSION> 换成你下的版本号
curl -L -o jdan.tar.gz https://github.com/xunull/jdan/releases/download/v<VERSION>/jdan_<VERSION>_darwin_arm64.tar.gz
tar xzf jdan.tar.gz
sudo mv jdan /usr/local/bin/
jdan version
```

校验 SHA256（同一 Release 页面有 `checksums.txt`）：

```bash
curl -LO https://github.com/xunull/jdan/releases/download/v<VERSION>/checksums.txt
shasum -a 256 -c checksums.txt --ignore-missing
```

### 方式 2：go install

```bash
go install github.com/xunull/jdan@latest
```

此方式 `jdan version` 显示的是 `dev / none / unknown`，因为没经过 goreleaser 的 ldflags 注入。

### 方式 3：从源码构建

```bash
git clone https://github.com/xunull/jdan.git
cd jdan
go build -o jdan .
# 若上级目录存在 go.work 导致构建报错：
# Linux/macOS: GOWORK=off go build -o jdan .
# Windows PowerShell: $env:GOWORK="off"; go build -o jdan.exe .
```

## 命令

按主题分组的目录（实际章节顺序按命令引入时间排列，所以网络类和文件类不连续）：

**网络 & DNS**
- [`jdan http timing`](#jdan-http-timing) — 测 HTTP 请求各阶段耗时
- [`jdan http serve`](#jdan-http-serve) — 临时静态文件服务器 + LAN URL + 终端二维码
- [`jdan dns lookup`](#jdan-dns-lookup) — 并发查询 6 个 record type，含 DoH 支持
- [`jdan dns reverse`](#jdan-dns-reverse) — IP → 域名（PTR 查询）
- [`jdan dns trace`](#jdan-dns-trace) — 从根服务器迭代解析，看委派路径
- [`jdan pubip4`](#jdan-pubip4--jdan-pubip6) / [`jdan pubip6`](#jdan-pubip4--jdan-pubip6) — 查本机公网 IP
- [`jdan ports`](#jdan-ports) — 显示本机正在监听的端口（macOS）

**文件 & 归档**
- [`jdan file bak`](#jdan-file-bak) — 给文件打带时间戳的备份
- [`jdan zip`](#jdan-zip) — 把文件或目录打成 `.zip`
- [`jdan tree2`](#jdan-tree2) — 多列展示两层目录树
- [`jdan readme`](#jdan-readme) — 输出指定目录的 README.md（带 bat 高亮）

**系统**
- [`jdan macgpu`](#jdan-macgpu) — Apple Silicon GPU TUI 监控
- [`jdan unix-time`](#jdan-unix-time) — Unix 时间戳 → 本地时间

**随机生成（CSPRNG）**
- [`jdan rand password`](#jdan-rand-password) — 1Password 风格随机密码
- [`jdan rand uuid`](#jdan-rand-uuid) — UUID v4 / v7
- [`jdan rand hex`](#jdan-rand-hex--base64--base64url--base32) / [`base64`](#jdan-rand-hex--base64--base64url--base32) / [`base64url`](#jdan-rand-hex--base64--base64url--base32) / [`base32`](#jdan-rand-hex--base64--base64url--base32) — 字节级随机 + 编码
- [`jdan rand alnum`](#jdan-rand-alnum) — 字母数字串（无类约束）
- [`jdan rand int`](#jdan-rand-int) — 闭区间随机整数
- [`jdan rand word`](#jdan-rand-word) — EFF diceware passphrase

**集成**
- [`jdan obsidian install-claudian`](#jdan-obsidian-install-claudian) — 装 Claudian Obsidian 插件

**编码 & 二维码**
- [`jdan qr`](#jdan-qr) — 生成二维码（终端 / PNG / SVG）
- [`jdan jwt decode`](#jdan-jwt-decode) — 纯本地 JWT 解码（不验签、不联网）

**元命令**
- [`jdan version`](#jdan-version) — 显示版本、commit、构建时间

### `jdan qr`

把任意字符串生成二维码并打印到终端，或写入 PNG / SVG 文件。**用途**：临时分享 URL 到手机（扫码）、把 Wi-Fi 密码 / 配置串嵌到文档、给将来的 `jdan http serve` 输出 LAN URL 时复用。

**终端默认用半角块** `▀▄` 叠合渲染，高度减半，长 URL 不至于撑爆 80 列宽：

```bash
$ jdan qr "https://github.com/xunull/jdan"

█▀▀▀▀▀█ ▄█  ▀▀▄▄█▄▄█  █▀▀▀▀▀█
█ ███ █   ▄ ▄██▄▀ ▄▀  █ ███ █
█ ▀▀▀ █ ▀▀▄ ▄▄▀  ███▄ █ ▀▀▀ █
▀▀▀▀▀▀▀ ▀ ▀▄█▄█ █▄▀▄█ ▀▀▀▀▀▀▀
...
```

flags：

| flag | 默认 | 作用 |
|------|------|------|
| `--ecc` | `M` | 容错等级 `L/M/Q/H`（30% 容错的 H 适合含 logo 或可能被遮挡） |
| `--invert` | false | 反色，适合白底终端 |
| `--full-block` | false | 用全角 `██` 而不是半角，兼容老终端（如某些 Windows CMD） |
| `--output <path>` | 终端 | 按扩展名写文件：`.png` / `.svg` |
| `--png-size <int>` | 256 | PNG 像素尺寸 |
| `--svg-module <int>` | 8 | SVG 每模块像素数 |
| `--json` | false | 输出 `{data, ecc, modules}` 元信息 |

stdin 也可以：

```bash
echo "data" | jdan qr
cat secret.txt | jdan qr --output secret.png --ecc H
```

不支持的扩展名（如 `.jpg`）会报错；要 JPEG 自行用 `sips`/`ffmpeg` 从 PNG 转。

### `jdan jwt decode`

纯本地解析 JWT 的 header 和 payload，**不验签、不发任何网络请求**。日常调试场景里通常不需要验签：你只想看一眼 token 里到底装了什么 claim，且不想把可能含 PII 的 token 粘到 jwt.io 这类在线工具。

```
$ jdan jwt decode "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImFiYyJ9..."

Header:
  {
    "alg": "RS256",
    "kid": "abc",
    "typ": "JWT"
  }

Payload:
  {
    "iat": 1516239022,
    "name": "John Doe",
    "sub": "1234567890"
  }

算法: RS256
Key ID: abc
Subject: 1234567890
签发: 2018-01-18 01:30:22 UTC
Signature: (present, 21 chars base64url)
```

**设计细节**：

- **不引 jwt 库**：JWT 三段 base64url 用 stdlib 20 行就能解开；引 `golang-jwt` 反而会暴露 secret/key API 表面，让用户误以为本工具会做签名验证
- **签名段在文本输出里只显示字符数**，不打印原文，避免误粘到 PR / 日志 / Slack 里
- **`--json` 输出含完整 signature**（脚本场景需要它做 verify pipeline）
- exp / iat / nbf 自动按 RFC 7519 NumericDate（unix 秒）解读；过期会标注 "已过期"，未过期显示剩余时间（紧凑写法 `3d 4h`）
- `aud` 支持 `string` 或 `[]string` 两种 RFC 7519 合法形态

flags：

| flag | 作用 |
|------|------|
| `--header-only` | 只输出 header（不打印 payload，适合只想看 alg/kid 的场景） |
| `--json` | 结构化 JSON 输出，含完整 signature，便于脚本消费 |
| `--raw` | 不 pretty-print，输出紧凑 JSON |

stdin 输入也行（适合从 `kubectl get secret` 等命令链管下来）：

```bash
echo "$TOKEN" | jdan jwt decode
kubectl get secret my-jwt -o jsonpath='{.data.token}' | base64 -d | jdan jwt decode
```

**不提供的功能**（设计取舍）：

- 不验签 —— 后续可能加 `jdan jwt verify --key ...` 单独子命令
- 不查 issuer 的 jwks_uri —— 任何网络行为都属于 `verify` 而不是 `decode`
- 不构造 JWT —— 同上

### `jdan version`

显示当前二进制的版本号、构建 commit、构建时间和平台。release 二进制由 GoReleaser 通过 `-ldflags` 在 build 时注入；`go install` 或本地 `go build` 编译的二进制会显示 `dev / none / unknown`，这是设计内的回退。

```bash
$ jdan version
jdan v0.1.0 (commit abc1234, built 2026-06-12T10:00:00Z, darwin/arm64)

$ jdan version --short
v0.1.0
```

`--short` 适合在脚本里捕获版本号：

```bash
JDAN_VER=$(jdan version --short)
echo "running jdan $JDAN_VER"
```

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

### `jdan zip`

把指定的**文件**或**目录**打成 zip 归档。输出文件命名为 `{源名}.zip`，写到**当前工作目录**（不是源所在目录）。

```bash
jdan zip ./report.pdf      # 生成 report.pdf.zip 到 CWD
jdan zip ./my-project      # 递归压缩目录，生成 my-project.zip
jdan zip /tmp/data         # 绝对路径也行，输出仍写到 CWD
```

| 参数 | 说明 |
|------|------|
| `path` | 文件或目录路径（必传） |

实现细节：

- 使用 Go 标准库 `archive/zip`，压缩方法 `Deflate`
- 目录场景下递归遍历，zip 内以源目录的 basename 作为根目录
- 不支持密码、不支持排除规则、不支持自定义输出名——保持单一职责
- 不依赖系统 `zip` 二进制，跨平台一致

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

### `jdan http serve`

临时静态文件服务器。**核心动作**：找空闲端口（8080 起 fallback）→ 探测 LAN IP（RFC1918 私有段）→ 在终端打印 LAN URL 的二维码（复用 `jdan qr` 的渲染器）→ 监听访问日志 → Ctrl+C 优雅关闭并打 summary。**用途**：mac → 手机文件传输、给同事分享 build artifact、临时分发安装包。

```bash
$ jdan http serve ~/Downloads

⚠  serving on all interfaces (0.0.0.0:8080) — anyone on your LAN can read these files
   to limit to localhost: --bind 127.0.0.1

serving /Users/quincy/Downloads on:
  http://localhost:8080
  http://192.168.10.16:8080

  █▀▀▀▀▀█ ▄ ▄ ▀▄█ █▀▀▀▀▀█
  █ ███ █  ▄▄ ▀  █ ███ █     ← 192.168.10.16:8080 的二维码
  █ ▀▀▀ █ ▀▄█▄▀▀▄ █ ▀▀▀ █
  ▀▀▀▀▀▀▀ ▀▄█▄▀▄█ ▀▀▀▀▀▀▀
  ...

press Ctrl+C to stop

[GET] 200 /             127.0.0.1     12ms  (3.2KB)
[GET] 200 /report.pdf   192.168.10.42 78ms  (124.3KB)  ← 手机扫码后下载
^C

served 2 request(s) to 2 client(s), 127.5KB total
```

**关键设计**：

- **默认 `--bind 0.0.0.0`**（LAN 可达），启动打 ⚠ 警告显眼提示风险。`--bind 127.0.0.1` 选退。这是 `python -m http.server` / `npx serve` 等的惯例
- **端口自动找空闲**：默认从 8080 试到 8129，失败回退到内核分配的随机端口
- **LAN IP 探测纯本机**：遍历 `net.Interfaces()` 过滤 loopback/down/IPv6 link-local，挑 RFC1918 私有地址。**不联网**（不像 `jdan pubip4` 查公网）
- **二维码用第一个 LAN IP**（家用 WiFi 一般 `192.168.1.x`，优先级高于 `10.x` 和 `172.16-31.x`）
- **单文件 serve**：`jdan http serve report.pdf` 自动 serve 父目录，根路径 `/` 重定向到 `/report.pdf`
- **directory traversal 防护**：`http.FileServer` 内置 `..` 路径清理 + symlink 跳出 root 检查（`filepath.EvalSymlinks` 规范化后比对前缀，特别处理 macOS `/var` → `/private/var` symlink）
- **优雅关闭**：SIGINT/SIGTERM 触发 `http.Server.Shutdown(5s)`，已有下载不被切断
- **`--upload` 双向模式**：启用后 `POST /upload` 接收 multipart 表单写入 `<root>/uploads/`，同名加时间戳后缀防覆盖；`GET /upload` 返回 mobile-friendly HTML 表单方便手机浏览器选文件

flags：

| flag | 默认 | 作用 |
|------|------|------|
| `--port` | 0（自动） | 强制端口，否则 8080 → +1 → 随机 |
| `--bind` | `0.0.0.0` | 绑定地址 |
| `--no-qr` | false | 不打印终端二维码 |
| `--upload` | false | 启用 `POST /upload` + 上传表单 |
| `--upload-dir` | `<root>/uploads` | 上传文件落地目录 |
| `--auth` | 无 | Basic Auth `user:pass` |
| `--quiet` | false | 不打访问日志 |
| `--json` | false | 访问日志输出 ndjson（每行一个 event） |

**有意不做**：

- TLS / HTTPS —— 自签证书 UX 越来越差（现代浏览器警告劝退），HTTPS 留给 reverse proxy。分享 5 分钟下载不值得这个复杂度
- 自动开浏览器 —— 服务器场景常常用 ssh，没浏览器；手动复制 URL 不麻烦

#### macOS firewall：LAN 连接被拒绝

**症状**：`jdan http serve` 启动后，**本机 `http://localhost:8080` 通**，但**用 LAN IP（如 `http://192.168.1.42:8080`）访问就 "Connection Refused" / "拒绝连接"**。

**原因**：macOS 自带的 Application Firewall 默认拦截**所有未经 Apple Developer 签名的二进制**的入站连接。jdan 即使是从 GitHub Releases 下载的也没有 Apple 签名（Apple Developer Program 是 $99/年，开源工具一般不会签），所以会被默认 deny。`localhost` 走 lo0 不经防火墙，所以本机通。

启动 banner 会自动检测并打提示：

```
⚠  serving on all interfaces (0.0.0.0:8080) — anyone on your LAN can read these files
   to limit to localhost: --bind 127.0.0.1
ℹ  macOS firewall is on; unsigned binaries may be blocked from LAN access.
   if LAN clients get "connection refused", see README §macOS firewall.
```

**两种修法**：

**方案 1：临时关防火墙（测试时最快）**

```bash
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate off
# 测试完一定要恢复：
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on
```

**方案 2：把 jdan 加白名单（sustainable，推荐）**

```bash
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add $(which jdan)
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp $(which jdan)
```

二进制路径变了（比如重新 `go install`、换装 brew 版本）就要重新 `--add`。

也可以走 GUI：**System Settings → Network → Firewall → Options →** 点 `+` 加 jdan 二进制 → 设为 **"Allow incoming connections"**。

**根本解决**需要 Apple Developer 签名 + notarize，这不是 jdan 这一刻该做的事。同样的问题在 `python3 -m http.server`、`npx serve`、自 build 的 Rust 二进制上也都有。

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

### `jdan ports`

显示本机当前所有处于 LISTEN 状态的网络端口。表格按协议分块（TCP 在前 / UDP 在后），同协议内按端口号升序排列。

```bash
jdan ports               # 默认表格输出，TCP + UDP 都显示
jdan ports --tcp         # 仅 TCP（-t）
jdan ports --udp         # 仅 UDP（-u）
jdan ports --json        # JSON 数组输出（-j），脚本友好
```

| 参数 | 说明 |
|------|------|
| `-j` / `--json` | 以 JSON 数组输出（`[{protocol, address, port, process}, ...]`） |
| `-t` / `--tcp` | 仅显示 TCP 端口 |
| `-u` / `--udp` | 仅显示 UDP 端口 |

每条记录包含：`PROTOCOL`、`ADDRESS`（如 `127.0.0.1`、`*`、`[::1]`）、`PORT`、`PROCESS`（进程名）。

实现细节：

- 底层调用 macOS 内置的 `lsof -i -P -n -sTCP:LISTEN`（TCP）和 `-sUDP:LISTEN`（UDP）
- 无 sudo 时也能显示端口和地址；进程名权限不足时显示 `-`
- Docker 通过 `-p` 映射到宿主的端口会被检测到（宿主 socket 真实存在）
- 不显示 LISTEN 之外的连接状态（ESTABLISHED 等）

> 当前仅 macOS。Linux 支持留作未来扩展（用 `ss` 或 `/proc/net/{tcp,udp}` 替代 `lsof`）。

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

### `jdan rand`

随机生成子命令族。**全部使用 `crypto/rand` (CSPRNG)**，禁止 `math/rand`；字符选取
一律走 `crypto/rand.Int(charsetLen)`，禁止 `b[i] % len(charset)` 这种 mod-bias
写法（`TestNoCharSelectionModulo` 静态门禁）。

9 个子命令，全部接受共享 flag `--count N` / `--json` / `--no-newline`（互斥与
`--count >1`）：

```bash
jdan rand password                       # 1Password 风格：20 位 + symbols + 排除歧义
jdan rand uuid                           # 默认 v4
jdan rand uuid -V 7 -c 10                # 10 个 v7（time-ordered）
jdan rand hex -l 32                      # 32 字节 → 64 hex chars
jdan rand base64 -l 32                   # 标准 base64
jdan rand base64url -l 32                # URL-safe base64（无 +/=）
jdan rand base32 -l 20                   # RFC 4648 base32
jdan rand alnum -l 12                    # 字母数字（无类约束）
jdan rand int 1 100                      # [1, 100] 闭区间
jdan rand int -c 5 -- -10 10             # 负数请用 -- 分隔，flag 须在 -- 前
jdan rand word                           # 6 词 diceware passphrase (EFF 7776 词)
jdan rand word -w 8 --sep "_"            # 8 词，下划线分隔
jdan rand hex --json -c 100              # 100 条 → JSON 数组（脚本友好）
jdan rand password --no-newline | pbcopy # 单条无换行管道
```

#### `jdan rand password`

| 参数 | 默认 | 说明 |
|------|------|------|
| `-l` / `--length` | `20` | 密码长度 |
| `--no-symbols` | `false` | 仅字母数字（仍要求每类至少一个） |
| `--include-ambiguous` | `false` | 不排除 `I`/`l`/`1`/`O`/`0` |

算法：**固定位置 + Fisher-Yates 洗牌**（每类先抽 1 字符放固定位置，剩余位置用全
字符集填充，最后 Fisher-Yates 洗牌）。无偏差，`-l 4` 边界也高效。

`--no-symbols` 与 `jdan rand alnum` **不同**：前者仍要求 lower/upper/digit 每类
至少一个；后者无类约束。

熵参考：默认 20 位 + symbols + 排除歧义 ≈ 123 bits（字符集 71）；`--no-symbols`
≈ 117 bits（字符集 57）。

#### `jdan rand uuid`

| 参数 | 默认 | 说明 |
|------|------|------|
| `-V` / `--version` | `4` | UUID 版本（`4` 或 `7`） |

- **v4** = 122 个随机比特 + 版本/variant 标记。RFC 9562。
- **v7** = 48-bit unix 毫秒时间戳 + 74-bit 随机。同毫秒内 `rand_a` 提供大致单调
  排序，适合数据库索引。RFC 9562。
- v1（含 MAC 地址）和 v5（SHA-1 命名空间）不在 scope。

UUID 子命令**手写实现**，不引入 `github.com/google/uuid` 依赖。

#### `jdan rand hex` / `base64` / `base64url` / `base32`

| 参数 | 默认 | 说明 |
|------|------|------|
| `-l` / `--length` | `32` | 字节数（编码后输出更长） |

- `hex` → 输出 `2 × length` hex chars（`0-9a-f`）
- `base64` → 标准 base64（含 `+ / =` padding）
- `base64url` → URL-safe base64（用 `- _`，无 `=` padding，可直接放 URL / JWT）
- `base32` → RFC 4648 大写 `A-Z` + `2-7`。Crockford 变体不支持

#### `jdan rand alnum`

| 参数 | 默认 | 说明 |
|------|------|------|
| `-l` / `--length` | `20` | 字符长度 |
| `--include-ambiguous` | `false` | 不排除 `I`/`l`/`1`/`O`/`0` |

字母数字串，**无类约束**——`-l 1` 也合法。与 `password --no-symbols` 区别明确：
后者仍要求 lower/upper/digit 每类至少一个。

#### `jdan rand int`

```bash
jdan rand int <min> <max>
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `min` `max` | — | 必传，`cobra.ExactArgs(2)` |
| `-c` / `--count` | `1` | 生成数量 |
| `-j` / `--json` | `false` | JSON **整数**数组（非字符串数组） |

闭区间 `[min, max]`，支持负数 / 跨零 / `min == max`。负数请用 `--` 分隔，且 flag
必须在 `--` **之前**：

```bash
jdan rand int -c 5 -- -10 10   # ✓ 对
jdan rand int -- -10 10 -c 5   # ✗ 错（-c 5 被当 positional）
```

不支持 `--no-newline`（整数 + newline 是标准 stdout 格式）。

#### `jdan rand word`

| 参数 | 默认 | 说明 |
|------|------|------|
| `-w` / `--words` | `6` | 每个 passphrase 的词数 |
| `--sep` | `-` | 词之间分隔符（空串合法，输出不可分割串） |

从 **EFF Large Wordlist** (7776 词，CC-BY 3.0，`go:embed` 嵌入二进制，
SHA256 在 `init()` 时校验) 抽词。12.9 bits 熵/词；默认 6 词约 77.5 bits 熵
（超过 12 字符 alnum 密码 ≈ 71 bits）。

注意 **`--words` 是每个 passphrase 的词数；`--count` 是 passphrase 数**：

```bash
jdan rand word                         # 1 个 6 词 passphrase
jdan rand word -w 8                    # 1 个 8 词 passphrase
jdan rand word -c 5                    # 5 个 6 词 passphrase（每行一个）
jdan rand word -w 8 -c 5 --json        # 5 个 8 词 passphrase → JSON 数组
```

> 当前仅 macOS + Linux（沿用 jdan 现状）。

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

## 全局 flag

所有子命令都接受：

| 参数 | 说明 |
|------|------|
| `--config` | 配置文件路径（可选；viper 加载，子命令各自决定是否消费） |
| `-h` / `--help` | 子命令帮助 |

## 开发

```bash
# 单元测试（默认不跑需要外网的集成测试）
go test ./...

# 集成测试（真实打 DNS / DoH；CI 默认不跑）
go test -tags integration ./internal/dnslookup/... ./internal/dnstrace/...

# 构建（若上级目录有 go.work 干扰）
GOWORK=off go build -o jdan .
```

设计文档在 `docs/brainstorms/` 与 `docs/plans/` 下按时间排列，每个新子命令通常对应一对 brainstorm + plan。

