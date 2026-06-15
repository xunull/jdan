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
- [`jdan net probe`](#jdan-net-probe) — 客户端视角逐阶段（DNS/TCP/TLS/HTTP）探查
- [`jdan net selfcheck`](#jdan-net-selfcheck) — 服务端自检 + 外部访问预测
- [`jdan ssl cert`](#jdan-ssl-cert) — 看 HTTPS 证书详情（chain / verification / OCSP）
- [`jdan ssl scan`](#jdan-ssl-scan) — TLS 配置综合审计（ssllabs 风格 A+/A/B/C/D/F 评分）
- [`jdan ssl pin`](#jdan-ssl-pin) — 生成 cert pinning 用的 SPKI hash（6 种格式）
- [`jdan ssh-key`](#jdan-ssh-key) — SSH 密钥解析（info / fingerprint / pubkey，对齐 ssh-keygen）
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
- [`jdan totp`](#jdan-totp) — TOTP 2FA 验证码（RFC 6238，兼容 Google Authenticator）
- [`jdan b64 enc/dec`](#jdan-b64) — base64 编码/解码（standard / URL-safe / no-pad）
- [`jdan url enc/dec`](#jdan-url) — URL percent-encoding
- [`jdan num`](#jdan-num) — 进制转换（dec/hex/bin/oct）+ 位运算
- [`jdan env`](#jdan-env) — .env 文件工具（lint / diff / redact / get）

**JSON / YAML / CSV**
- [`jdan json`](#jdan-json) — pretty/minify/path/keys/diff/lines + yaml ↔ json + csv ↔ json

**网络 / 查询**
- [`jdan whois`](#jdan-whois) — 域名/IP WHOIS（自动路由 + IANA/ARIN referral 跟随 + parsed 表）
- [`jdan ip`](#jdan-ip) — IP / CIDR 计算（info / contains / range / split / normalize）

**文件 hash & 归档**
- [`jdan hash`](#jdan-hash) — 跨平台 md5/sha1/sha256/sha512 + `--check` 校验
- [`jdan extract`](#jdan-extract) — 通用解压 zip/tar/tar.gz/tar.bz2/gz/bz2



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

### `jdan hash`

跨平台计算文件的 md5 / sha1 / sha256 / sha512。**streaming**（不全读进内存，1GB+ 文件 OK）；多算法时一遍读取并行算（`io.MultiWriter` 喂多个 hasher）。

**为什么单独做一个**：macOS 的 `shasum -a 256` 跟 Linux 的 `sha256sum` 命令名不一致；`md5sum` 在 macOS 上根本没有（叫 `md5`）；输出格式还略有差异。`jdan hash` 跨平台一致 + 输出格式跟系统工具兼容。

```bash
$ jdan hash file.zip
edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb  file.zip

$ jdan hash file.zip --algo md5,sha256
MD5:    0bee89b07a248e27c83fc3d5951213c1
SHA256: edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb
file:   file.zip

$ jdan hash file.zip --all
MD5:    0bee89b07a248e27c83fc3d5951213c1
SHA1:   03cfd743661f07975fa2f1220c5194cbaff48451
SHA256: edeaaff3...
SHA512: 4f285d0c...

$ echo "hi" | jdan hash -
98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4  -
```

**`--check` 模式**（跟 `shasum -c` / `sha256sum -c` 输出 byte-equal）：

```bash
$ cat checksums.txt
abc123...sha256...  file1.zip
def456...sha256...  file2.tar

$ jdan hash --check checksums.txt
file1.zip: OK
file2.tar: OK

2 total, 0 failed
```

如果有 FAILED → exit 1，方便监控 / CI gate。**算法按 hash 长度自动识别**：32 chars = md5、40 = sha1、64 = sha256、128 = sha512。所以 `--check` 不需要再加 `--algo` flag。

**flags**：

| flag | 默认 | 作用 |
|------|------|------|
| `--algo` | `sha256` | csv：`md5,sha256` 多算法一遍读取 |
| `--all` | false | md5 + sha1 + sha256 + sha512 全跑（覆盖 `--algo`） |
| `--check <file>` | 无 | 校验模式；FAILED → exit 1 |
| `--json` | false | 结构化输出 |

**有意不做**：

- xxh3（非加密但 4 GB/s 的 hash）—— 引第三方 dep（`github.com/zeebo/xxh3`），等用户真要再加
- BLAKE2 / BLAKE3 —— 同上
- `--binary` flag（跟 GNU `sha256sum -b` 对齐）—— 文本 / binary 模式在 Unix 上没区别

### `jdan extract`

通用解压。识别 8 种格式（按文件扩展名），拒绝 directory traversal（`..` 跳出 root）。

**为什么单独做一个**：`tar xzvf` vs `unzip` vs `bzip2 -d` 各自语法不同，命令选错就报错。`jdan extract <anything>` 自动按扩展名识别格式。

```bash
$ jdan extract release.tar.gz
✓ extracted 42 entry(ies) to release

$ jdan extract data.zip -o /tmp/out
✓ extracted 7 entry(ies) to /tmp/out

$ jdan extract docs.zip --here          # 不创建子目录，解压到 cwd
✓ extracted 12 entry(ies) to .

$ jdan extract data.zip --list          # 只列内容不解压
archive: data.zip  (5 entries, 1.2MB total)

           1.2KB  README.md
           300KB  bin/foo
  d            -  bin/
           950KB  data.json
```

**默认行为**：解压到当前目录的 `<archive-name>/` 子目录。`.tar.gz` / `.tar.bz2` / `.tgz` 去掉**双后缀**（`release.tar.gz` → `release/`）。

**支持的格式**：

| 格式 | 检测后缀 |
|------|---------|
| zip | `.zip` |
| tar | `.tar` |
| tar.gz | `.tar.gz` / `.tgz` |
| tar.bz2 | `.tar.bz2` / `.tbz2` / `.tbz` |
| gz（单文件）| `.gz`（输出去掉 `.gz` 后缀的文件） |
| bz2（单文件）| `.bz2` |

**flags**：

| flag | 默认 | 作用 |
|------|------|------|
| `-o` / `--output` | `<archive-name>/` 子目录 | 解压目标目录 |
| `--here` | false | 解压到 cwd（不创建子目录） |
| `--list` | false | 只列内容，不实际解压 |
| `--json` | false | 结构化输出 |

**安全**：

- **拒绝 directory traversal**：entry 名含 `..` 段直接 reject（不静默 sanitize）—— zip slip 攻击的标准防护
- **拒绝绝对路径 entry**：`/etc/passwd` 这类名也 reject
- **拒绝 symlink entry**：tar 里的 symlink 跳过（防 symlink-then-write 攻击）
- **4 GiB 单 entry 上限**：防 zip bomb

**有意不做**：

- `.7z` —— 外部 lib（`github.com/saracen/go7z` 或调用 7zz 二进制）复杂
- `.tar.xz` —— Go stdlib 无 lzma；引 `github.com/ulikunitz/xz` 是新 dep，等用户真要再加
- `.rar` —— 专利问题

### `jdan totp`

TOTP 2FA 验证码工具（RFC 6238）。3 个子命令覆盖 **生成 / 解析 otpauth URI / 验证**。兼容 Google Authenticator / Authy / 1Password。

详细技术文档：[docs/jdan-totp.md](docs/jdan-totp.md)

**为什么单独做一个**：secret 已经在本机时（dotfiles / 密码管理器导出 / CI secret），CLI 直接出码秒杀"掏手机 → 开 app → 找条目 → 念数字"的流程；脚本里还能自动填 2FA。0 新依赖（`crypto/hmac` + `encoding/base32` 都是 stdlib）。

> ⚠️ **secret 是长期凭证**。直接传 arg 会进 shell history + 进程列表(`ps`)，只适合临时/测试。长期用走 stdin 或环境变量。

```bash
# 生成当前码（默认对齐 Google Authenticator：SHA1/6位/30s）
$ jdan totp code JBSWY3DPEHPK3PXP
283461   (expires in 17s)

# 更安全的 secret 来源（不进 history）
$ echo "$SECRET" | jdan totp code -
$ JDAN_TOTP_SECRET="$SECRET" jdan totp code

# 解析扫码得到的 otpauth URI（参数自动用上）
$ jdan totp uri "otpauth://totp/GitHub:quincy?secret=JBSWY3DP&issuer=GitHub&digits=6&period=30"
Issuer:    GitHub
Account:   quincy
Algorithm: SHA1
Digits:    6
Period:    30s
Code:      283461   (expires in 17s)

# 验证一个码（退出码 0/1，--window 容时钟漂移）
$ jdan totp verify JBSWY3DPEHPK3PXP 283461
✓ valid

# JSON 给脚本消费
$ jdan totp code JBSWY3DPEHPK3PXP --json
{"code":"283461","expires_in":17,"period":30,"digits":6}
```

**base32 secret 容错**：小写 / 空格分组（Google 显示格式）/ 缺 padding 全部自动处理。少数用 SHA256 或 8 位码的服务用 `--algo` / `--digits` 覆盖，或直接走 `uri`（参数在 URI 里）。

实现跟 **RFC 6238 / RFC 4226 官方测试向量 byte-equal**（TOTP 实现的金标准）。

### `jdan b64`

base64 编码/解码。支持 standard / URL-safe 字母表 + 可选 padding。

```bash
$ jdan b64 enc "hello world"
aGVsbG8gd29ybGQ=

$ jdan b64 dec "aGVsbG8gd29ybGQ="
hello world

$ jdan b64 enc "data" --url --no-pad      # URL-safe + 去 padding
ZGF0YQ

$ echo "secret" | jdan b64 enc -          # stdin
c2VjcmV0Cg==

$ jdan b64 enc -i input.bin -o out.b64    # file → file
```

| flag | 作用 |
|------|------|
| `--url` | URL-safe 字母表（`-_` 替换 `+/`） |
| `--no-pad` | 去掉末尾 `=` padding（用于 enc）|
| `-i <file>` | 从文件读 |
| `-o <file>` | 写到文件 |
| `--no-newline` | enc 输出末尾不加换行（脚本管道用） |

**dec 自动识别 padding**：含 `=` 用 standard，不含用 raw。无需 flag。

### `jdan url`

URL percent-encoding / decoding（RFC 3986）。

```bash
$ jdan url enc "hello world"
hello%20world

$ jdan url dec "hello%20world"
hello world

$ jdan url enc "a b" --query              # query string 模式（+ 代空格）
a+b

$ jdan url dec "a+b" --query
a b
```

**path vs query 模式**：

| 模式 | 空格编码为 | 用途 |
|------|-----------|------|
| 默认 / `--path` | `%20` | URL path 段 / 大多数场景 |
| `--query` | `+` | URL query string（兼容 application/x-www-form-urlencoded）|

### `jdan num`

进制转换 + 位运算工具。主命令自动检测输入进制，一次性输出 dec/hex/bin/oct + 位信息；`bit` 子命令做位运算。uint64 范围，0 新依赖（纯 `strconv` + `math/bits`）。

详细技术文档：[docs/jdan-num.md](docs/jdan-num.md)

**为什么单独做一个**：看寄存器值、Unix 权限位、flag mask、子网掩码时要在进制间转换，掏计算器/开 python 都慢。`jdan num` 一行出全部进制 + 位展示，`bit` 子命令直接算位运算。

```bash
# 自动检测进制（0x/0b/0o/前导0/十进制），一次出全部
$ jdan num 0xDEADBEEF
Decimal:  3735928559
Hex:      0xDEADBEEF
Binary:   0b11011110101011011011111011101111
Octal:    0o33653337357
Bits:     24 set (...), width 32

# 位展示（看 flag / mask）
$ jdan num 0b10110 --bits
...
          bit:  4 3 2 1 0
          val:  1 0 1 1 0

# 二进制零填充对齐寄存器
$ jdan num 0xFF --width 16
Binary:   0b0000000011111111

# 位运算（AND/OR/XOR/NOT/<</>>，符号别名 & | ^ ~）
$ jdan num bit "0xFF AND 0x0F"
0x0F  (15, 0b1111)
$ jdan num bit "1 << 8"
0x100  (256, 0b100000000)
$ jdan num bit "NOT 0xFF" --width 8
0x0  (0, 0b0)

# JSON 给脚本消费
$ jdan num 255 --json
{"decimal":255,"hex":"0xFF","binary":"0b11111111","octal":"0o377","bits_set":8,"bit_width":8}
```

**uint64 范围**，负数 / 超 64 位清晰报错不静默 wrap。跟 `jdan hash` / `jdan b64` 同属"编码/进制"工具。

### `jdan env`

`.env` 文件检查工具。4 个子命令覆盖 **lint / diff / redact / get**。偏"检查 / 对比 / 脱敏"，不做加载（那是 `direnv` / `dotenv-cli`）。0 新依赖。

详细技术文档：[docs/jdan-env.md](docs/jdan-env.md)

**为什么单独做一个**：`.env` 出问题很隐蔽——prod 少个 key 部署才发现、未引号空格被 shell 截断、重复 key 悄悄覆盖、贴 issue 泄露 secret。`jdan env` 把这些变成一行命令。

```bash
# lint：6 类检查，error → 退出码 1（CI gate）
$ jdan env lint .env
.env:3   warning  duplicate key DATABASE_URL (first at line 1)
.env:5   warning  unquoted value with spaces: KEY=hello world
.env:6   error    invalid key name "2FOO" (must match [A-Za-z_][A-Za-z0-9_]*)

# diff：部署前查漏 key（默认只比 key 不泄露 value）
$ jdan env diff .env.example .env
Only in .env.example (3):
  + STRIPE_SECRET_KEY
  + REDIS_URL
  + SMTP_HOST
Common keys: 12
$ jdan env diff .env.example .env.prod --exit-code && echo "all keys present"

# redact：脱敏后安全贴 issue
$ jdan env redact .env | pbcopy
DATABASE_URL=po**************************db
export API_KEY=sk***********56

# get：比 grep+cut 可靠（处理引号 / export / 行内注释）
$ jdan env get .env DATABASE_URL
postgres://localhost:5432/mydb
```

支持引号 / `export` 前缀 / 行内注释 / 重复 key 取最后（shell 语义）。`--strict`（warning 也拦）/ `--values`（diff 比 value）/ `--full` / `--keep-short`（redact 策略）。

### `jdan json`

JSON 工具集（**10 个子命令**）。设计目标：常见操作 0 学习曲线，**不替代 jq**。复杂查询请用 jq；jdan json 覆盖日常 80% 高频场景。

详细技术文档：[docs/jdan-json.md](docs/jdan-json.md)

**为什么单独做一个**：`python -m json.tool` 美化但参数难记 + 丢数字精度；`jq` 强大但语法陡；YAML / CSV 想转 JSON 要单独装 `yq` / `csvjson`；JSONL（结构化日志）没有趁手命令。`jdan json` 一组命令统一搞定。

```bash
# 美化 / 压缩（保留数字精度，2^53 + 1 不丢）
$ jdan json pretty data.json
$ jdan json minify data.json --in-place

# 按 path 取值（dot-path / bracket / RFC 6901 三选一，可混用）
$ jdan json path "users[0].name" data.json
"alice"
$ jdan json path "users.0.name" data.json -r       # -r 去引号
alice
$ jdan json path "/users/0/name" data.json --pointer

# 列 key（顶层 / 递归所有路径）
$ jdan json keys data.json --all
age
name
users[0].email
users[0].name

# 语义 diff（输出 RFC 6902 JSON Patch）
$ jdan json diff a.json b.json
~ /age: 30 -> 31
+ /new = true
$ jdan json diff a.json b.json --json              # RFC 6902 patch
$ jdan json diff schema.json prod.json --exit-code # CI gate

# JSONL（结构化日志，一行一个 JSON）
$ jdan json lines --count < logs.jsonl
12847
$ jdan json lines --head 5 < logs.jsonl

# YAML ↔ JSON（数字、嵌套、大 int 都不丢精度）
$ jdan json from-yaml config.yaml > config.json
$ jdan json to-yaml config.json > config.yaml

# CSV ↔ JSON（UTF-8 BOM 自动剥除、quoted fields 正确处理）
$ jdan json from-csv users.csv               # → array of objects
$ jdan json from-csv data.tsv --delim '\t'
$ jdan json to-csv users.json --header "name,age"
```

**与 jq 配合**：

```bash
# YAML → JSON 后用 jq 查询
$ jdan json from-yaml config.yaml | jq '.servers[].port'

# CSV → JSON 后取第一行的 name 字段
$ jdan json from-csv users.csv --pretty=false | jdan json path "0.name" -r
```

### `jdan whois`

WHOIS 查询命令（RFC 3912）。自动检测 domain vs IP，自动路由到正确的 server，跟随 IANA / ARIN referral 到最终响应，**默认输出解析后的字段表**。

详细技术文档：[docs/jdan-whois.md](docs/jdan-whois.md)

**为什么单独做一个**：macOS 自带的 BSD `whois` TLD 映射表过时（很多新 gTLD 不识别）；Linux 要 `apt install whois`；Windows 没有原生支持；各平台输出原始文本要靠人脑 grep。`jdan whois` 跨平台 0 配置 + 53 个内置 TLD 映射 + IANA fallback + parsed 表，关键字段（expiry/registrar/nameservers）一眼可见。

```bash
$ jdan whois example.com
Target:    example.com (domain)
Server:    whois.verisign-grs.com

  Domain:         EXAMPLE.COM
  Registrar:      RESERVED-Internet Assigned Numbers Authority
  Created:        1995-08-14 04:00 UTC  (31 years ago)
  Expires:        2026-08-13 04:00 UTC  (in 2 months)
  DNSSEC:         signedDelegation
  Status:         clientDeleteProhibited
                  clientTransferProhibited
                  clientUpdateProhibited
  Nameservers:    elliott.ns.cloudflare.com
                  hera.ns.cloudflare.com

$ jdan whois 193.0.0.1                        # IPv4 → ARIN → 跟到 RIPE
Target:    193.0.0.1 (ipv4)
Server:    whois.ripe.net
Chain:     whois.arin.net -> whois.ripe.net

  Range:          193.0.0.0 - 193.0.7.255
  Org:            Reseaux IP Europeens Network Coordination Centre (RIPE NCC)
  Country:        NL
  Abuse email:    abuse@ripe.net

$ jdan whois example.com --raw                # 原始 WHOIS 文本
$ jdan whois example.com --full               # parsed 表 + 原文
$ jdan whois example.com --json               # 结构化 JSON（含 parsed）
$ jdan whois example.com --server custom.whois.com  # 覆盖默认 server
```

**跟 jdan ssl cert 配套**：cert 看 TLS 过期，whois 看 domain 注册过期，两个都要监控：

```bash
# 监控 pipeline 示例
jdan whois example.com --json | jdan json path "parsed.expiry_date" -r
# → 2026-08-13T04:00:00Z

jdan ssl cert example.com --json | jdan json path "not_after" -r
# → 2026-XX-XX (cert 过期)
```

**parser 兜底**：parser 失败（schema 不识别如 `.br`）→ **自动回退到 raw**，永远有内容；`--raw` 永远拿原文，是 1st-class citizen。

### `jdan ip`

IP 地址 & CIDR 计算工具集。5 个子命令覆盖 **综合信息 / 网段包含判断 / IP 列表 / 子网划分 / IPv6 标准化**。

详细技术文档：[docs/jdan-ip.md](docs/jdan-ip.md)

**为什么单独做一个**：SRE / 网管 / 后端的日常工具链零碎（在线 CIDR 计算器、`ipcalc` 不跨平台、`sipcalc` 不在 macOS 默认装），且没有一个统一接口能同时吃 IP 和 CIDR、IPv4 和 IPv6。`jdan ip` 一组命令统一搞定，0 新依赖（纯 Go stdlib `net/netip`）。

```bash
# 综合信息（吃 IP / CIDR / IPv4 / IPv6）
$ jdan ip info 192.168.1.0/24
  CIDR:           192.168.1.0/24
  Version:        IPv4
  Network:        192.168.1.0
  Broadcast:      192.168.1.255
  First host:     192.168.1.1
  Last host:      192.168.1.254
  Netmask:        255.255.255.0
  Wildcard:       0.0.0.255
  Total IPs:      256
  Usable:         254

$ jdan ip info 192.168.1.42                  # 单 IP：分类 + binary/hex/decimal + reverse-DNS
  Address:        192.168.1.42
  Version:        IPv4
  Hex:            0xC0A8012A
  Decimal:        3232235818
  Binary:         11000000.10101000.00000001.00101010
  Reverse DNS:    42.1.168.192.in-addr.arpa
  Private:        yes

# 退出码：CI gate 友好
$ jdan ip contains 10.0.0.0/8 10.5.1.2 && echo "internal"
internal
$ jdan ip contains 10.0.0.0/8 10.5.1.2 --verbose
yes

# 子网划分
$ jdan ip split 10.0.0.0/22 24
10.0.0.0/24
10.0.1.0/24
10.0.2.0/24
10.0.3.0/24
(4 subnets)

# 列出 IP（默认 16 个，--limit 0 全列，硬上限 1M 防 OOM）
$ jdan ip range 192.168.1.0/29
192.168.1.0
...
192.168.1.7
(8 total)

# IPv6 expand / compact
$ jdan ip normalize 2001:db8::1 --expand
2001:0db8:0000:0000:0000:0000:0000:0001
```

**Classification 字段** 覆盖 RFC 1918 / 3849 / 4193 / 5737 / 6598：Private / Loopback / Multicast / Link-local / Doc range / Unique local / CGNAT / Global unicast 都打 tag。

**跟 whois / dns 配套**：

```bash
# WHOIS NetRange → ip 计算
jdan whois 8.8.8.8 --json | jdan json path "parsed.netrange" -r

# DNS A 记录拿 IP → 判断是否内部
ip=$(jdan dns lookup myserver.com -t A | tail -1)
jdan ip contains 10.0.0.0/8 "$ip" && deploy-internal
```

### `jdan ssh-key`

SSH 公钥/私钥解析工具。3 个子命令覆盖 **综合信息 / fingerprint / 公钥提取**。跟 `jdan ssl` 套件并列成"密钥/证书检查"工具。

详细技术文档：[docs/jdan-ssh-key.md](docs/jdan-ssh-key.md)

**为什么单独做一个**：`ssh-keygen` 语法零碎（`-lf` 看指纹、`-lf -E md5` 看 MD5、`-y` 提公钥），且没有单命令一次看"类型 + 位数 + fingerprint + comment"。`jdan ssh-key` 提供统一接口，自动识别公钥 vs 私钥，fingerprint 跟 ssh-keygen **byte-equal** 能交叉验证。0 新依赖（`golang.org/x/crypto/ssh` 已是 direct dep）。

```bash
# info：公钥/私钥都吃，一键全字段
$ jdan ssh-key info ~/.ssh/id_ed25519.pub
Type:         ssh-ed25519
Algorithm:    Ed25519
Bits:         256
Comment:      quincy@macbook
Fingerprint:  SHA256:Hk8x...
MD5:          MD5:43:51:43:a1:...

$ jdan ssh-key info ~/.ssh/id_rsa.pub     # RSA 显示真实位数（从 modulus 算）
Type:         ssh-rsa
Algorithm:    RSA
Bits:         4096
...

# 加密私钥：不解密只识别，不泄露 key material
$ jdan ssh-key info ~/.ssh/id_ed25519     # passphrase 保护的
Type:         OpenSSH private key
Encrypted:    yes (passphrase-protected; cannot derive public key without it)

# fingerprint：byte-equal 对齐 ssh-keygen -lf
$ jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
256 SHA256:Hk8x... quincy@macbook (ED25519)
$ jdan ssh-key fingerprint ~/.ssh/id_rsa.pub --md5
4096 MD5:43:51:... quincy@macbook (RSA)

# pubkey：私钥重建公钥（= ssh-keygen -y），丢了 .pub 文件时用
$ jdan ssh-key pubkey ~/.ssh/id_ed25519
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... quincy@macbook
```

**支持** Ed25519 / RSA / ECDSA (p256/384/521) + FIDO/U2F 硬件密钥（`sk-*`）。输入吃文件路径 / `-` stdin / 直接粘贴公钥字符串。

**典型用途**：验证本地 key 跟 GitHub / GitLab / server 上注册的是同一把：

```bash
jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
# → 256 SHA256:Hk8x...  ← 跟 GitHub Settings → SSH keys 显示的对比
```

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

### `jdan net probe`

从客户端视角逐阶段探查目标主机/端口/URL，**DNS → TCP → tcp_health → TLS → HTTP** 五阶段实时输出。**每个失败都带一个醒目的 `ErrorClass` 标签** 让你 0.5 秒识别"哪类问题"，配 **"what it means"** 中等长度解释和针对性 hint。**用途**：撞到"连不上 / 拒绝连接 / 证书报错 / 被踢"时，30 秒内定位是哪一层出问题。

```
$ jdan net probe https://github.com

✓ resolve      github.com → 1 record(s) via system resolver
  A     140.82.113.4
  duration: 8ms

✓ tcp          connected to 140.82.113.4:443 in 38ms
  ✓ 140.82.113.4 from 192.168.10.16:54321 (38ms)

✓ tcp_health   held 1s without remote close (healthy)

✓ tls          TLS 1.3, cert: github.com (issued by Sectigo, exp 2026-08-02)
  ALPN=h2, SNI=github.com, duration=142ms

✓ http         HEAD HTTP/2.0, 200 OK
  server: github.com
  content-length: 56012
  duration: 312ms

✓ all green · total 1521ms
```

失败时显示 `ErrorClass` 标签 + 三层信息（标签 / what it means / what to check）：

```
$ jdan net probe 127.0.0.1:1

✓ resolve      127.0.0.1 (literal IP)
✗ tcp          CONNECTION_REFUSED
  ✗ 127.0.0.1: dial tcp 127.0.0.1:1: connect: connection refused

  what it means:
    target host received our SYN but responded with RST.
    either no process is listening on this port, or a host-level
    firewall is actively rejecting connections.
  raw error: dial tcp 127.0.0.1:1: connect: connection refused

  what to check:
    • target host not listening (check: lsof -i :PORT on target)
    • OS firewall blocking (macOS App Firewall, ufw, Windows Defender)
    ↳ run `jdan net selfcheck :PORT` on the target host to investigate

✗ failed at tcp · total 287µs
```

#### ErrorClass 分类清单

probe 把失败按 **协议层 + 用户视角语义** 分类，避免你看 Go 内部错误字符串猜原因。完整的 class 表：

| 阶段 | Class | 含义 |
|------|-------|------|
| **resolve** | `DNS_NO_SUCH_HOST` | 域名不存在 |
| | `DNS_RESOLVER_UNREACHABLE` | DNS server 连不上 |
| | `DNS_TIMEOUT` | DNS 查询超时 |
| **tcp**（建立连接失败） | `CONNECTION_REFUSED` | 收到 RST：端口无人 listen / 防火墙 reject |
| | `CONNECTION_TIMEOUT` | SYN 无回应：防火墙静默 drop |
| | `NO_ROUTE_TO_HOST` | 你说的"链路不通"：路由器返回 unreachable |
| | `NETWORK_UNREACHABLE` | 本地网络 down / 无默认路由 |
| **tcp_health**（被远程关闭）| `REMOTE_RESET_AFTER_CONNECT` | TCP 建好后立刻 RST：**stateful firewall / IPS / 反爬** |
| | `REMOTE_CLOSED_AFTER_CONNECT` | 被 FIN：服务在 drain / 协议不匹配 |
| **tls** | `TLS_CERT_INVALID` | 自签 / 过期 / SAN 不匹配 |
| | `TLS_HANDSHAKE_FAIL` | 协议错位 / 中间人切断 |
| | `TLS_PLAIN_HTTP_ON_TLS_PORT` | 用 https:// 访问到 plain HTTP 服务 |
| **http** | `HTTP_4XX` / `HTTP_5XX` | 应用层错误（连接本身健康） |
| | `HTTP_PROTOCOL_ERROR` | 协议级失败 |

**分类算法**（class.go）：优先用 `errors.Is(err, syscall.ECONNREFUSED)` 等 errno 比对（跨 Go 版本最稳定），其次 `net.Error.Timeout()` 接口，最后字符串关键词兜底。

#### tcp_health 阶段：检测"被远程立刻关"

TCP 三次握手成功后，**默认 hold 1s 不发数据，看远端是否会主动 RST/FIN**。这是普通 curl 看不出来的语义——curl 只会显示 "connection reset"，分不清是 TCP 建好就被踢，还是发出 HTTP request 后被踢。tcp_health 把第一种情况单独归类成 `REMOTE_RESET_AFTER_CONNECT`，常见于：

- **反爬虫 / 安全设备**（CDN WAF、IPS）在 SYN-ACK 后基于 source IP 做 policy 判定再 RST
- **云 LB 健康检查失败** 导致流量被 drop
- **反向代理 IP allowlist** 拒绝你的源 IP

tcp_health 也识别 **server banner**（SSH/SMTP/POP3 在 accept 后立刻发欢迎行）：

```
✓ tcp_health   server pushed banner (12 bytes): SSH-2.0-OpenSSH_8.0
```

banner 不算错误——你 probe 的目标本来就不是 HTTP 服务。

#### flags

| flag | 默认 | 作用 |
|------|------|------|
| `--timeout` | 10s | 单阶段超时 |
| `--resolver` | 系统 | 指定 DNS server（`host[:port]`） |
| `--method` | HEAD | HTTP 方法；405 时自动 fallback GET 一次 |
| `-k` / `--insecure` | false | 跳过 TLS 证书验证 |
| `-v` / `--verbose` | false | 显示 cert chain + 所有响应 header |
| `--json` | false | 结构化输出（含 Class / Explanation / Hint 字段）|
| `--no-health` | false | 跳过 tcp_health 阶段（节省 1s，脚本场景） |
| `--health-duration` | 1s | tcp_health 阶段 hold 时长 |

**支持的 target 形态**：

| 形态 | 推断 |
|------|------|
| `https://github.com` | https + 443 |
| `example.com` | https + 443（无 scheme 默认 https） |
| `example.com:80` | http + 80（端口推断 scheme） |
| `192.168.1.42:8080` | http + 8080 |
| `[::1]:8080` | IPv6 literal |

**设计要点**：

- **逐 IP 串行 TCP connect**，不用 Go 默认的 Happy Eyeballs。探查工具的核心价值就是显示每个 IP 的具体结果（IPv4 通但 IPv6 不通这种问题用 Happy Eyeballs 会被隐藏）
- **HEAD 默认**，405 时自动 fallback 到 GET（很多服务器不支持 HEAD）
- **errno-based 错误分类**：`errors.Is(err, syscall.ECONNREFUSED)` 这类比字符串关键词匹配跨 Go 版本稳定，字符串匹配作为兜底
- **cross-reference 到 `jdan net selfcheck`**：连不上时引导用户去服务端跑自检
- 退出码恒为 0（probe 命令本身正常）；要识别"probe 是否通过"用 `--json` 看 `.ok` 字段

### `jdan net selfcheck`

服务端视角的诊断："我作为 server 该不该被外部访问？"和 `jdan net probe` 配对：probe 在客户端发现连不上时，hint 会让用户去服务端跑 `jdan net selfcheck :PORT`。

```
$ jdan net selfcheck :8080

◇ os & firewall
  • darwin/arm64
  ⚠ Application Firewall: ON
    macOS App Firewall is ON. unsigned binaries (like jdan) may be blocked.
      fix:
        sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add $(which jdan)
        sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp $(which jdan)

◇ network interfaces
    lo0 (loopback)
      127.0.0.1/8
    en0 (LAN)
      192.168.10.16/24
  ★ utun1024 (primary)
      198.18.0.1/30

◇ listening on :8080
  ✓ jdan (pid 29377, user quincy) bind=127.0.0.1:8080 (localhost-only)

◇ self-loop test
  ✓ http://127.0.0.1:8080 in 1ms

◇ prediction
  port :8080 is bound to loopback only (127.0.0.1 or ::1).
    external clients CANNOT reach this. server must bind 0.0.0.0 or specific LAN IP.
```

**它做的检查**：

| 检查 | 方式 |
|------|------|
| OS / 架构 | `runtime.GOOS` / `runtime.GOARCH` |
| macOS 防火墙状态 | exec `socketfilterfw --getglobalstate`（复用 `internal/sysprobe`） |
| 网络接口列表 | `net.Interfaces()` + 标 LAN / loopback / **★ primary**（默认路由出口） |
| 端口监听情况（带端口时） | exec `lsof -iTCP:PORT -sTCP:LISTEN` 看进程、PID、用户、bind 地址 |
| `bind` 是 LAN-reachable 还是 localhost-only | 区分 `0.0.0.0` / `*` / 具体 LAN IP（可达）vs `127.0.0.1` / `::1`（只本机） |
| Self-loop 测试 | HTTP GET `http://localhost:PORT` 和 `http://<primary LAN IP>:PORT` |
| Prediction | 综合上面所有，给一句"外部客户端能/不能访问"的判断 + 修复路径 |

**CLI**：

```bash
jdan net selfcheck                 # 通用诊断（不查具体端口）
jdan net selfcheck 8080            # 显式端口
jdan net selfcheck :8080           # 同上（冒号可有可无）
jdan net selfcheck 8080 --json     # 结构化输出
```

**prediction 的几种典型场景**：

| 状况 | prediction 怎么说 |
|------|------|
| firewall off + bind 0.0.0.0 + 自连通 | "LAN-reachable from self. external clients should reach ..." |
| firewall ON + bind 0.0.0.0 | "LAN-reachable, BUT firewall is on; clients may see 'connection refused', apply fix above" |
| bind 127.0.0.1 | "bound to loopback only ... external clients CANNOT reach this. server must bind 0.0.0.0" |
| 端口上没人 listen | "nothing is listening on :PORT. start your server first." |
| lsof 不存在 | "can't determine if anyone is listening on :PORT (install lsof to enable)." |

**依赖**：

- macOS / 主流 Linux 默认带 `lsof`。Alpine 等极简环境可能没，selfcheck 会优雅降级提示 `install lsof`
- 只 macOS 有真正的应用层防火墙检测；Linux/Windows 暂不实现（iptables/ufw/Defender 语义差异大）

### `jdan ssl cert`

看一个 HTTPS host 的证书详情：完整 chain + trust/hostname/expiry 三项验证 + OCSP 吊销状态查询。**用途**：

- 看 cert 还有多久过期（带进度条）
- 看 cert 包了哪些域名（SAN）
- 看完整 chain 排查 missing intermediate
- 看 fingerprint 给 cert pinning 用
- 看本地 PEM 文件（不联网）
- 监控脚本：`--expires-in 30d` 触发 exit 1

```
$ jdan ssl cert github.com

╭─ leaf ──────────────────────────────────────────────────────────────╮
│ Subject:    CN=github.com                                          │
│ Issuer:     CN=Sectigo Public Server Authentication CA DV E36,...  │
│ Valid:      2026-05-05 → 2026-08-02  (89d total)                   │
│ Days left:  █████░░░░░  50 days                                    │
│ SAN:        github.com, www.github.com                             │
│ Key:        EC P-256                                               │
│ Signed:     ECDSA-SHA256                                           │
│ Serial:     e7:ce:cc:3b:13:fb:3b:7b:8a:46:ea:8c:d0:ae:b7:1c        │
│ SHA256:     a7:b8:10:34:cd:43:95:51:c...9e:12:85:6c:85:5b:64:b6:5f │
╰────────────────────────────────────────────────────────────────────╯

Chain:
  ▸ leaf:        CN=github.com  (exp in 50d)
  ▸ intermediate: CN=Sectigo Public Server Authentication CA DV E36  (exp in 3569d)
  ▸ root:        CN=Sectigo Public Server Authentication Root E46  (exp in 7221d, self-signed)

Verification:
  ✓ chain trusted (system trust store)
  ✓ hostname matches SAN
  ✓ not expired

OCSP:
  ✓ CN=github.com  OCSP good
  ✓ CN=Sectigo Public Server Authentication CA DV E36  OCSP good
```

**flags**：

| flag | 默认 | 作用 |
|------|------|------|
| `-f` / `--file` | 无 | 从本地 PEM 文件读，不联网 |
| `--sni` | host | TLS 握手发的 SNI（虚拟主机场景） |
| `--full` | false | 展开 extensions / KeyUsage / OCSP URL 等 |
| `--json` | false | 结构化输出（含 Verification + OCSP 字段） |
| `--pem` | false | 输出标准 PEM 给管道 |
| `--no-ocsp` | false | 跳过 OCSP（节省 ~300-500ms） |
| `--timeout` | 5s | 整体超时 |
| `--expires-in` | 无 | 如 `30d` / `720h`，leaf 在此期内过期则 exit 1 |

**关键设计**：

- **`InsecureSkipVerify` 取 cert，但单独 verify**：要"看证书"就不能因为 cert 不可信直接拒绝。fetch 阶段无视信任拿到完整 chain，verify 阶段单独跑系统 trust store + hostname + expiry，结果当 report 显示给用户
- **errno-based OCSP**：用 `golang.org/x/crypto/ocsp`（quasi-stdlib）；cert 没 OCSP responder URL 时静默跳过（root cert 常见情况）；网络失败带 `⚠` 警告但不拒绝命令
- **过期倒计时进度条**：`█████░░░░░  50 days`——一眼看出"这 cert 还活着多久"，比 `openssl x509 -text -noout` 那一坨 ASCII 友好
- **过期检测脚本场景**：`--expires-in 30d` 让监控脚本能 `if ! jdan ssl cert host --expires-in 30d; then alert; fi`
- **复用 internal/sslcert/ package**：`internal/netprobe/tls.go` 未来可升级用同一套 Describe 出 SAN，零额外工作量
- **不做 OCSP stapling**（从 TLS 握手抓 stapled response）：复杂、覆盖率低，直查 OCSP responder 更稳；**CRL 不做**：大文件、场景窄
- **DSA 算法识别**：现代 cert 几乎不用 DSA，落到 `PublicKeyAlgorithm.String()` fallback 即可

**有意不做**：

- `jdan ssl diff a b` 对比两个 host 的 cert
- `jdan ssl watch` 持续监控
- `jdan ssl ct` 查 Certificate Transparency log
- CRL revocation 检查（用 OCSP 就够）
- OCSP stapling 解析

### `jdan ssl scan`

TLS 配置综合审计：对一个 HTTPS host 做 5 大块检查（版本 / cipher / ALPN / HSTS / session 重用 / cert），按 ssllabs 风格 5 维度加权给出 A+/A/B/C/D/F 评分。**用途**：

- 替代 ssllabs.com 在**内网 / 私有 host** 的本地能力
- CI/CD 安全门禁：`--grade-only` 输出 grade 字母，C 以下 exit 1
- 运维快速回答"我这 server 配置安全吗"
- 升级 TLS 配置后前后对比

```
$ jdan ssl scan github.com

╭─ TLS Versions ─────────────────────────────────────────────╮
│ ✗ TLS 1.0   refused    (recommended off)                 │
│ ✗ TLS 1.1   refused    (recommended off)                 │
│ ✓ TLS 1.2   supported                                    │
│ ✓ TLS 1.3   supported (preferred)                        │
╰──────────────────────────────────────────────────────────╯

╭─ Cipher Suites (TLS 1.2) ──────────────────────────────────╮
│ ✓ TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384    (strong)      │
│ ✓ TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305   (strong)      │
│ ✓ TLS_RSA_WITH_AES_256_GCM_SHA384  (acceptable; no forward sec) │
│                                                          │
│ Weak ciphers correctly refused:                          │
│   ✓ TLS_RSA_WITH_3DES_EDE_CBC_SHA refused                │
│                                                          │
│ TLS 1.3 ciphers are mandatory (5 fixed suites); not enumerated │
╰──────────────────────────────────────────────────────────╯

╭─ HTTP Stack ───────────────────────────────────────────────╮
│ ALPN:    h2, http/1.1                                    │
│ HSTS:    max-age=31536000; includeSubdomains; preload    │
│          strength=preload, max-age=31536000              │
╰──────────────────────────────────────────────────────────╯

╭─ Cert ─────────────────────────────────────────────────────╮
│ Subject:    CN=github.com                                │
│ Key:        EC P-256                                     │
│ Days left:  49                                           │
│ Chain:      trusted ✓                                    │
│ Hostname:   matches SAN ✓                                │
╰──────────────────────────────────────────────────────────╯

Overall: A+  (100/100)

Strong points:
  ✓ certificate trusted and valid
  ✓ TLS 1.3 supported
  ✓ TLS 1.3 enforces forward secrecy
  ✓ 6 modern cipher(s) supported (AES-GCM/ChaCha20)
  ✓ HSTS with preload (1y + subdomains + preload list)
  ✓ HTTP/2 supported via ALPN
```

**评分逻辑**（借鉴 ssllabs SSL Server Test）：

| 维度 | 权重 | 评判 |
|------|------|------|
| Cert | 25 分 | trusted + valid + key ≥ 2048 + sig ≠ SHA1 |
| Protocol | 30 分 | TLS 1.3 +30 / 1.2 +20 / 1.1 -15 / 1.0 -20 |
| Key Exchange | 25 分 | Forward Secrecy（ECDHE / DHE） |
| Cipher Strength | 20 分 | RC4/DES/3DES 减分；AES-GCM/ChaCha20 加分 |
| Modifiers | bonus | HSTS preload +5 / HSTS good +3 / H2 +2 / resume +1 |

映射：90+ A+ / 80+ A / 65+ B / 50+ C / 35+ D / < 35 F

**flags**：

| flag | 默认 | 作用 |
|------|------|------|
| `--sni` | host | TLS 握手发的 server_name |
| `--full-cipher` | false | 试 40 个 cipher 而不是 16 个常见（更慢） |
| `--no-cipher` | false | 跳过 cipher 枚举（最快） |
| `--no-hsts` | false | 跳过 HSTS HTTP GET |
| `--no-resume` | false | 跳过 session resumption 测试 |
| `--json` | false | 结构化输出 |
| `--grade-only` | false | 只输出 grade 字母；C 以下 exit 1（CI/CD 用） |
| `--timeout` | 15s | 整体超时 |

**设计要点**：

- **逐版本独立握手**：用 `MinVersion=MaxVersion` 强制单一版本，server 失败 = 不支持。比"询问 server 支持列表"更可靠
- **TLS 1.3 cipher 不枚举**：协议规定 mandatory 5 个固定 suite，没意义
- **不做 SSL 3.0**：Go stdlib 已移除，且生产环境已绝迹
- **不做密码学评估**：用静态分类表（RC4/DES = weak, AES-GCM = strong）。jdan 不是密码学审计工具，是配置审计
- **HSTS 通过 HTTPS GET 抓 header**：失败不影响 grade（标 "not configured"）
- **CI/CD 门禁**：`--grade-only` 让 `if ! jdan ssl scan host --grade-only; then alert; fi` 一行接入监控
- **复用 internal/sslcert/**：cert 块用同一套 fetch + Describe，零额外代码

**有意不做**：

- SSL Labs 那种公网测试 + 缓存共享
- 真实密码学算法强度评估
- Certificate Transparency log 查询
- Client cert / mTLS 测试
- HTTP/3 (QUIC) 支持（QUIC 走 UDP 不在 TCP+TLS 范围）

### `jdan ssl pin`

生成 cert pinning 用的 SPKI hash，配合主流 cert pinning 格式：**OkHttp (Android)** / **iOS NSAppTransportSecurity** / **HPKP HTTP header** / **Mozilla NSS** / **curl `--pinnedpubkey`** / 原始 base64。

#### ⚠ 重要：SPKI hash ≠ cert fingerprint

cert pinning **不能用 cert fingerprint**（即 `jdan ssl cert` 显示的 SHA256），必须用 **SPKI hash**：

| 概念 | 公式 | 用途 |
|------|------|------|
| Certificate fingerprint | `SHA256(cert.Raw)` | cert 完整内容 hash |
| **SPKI hash** | `SHA256(cert.RawSubjectPublicKeyInfo)` | **cert pinning 用这个** |

cert 经常 renew（同 key），renew 后 cert fingerprint 变了，**pinning 就坏**；SPKI hash 在 key 不变时 **stable**。HPKP RFC 7469 / Chrome static pins / iOS Apple Doc / Android Network Security Config 全部统一用 SPKI hash。

#### 默认 pin leaf + 第一个 intermediate

Apple / Android / Chromium static pins 推荐 best practice：
- **leaf hash** 让 pin 精准
- **intermediate hash** 让 cert renew 仍能匹配（renew 通常 issuer 不变）

`--leaf-only` 选退到只 leaf；`--full` chain 里所有 cert 都算。

#### 输出样例

```
$ jdan ssl pin github.com

╭─ Leaf ─────────────────────────────────────────────────────
│ Subject:    CN=github.com
│ Issuer:     CN=Sectigo Public Server Authentication CA DV E36
│ SPKI hash:  Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=
╰──────────────────────────────────────────────────────────

╭─ Intermediate ─────────────────────────────────────────────
│ Subject:    CN=Sectigo Public Server Authentication CA DV E36
│ Issuer:     CN=Sectigo Public Server Authentication Root E46
│ SPKI hash:  ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=
╰──────────────────────────────────────────────────────────

─── Pin formats ─────────────────────────────────────────────

▸ okhttp:
    CertificatePinner.Builder()
      .add("github.com", "sha256/Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=")
      .add("github.com", "sha256/ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=")
      .build()

▸ ios:
    <key>NSAppTransportSecurity</key>
    <dict>
      <key>NSPinnedDomains</key>
      <dict>
        <key>github.com</key>
        <dict>
          <key>NSIncludesSubdomains</key>
          <true/>
          <key>NSPinnedCAIdentities</key>
          <array>
            <dict>
              <key>SPKI-SHA256-BASE64</key>
              <string>Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=</string>
            </dict>
            ...

▸ hpkp:
    Public-Key-Pins: pin-sha256="Ry0vLQc..."; pin-sha256="ZSagvDz..."; max-age=5184000; includeSubDomains

▸ nss:
    pin-sha256:Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=
    pin-sha256:ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=

▸ curl:
    curl --pinnedpubkey 'sha256//Ry0vLQc...=;sha256//ZSagvDz...=' https://github.com/

▸ raw:
    Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=
    ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=
```

#### CLI 用法

```bash
jdan ssl pin github.com                        # 默认所有 6 种格式
jdan ssl pin example.com:8443 --format okhttp  # 只 OkHttp，给管道用
jdan ssl pin example.com --leaf-only           # 只 leaf SPKI
jdan ssl pin example.com --full                # chain 里所有 cert
jdan ssl pin -f cert.pem                       # 本地 PEM 文件
jdan ssl pin example.com --json                # 结构化输出
```

#### flags

| flag | 默认 | 作用 |
|------|------|------|
| `-f` / `--file` | 无 | 本地 PEM 文件 |
| `--sni` | host | TLS SNI |
| `--format` | 全部 6 个 | 单一格式：`okhttp` / `ios` / `hpkp` / `nss` / `curl` / `raw` |
| `--leaf-only` | false | 只算 leaf SPKI |
| `--full` | false | chain 所有 cert |
| `--json` | false | 结构化输出含 `entries` + `formats` 两段 |
| `--timeout` | 5s | TLS 握手超时 |

`--leaf-only` 和 `--full` 互斥；其他 flag 可组合。

#### 验证算法正确性

我们的 SPKI hash 跟 OpenSSL 算的等价：

```bash
# OpenSSL 算 SPKI hash 的标准 pipeline
openssl x509 -in cert.pem -pubkey -noout |
  openssl pkey -pubin -outform DER |
  openssl dgst -sha256 -binary | base64

# jdan 输出应当 byte-equal
jdan ssl pin -f cert.pem --format raw --leaf-only
```

测试用 `crypto/x509.MarshalPKIXPublicKey` 独立计算等价 SPKI hash，确保两者 byte 相同（覆盖 RSA / EC / Ed25519 三种 key 类型）。

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

