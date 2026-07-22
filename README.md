# jdan

**简体中文** | [English](README.en.md)

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

## Shell 自动补全

命令多了，装上补全 `<Tab>` 起来很省事：补子命令名、flag 名，部分 flag 还能补**值**（`hash --algo <Tab>` → md5/sha1/sha256/sha512、`ascii-art --ramp <Tab>` → standard/detailed/blocks、`dns lookup --type`、`cal` 月份、`--doh` 动态 provider 别名等）。基于 cobra,`jdan completion <shell>` 直接生成:

```bash
# zsh（确保 ~/.zshrc 里有 compinit）
jdan completion zsh > "${fpath[1]}/_jdan"   # 然后重开终端或跑 compinit

# bash（需 bash-completion）
jdan completion bash | sudo tee /etc/bash_completion.d/jdan >/dev/null

# fish
jdan completion fish > ~/.config/fish/completions/jdan.fish

# powershell（写进 $PROFILE）
jdan completion powershell | Out-String | Invoke-Expression
```

试一把当前 session（zsh）：`source <(jdan completion zsh)`。

## 命令

按主题分组的目录（实际章节顺序按命令引入时间排列，所以网络类和文件类不连续）：

**网络 & DNS**
- [`jdan http timing`](#jdan-http-timing) — 测 HTTP 请求各阶段耗时
- [`jdan http headers`](#jdan-http-headers) — 看响应头 + 完整重定向链（逐跳）
- [`jdan http grade`](#jdan-http-grade) — 给站点安全响应头打分（A+~F，securityheaders 风格）
- [`jdan http serve`](#jdan-http-serve) — 临时静态文件服务器 + LAN URL + 终端二维码
- [`jdan net probe`](#jdan-net-probe) — 客户端视角逐阶段（DNS/TCP/TLS/HTTP）探查
- [`jdan net selfcheck`](#jdan-net-selfcheck) — 服务端自检 + 外部访问预测
- [`jdan net cdn`](#jdan-net-cdn) — 识别站点前面挂的 CDN/WAF（Cloudflare/阿里云/百度/腾讯/京东/CloudFront/Akamai/Fastly 等）
- [`jdan net ws`](#jdan-net-ws) — 探测 WebSocket 端点（握手 + ping/pong 往返）
- [`jdan ssl cert`](#jdan-ssl-cert) — 看 HTTPS 证书详情（chain / verification / OCSP）
- [`jdan ssl scan`](#jdan-ssl-scan) — TLS 配置综合审计（ssllabs 风格 A+/A/B/C/D/F 评分）
- [`jdan ssl pin`](#jdan-ssl-pin) — 生成 cert pinning 用的 SPKI hash（6 种格式）
- [`jdan cert`](#jdan-cert) — 生成本地开发用自签名 TLS 证书（造 → 看闭环）
- [`jdan pem`](#jdan-pem) — 离线检视 PEM 文件（证书/CSR/私钥/公钥；key↔cert 匹配；不打印私钥）
- [`jdan ssh-key`](#jdan-ssh-key) — SSH 密钥解析（info / fingerprint / pubkey，对齐 ssh-keygen）
- [`jdan dns lookup`](#jdan-dns-lookup) — 并发查询 6 个 record type，含 DoH 支持
- [`jdan dns reverse`](#jdan-dns-reverse) — IP → 域名（PTR 查询）
- [`jdan dns trace`](#jdan-dns-trace) — 从根服务器迭代解析，看委派路径
- [`jdan ping`](#jdan-ping) — ping，可用 `--dns` 指定解析域名用的 DNS server
- [`jdan pubip4`](#jdan-pubip4--jdan-pubip6) / [`jdan pubip6`](#jdan-pubip4--jdan-pubip6) — 查本机公网 IP
- [`jdan ports`](#jdan-ports) — 显示本机正在监听的端口（macOS）

**文件 & 归档**
- [`jdan file bak`](#jdan-file-bak) — 给文件打带时间戳的备份
- [`jdan zip`](#jdan-zip) — 把文件或目录打成 `.zip`
- [`jdan tree2`](#jdan-tree2) — 多列展示两层目录树
- [`jdan disk`](#jdan-disk) — 磁盘使用一览（各挂载点容量/占用，df 式）
- [`jdan size`](#jdan-size) — 目录体积排行（谁吃了空间；带占比条形图）
- [`jdan wifi`](#jdan-wifi) — WiFi 状态与信道拥挤分析（该换到哪个信道）
- [`jdan readme`](#jdan-readme) — 输出指定目录的 README.md（带 bat 高亮）

**系统**
- [`jdan macgpu`](#jdan-macgpu) — Apple Silicon GPU TUI 监控
- [`jdan unix-time`](#jdan-unix-time) — Unix 时间戳 → 本地时间
- [`jdan cal`](#jdan-cal) — 打印月/年日历（高亮今天，周一起始）
- [`jdan lunar`](#jdan-lunar) — 公历 ↔ 农历（干支/生肖/农历节日）

**随机生成（CSPRNG）**
- [`jdan rand password`](#jdan-rand-password) — 1Password 风格随机密码
- [`jdan rand uuid`](#jdan-rand-uuid) — UUID v4 / v7
- [`jdan uuid`](#jdan-uuid) — 检视 UUID（版本/variant/v1·v7 时间戳/字节）
- [`jdan rand hex`](#jdan-rand-hex--base64--base64url--base32) / [`base64`](#jdan-rand-hex--base64--base64url--base32) / [`base64url`](#jdan-rand-hex--base64--base64url--base32) / [`base32`](#jdan-rand-hex--base64--base64url--base32) — 字节级随机 + 编码
- [`jdan rand alnum`](#jdan-rand-alnum) — 字母数字串（无类约束）
- [`jdan rand int`](#jdan-rand-int) — 闭区间随机整数
- [`jdan rand word`](#jdan-rand-word) — EFF diceware passphrase

**测试数据**
- [`jdan fake`](#jdan-fake) — 生成像真实数据的假值（name/email/uuid/date/ip…），`--seed` 可复现

**Git**
- [`jdan git summary`](#jdan-git-summary) — 仓库一眼看（commit/分支/tag/年龄/贡献者/hotspots）
- [`jdan git changelog`](#jdan-git-changelog) — 从最近 tag 到 HEAD 生成 changelog（Conventional Commits 分组）
- [`jdan git commitlint`](#jdan-git-commitlint) — 按 Conventional Commits 规范校验提交信息
- [`jdan git secrets`](#jdan-git-secrets) — 扫 git 历史里提交过的密钥/凭据（底层 gitleaks，默认脱敏）

**文档 / Markdown**
- [`jdan toc`](#jdan-toc) — 从 Markdown 标题生成目录（GitHub 风格 anchor，可回填）

**集成**
- [`jdan obsidian install-claudian`](#jdan-obsidian-install-claudian) — 装 Claudian Obsidian 插件

**编码 & 二维码**
- [`jdan qr`](#jdan-qr) — 生成二维码（终端 / PNG / SVG）
- [`jdan qrwifi`](#jdan-qrwifi) — 生成 WiFi 入网二维码（扫码即连，自动转义）
- [`jdan barcode`](#jdan-barcode) — 生成 Code128 一维条码（终端 / PNG / SVG）
- [`jdan figlet`](#jdan-figlet) — 文字 → ASCII art 大横幅（standard / block 字体）
- [`jdan morse`](#jdan-morse) — 文本 ↔ 摩斯电码（ITU，自动判方向）
- [`jdan alpha`](#jdan-alpha) — 字母表 ↔ 序号对照（A1Z26；表格 + 单向查询）
- [`jdan pinyin`](#jdan-pinyin) — 中文 → 拼音（多种声调样式；t9/sp 的共同第一步）
- [`jdan strokes`](#jdan-strokes) — 查汉字笔画数（逐字 + 总数；数据来自 Unicode Unihan）
- [`jdan trad`](#jdan-trad) — 简繁转换（词汇级：发→發/髮 消歧、软件→軟體 地区词；数据来自 OpenCC）
- [`jdan sijiao`](#jdan-sijiao) — 查汉字四角号码（王云五检字法；数据来自 Unicode Unihan）
- [`jdan cangjie`](#jdan-cangjie) — 查汉字仓颉码（含字根，如 明 AB 日月；数据来自 Unicode Unihan）
- [`jdan t9`](#jdan-t9) — 中文/英文 → 九宫格(T9)按键序列（汉字按拼音）
- [`jdan spt9`](#jdan-spt9) — 中文 → 小鹤双拼九宫格按键（每字 2 键）
- [`jdan sp`](#jdan-sp) — 中文 → 26 键双拼按键（多方案 + `--all` 对比）
- [`jdan img`](#jdan-img) — 读图片文件头报尺寸/格式/颜色/大小（PNG/JPEG/GIF）
- [`jdan ascii-art`](#jdan-ascii-art) — 图片 → ASCII 字符画（可选真彩）
- [`jdan mime`](#jdan-mime) — 按 magic bytes 判断文件真实类型（不看扩展名）
- [`jdan jwt decode`](#jdan-jwt-decode) — 纯本地 JWT 解码（不验签、不联网）
- [`jdan jwt verify`](#jdan-jwt-verify) — HMAC 校验 JWT 签名（HS256/384/512，防 alg-confusion）
- [`jdan totp`](#jdan-totp) — TOTP 2FA 验证码（RFC 6238，兼容 Google Authenticator）
- [`jdan b64 enc/dec`](#jdan-b64) — base64 编码/解码（standard / URL-safe / no-pad）
- [`jdan url enc/dec`](#jdan-url) — URL percent-encoding
- [`jdan grab`](#jdan-grab) — 从文本捞 URL / email / IP（松正则 + stdlib 校验）
- [`jdan num`](#jdan-num) — 进制转换（dec/hex/bin/oct）+ 位运算
- [`jdan calc`](#jdan-calc) — 算术表达式计算器（+ - * / % ^ + 函数）
- [`jdan env`](#jdan-env) — .env 文件工具（lint / diff / redact / get）

**JSON / YAML / CSV**
- [`jdan json`](#jdan-json) — pretty/minify/path/keys/diff/lines/flatten/merge + yaml ↔ json + csv ↔ json

**网络 / 查询**
- [`jdan whois`](#jdan-whois) — 域名/IP WHOIS（自动路由 + IANA/ARIN referral 跟随 + parsed 表）
- [`jdan ip`](#jdan-ip) — IP / CIDR 计算（info / contains / range / range-cidr / split / aggregate / normalize）
- [`jdan meta`](#jdan-meta) — 抓网页 meta / Open Graph / Twitter Card（分享卡片体检）
- [`jdan csp`](#jdan-csp) — 解析 Content-Security-Policy + 安全体检
- [`jdan cookie`](#jdan-cookie) — 解析 Set-Cookie / Cookie + 安全体检

**文件 hash & 归档**
- [`jdan hash`](#jdan-hash) — 跨平台 md5/sha1/sha256/sha512 + `--check` 校验
- [`jdan entropy`](#jdan-entropy) — 算 Shannon 熵（判断是否加密/压缩/随机；滑窗 sparkline）
- [`jdan secrets-scan`](#jdan-secrets-scan) — 扫硬编码密钥/token（正则 + 高熵；输出脱敏）
- [`jdan pwned`](#jdan-pwned) — 查密码是否已泄露（HIBP k-匿名，密码不出本机）
- [`jdan htpasswd`](#jdan-htpasswd) — 生成/校验 Basic Auth 密码哈希（bcrypt/apr1/SHA）
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

### `jdan qrwifi`

生成 **WiFi 入网二维码**：手机相机 / 微信扫一下直接连网，不用念密码。是 `jdan qr` 的「特定 payload」封装——**自动转义 SSID/密码里的 `\ ; , " :`**（手搓 `WIFI:` 串最容易漏的一步，漏了码就是错的、扫了静默连不上），渲染完全复用 `qr` 管线，**0 新依赖**。

详细技术文档：[docs/jdan-qrwifi.md](docs/jdan-qrwifi.md)

```bash
$ jdan qrwifi MyNetwork -p 's3cr3t'              # 终端二维码
$ jdan qrwifi --ssid "Cafe Guest" --auth nopass  # 开放网络，无密码
$ jdan qrwifi Home -p pw --hidden                # 隐藏网络
$ jdan qrwifi Home -p pw -o wifi.png             # 存 PNG（贴墙上）
$ jdan qrwifi Home --password-stdin <<< 'pw'     # 密码走 stdin，不进 shell history
$ jdan qrwifi Home -p pw --json                  # {ssid,auth,hidden,payload,...}
```

payload 按 `WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;` 标准拼。认证类型 `wpa`（默认，含 WPA2/WPA3）/ `wep` / `nopass`（开放网络，省略 `P:`）。**WPA/WEP 忘了给密码会直接报错**（空密码的码扫了静默连不上），真·开放网络请显式 `--auth nopass`。SSID 可用位置参数或 `--ssid`。密码 `-p` 方便、`--password-stdin` 避免进 shell history（二维码本身会暴露密码,故只防 history/`ps` 这层）。渲染 flag（`--ecc`/`--invert`/`--full-block`/`--output .png/.svg`/`--json`）全继承 `qr`。**有意不做**企业级 802.1X / EAP（payload 复杂、扫码端支持差）和「读系统已存 WiFi 密码」（要抠各平台钥匙串，越权）。

### `jdan barcode`

生成 **Code128 一维条码**（库存 / 物流 / 快递单常用）。**0 新依赖**：内嵌 107 行 Code128 模式表自己编码 + 自己渲染（`image/png` 是 stdlib），不引外部 barcode 库——跟 `lunar` 一样的「内嵌表 + 算法」路子。

详细技术文档：[docs/jdan-barcode.md](docs/jdan-barcode.md)

```bash
$ jdan barcode "ABC-123"           # 终端竖条 + 下方人眼可读文本
$ jdan barcode 5901234123457 -o label.png
$ jdan barcode "SKU42" -o tag.svg
$ echo "data" | jdan barcode       # stdin
$ jdan barcode "ABC-123" --json    # {data, code_set, checksum, modules}
```

**原理**：一维条码 = 一排竖条+空白，靠宽度编码。Code128 共 107 个符号、每个 11 模块（3 条+3 空），结构 `[静区] Start 数据 校验 Stop [静区]`，校验 = `(Start + Σ位置×值) mod 103`。字符集默认 **B**（可打印 ASCII 32-126）；输入**全数字且偶数长度**时自动用 **C** 集（一个符号编 2 位数，宽度减半）。输出：终端（整列 █ 竖条 + 下方文本）/ PNG / SVG（`-o` 按扩展名），`--module` 调粗细、`--invert` 反色、`--no-text` 隐藏文本。**有意不做** EAN-13 / UPC / Code39（各有独立校验位规则，可后续单加）、中途切 code set 的最优压缩、条码**识别**（图→数字，要图像处理，跟 `qr` 没做解码同理）。注：PNG 不画人眼文本（要字体依赖），终端和 SVG 画。

### `jdan figlet`

把文字渲染成 ASCII art 大横幅（figlet 风格）。0 新依赖（内置字体）。跟 `jdan qr` 同属"把文字变成视觉输出"的家族。

详细技术文档：[docs/jdan-figlet.md](docs/jdan-figlet.md)

**用途**：给 CLI 输出加标题、section 分隔、README banner、终端 MOTD、脚本步骤提示。

```bash
$ jdan figlet "jdan"
  ### ####   ###  #   #
   #  #   # #   # ##  #
   #  #   # ##### # # #
#  #  #   # #   # #  ##
 ##   ####  #   # #   #

# 实心块字体
$ jdan figlet "OK" --font block
 ███  █   █
█   █ █  █
█   █ ███
█   █ █  █
 ███  █   █

$ jdan figlet Deploy OK            # 多 arg 拼接
$ jdan figlet "Title" --center --width 60
$ echo "Build Done" | jdan figlet  # stdin
$ jdan figlet --list               # 列出字体
```

字体 `standard`（`#` 描边）/ `block`（实心块 `█`）；覆盖 A-Z / a-z / 0-9 / 标点，小写折叠大写，不支持字符空白占位。`--width` 超长自动换行，`--center` 居中。

### `jdan morse`

文本 ↔ 国际摩斯电码（ITU）互转。**自动判方向**：输入只含 `.`/`-`/`/`/空格 → 解码，否则编码。学习/解谜/玩。0 新依赖（一张查表）。

详细技术文档：[docs/jdan-morse.md](docs/jdan-morse.md)

```bash
$ jdan morse "SOS"
... --- ...

$ jdan morse "... --- ..."          # 自动认出是摩斯码 → 解码
SOS

$ jdan morse "Hello World"
.... . .-.. .-.. --- / .-- --- .-. .-.. -..

$ jdan morse "E" --encode            # 极短/有歧义时强制方向（-d 强制解码）
```

字母间单空格、单词间 ` / `；大小写无关，解码输出大写。覆盖 A–Z / 0–9 + 标准标点。无法编码的字符（中文/emoji）跳过、无法识别的码出 `#`，计数走 stderr（stdout 保持干净可管道）。`--json` 给 `{direction, output}`。

### `jdan alpha`

字母表 ↔ 序号对照（A1Z26）。0 依赖（纯 stdlib）。

```
$ jdan alpha
a b c d e f g h i j  k  l  m  n  o  p  q  r  s  t  u  v  w  x  y  z
1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26

$ jdan alpha k         # 字母 → 序号
11
$ jdan alpha 11        # 序号 → 字母
k
$ jdan alpha -u        # 大写表
```

无参数打印字母行 + **列对齐在正下方**的序号行（每个字母正好落在它的序号上方）；带一个参数就单向查询：字母 → 序号 / 序号 → 字母。`-u` 用大写。越界（0/27）或非单字母报错。

### `jdan strokes`

查汉字笔画数，整句逐字列出并给总数——这是输入法给不了的（输入法只显示你正在打的那一个字）。数据是 Unicode Unihan 的 `kTotalStrokes`，离线查表，**0 新依赖**，覆盖全部 CJK 含扩展区（实测 U17 共 102,998 字），起名用的生僻字也查得到。

详细技术文档：[docs/jdan-strokes.md](docs/jdan-strokes.md)

```bash
$ jdan strokes 龙凤呈祥
龙 5 / 凤 4 / 呈 7 / 祥 10
共 26 画

$ jdan strokes 龙     # 5 画（简体）
$ jdan strokes 龍     # 16 画（繁体，不同码点不同画数）
$ jdan strokes 鑫龗   # 24 / 33（生僻字也查得到）
$ echo 龙凤呈祥 | jdan strokes    # 从 stdin 读
$ jdan strokes --json 龙凤
```

**只做笔画数，不做笔顺**：笔顺没有权威的开放机读数据（Unicode 无此字段，国标无开放机读版，开源数据都是从字体推导的近似值且不完全等同国标），给一个「看起来权威实则可能错」的笔顺比不给更糟。非汉字（字母/标点/emoji）跳过不计；表里查不到的汉字标为「未知」，总数不含它但会提示。

数据表用**排序 slice + 二分查找**而非 map 字面量（103k 条 map 字面量约 1.3 MB 源码、编译慢）。`kTotalStrokes` 在 `Unihan_IRGSources.txt`（Unicode 15 起从 DictionaryLikeData 移过去了），现全是单值，最大 84 画（U+3106C），uint8 够用。

### `jdan trad`

中文简↔繁转换。不止换字形，还**按词消歧**（同一个「发」在「头发」里是 髮、在「发展」里是 發）、可选**地区用词**（软件→軟體、网络→網路）。数据是 [OpenCC](https://github.com/BYVoid/OpenCC)（Apache-2.0）离线词典，`go:embed` 内嵌 9 个 `.txt`（~1.18MB），**0 新依赖**，算法是自写的前向最大匹配——跟 `lunar`/`strokes` 一样的「内嵌数据 + 自写算法」路子。

详细技术文档：[docs/jdan-trad.md](docs/jdan-trad.md)

```bash
$ jdan trad 头发和发展            # 頭髮和發展（同字不同繁，按词分）
$ jdan trad --to twp 软件网络     # 軟體網路（台湾用词）
$ jdan trad --to s 軟體           # 软体（繁 → 简）
$ echo 软件 | jdan trad --to twp  # 从 stdin 读（大输入逐行处理）
$ jdan trad --diff --to twp 软件和网络
「軟體」和「網路」
改动 2 处：
  软件 → 軟體
  网络 → 網路
$ jdan trad --json --to twp 软件
```

`--to`：`t`(默认，简→繁) / `tw`(台湾字形) / `twp`(台湾含用词) / `hk`(香港字形) / `s`(繁→简)。非汉字（字母/数字/标点/emoji）原样透传。

**为什么不能逐字换**：`发→發/髮`、`干→幹/乾/干`、`台→臺/颱/檯` 都是一简对多繁，必须看词。管线严格复刻 OpenCC 源码：多趟 conversion 顺序执行，词典组分 `union`（取最长）与 `short_circuit`（第一个命中的词典即停）两种语义。

**能力边界（诚实划界）**：简繁 + 地区词到 OpenCC 词典为止，不做全量翻译。`s2t/t2s` 与 OpenCC 逐字节一致；`tw/twp/hk` 未做 MMSEG 预分词，极少数跨词边界例可能与 OpenCC 有别；另有约 1.7% 台湾地区词（多为外国地名，如 索馬里/毛里塔尼亞）因缺 OpenCC 编译期生成词典而回转不到——均写进文档，量化见 `TestTWPhrasesReachability`。

### `jdan sijiao`

查汉字的**四角号码**（王云五检字法）。看字四个角的笔形取 4 位主码 + 附号 → `NNNN.N`。数据是 Unicode Unihan 的 `kFourCornerCode`，离线查表，**0 新依赖**——跟 `strokes` 同源同架构（同一份 Unihan、同一套 gen + 排序表 + 二分）。只做正查（字→码），反查（码→字）暂不支持。

详细技术文档：[docs/jdan-sijiao.md](docs/jdan-sijiao.md)

```bash
$ jdan sijiao 王              # 王  1010.4
$ jdan sijiao 你              # 你  2729.0, 2729.2   （多值，同字用逗号）
$ jdan sijiao 口业专          # 口 6000.0 / 业 3210 / 专 5030   （字与字用斜杠）
$ echo 王 | jdan sijiao        # 从 stdin 读
$ jdan sijiao --json 你口
```

**只能查表、不能算**：四个角的笔形要看字形，从码点算不出来（同笔顺）。表就是 `kFourCornerCode`，和 `strokes` 的 `kTotalStrokes` 一个模子。十类笔形口诀「横一 垂二 三点捺 叉四 插五 方框六 七角 八八 九是小 点下有横变零头」内置在 `--help` 里教方法，但**只给码、不做逐角分解**（Unihan 只存最终码）。

**能力边界**：覆盖约 1.69 万常用/传统字（深扩展区生僻字无码，表外汉字标「无」）；149 个多值字（如 `你`→两个码）**整串保留不截断**；四角号码是老检字法，受众小，价值在"离线 + 补检字线"。

### `jdan cangjie`

查汉字的**仓颉码**（朱邦復输入法，台/港主流），并把字母码翻成字根一并显示：`明 → AB（日月）`。数据是 Unicode Unihan 的 `kCangjie`（仓颉三代），离线查表，**0 新依赖**——跟 `strokes`（笔画）、`sijiao`（四角）是三胞胎（同一份 Unihan、同一套 gen + 排序表 + 二分）。只做正查（字→码），反查（码→字）暂不支持。

详细技术文档：[docs/jdan-cangjie.md](docs/jdan-cangjie.md)

```bash
$ jdan cangjie 明              # 明  AB（日月）
$ jdan cangjie 你              # 你  ONF（人弓火）
$ jdan cangjie 明变            # 明 AB（日月） / 变 YCE（卜金水）   （字与字用斜杠）
$ echo 明 | jdan cangjie        # 从 stdin 读
$ jdan cangjie --json 明你
```

**为什么能显示字根**：仓颉把字拆成 1-5 个字根、每根对应一个字母键（`明`=日+月=`AB`）。拆成哪几个根靠字形、算不出来（同笔顺），只能查 `kCangjie` 表；但**字母↔字根是固定 25 键映射**（A日 B月 … X難 Y卜，无 Z），所以能把 `AB` 翻成 `日月`——这是 `sijiao` 给不了的（四角只有最终码、无法逐角分解）。字根表正确性经变异测试守卫。

**能力边界**：仓颉三代（与五代等版本可能有出入）；覆盖约 2.9 万字（表外字标「无」）；全单值（比 sijiao 更简单）；仓颉主要台/港用，价值在"离线 + 补输入法/检字线 + 字根教学"。

### `jdan pinyin`

把中文转成**拼音**,多种声调样式,非汉字原样穿插。是 `jdan t9` / `sp` / `spt9` 的**共同第一步**单独成命令(它们内部都先「中文→拼音」再往键盘映射)。

```
$ jdan pinyin 中文输入法
zhōng wén shū rù fǎ

$ jdan pinyin "Hello 世界 2024" --style plain     # 非汉字穿插 + 无调
Hello shi jie 2024

$ jdan pinyin 银行 --heteronym                     # 多音字列全部
yín xíng/háng/héng/xìng/hàng
```

样式 `-s/--style`(默认 `tone` 带调符):`num`(`zhong1 wen2`)/`plain`(无调)/`initials`(声母 `zh w`)/`first`(首字母)。`--sep` 改分隔符、`--heteronym` 多音字、`--json` 逐字结构化。底层 go-pinyin ~4 万条 Unihan 读音(离线),`t9`/`sp`/`spt9` 的拼音基建已收敛到同一份 `internal/pinyinx`。**局限**:逐字取常见读音、不按词消歧(`银行` 的行默认可能不是 háng)。原理详见 [docs/jdan-pinyin.md](docs/jdan-pinyin.md)。

### `jdan t9`

把一段文字翻成**九宫格键盘（T9）实际要按的数字键**。汉字先转拼音再映射（中 → zhong → 94664），英文字母直接映射（hi → 44）。

```
$ jdan t9 "你好 hi 2024"
你    ni   64
好    hao  426
hi    —    44
2024  —    2024
─────
64 426 44 2024
```

键位 `2 abc / 3 def / 4 ghi / 5 jkl / 6 mno / 7 pqrs / 8 tuv / 9 wxyz`。每个汉字一行（字+拼音+数字），英文单词一行，阿拉伯数字原样透传，空格/标点静默跳过、其它无法映射的字符跳过并计数（走 stderr）。`--digits` 只出整串数字（可管道），`--json` 机读。

汉字→拼音这步是纯查表、不是逻辑（`jdan` 是离线二进制，读音必须自带字典），故用 `go-pinyin`（内嵌 ~4 万条 Unihan 读音，离线可用）。**局限**：多音字取最常见读音（如「行」默认 xíng，不按词组消歧），个别可能不准。原理详见 [docs/jdan-t9.md](docs/jdan-t9.md)。

### `jdan spt9`

把中文翻成【**小鹤双拼**】在九宫格上的按键 —— 每个字**固定按 2 下**（一键声母 + 一键韵母）。跟 `jdan t9`（全拼、每字不定长）互补。

```
$ jdan spt9 "你好世界 hi"
你  ni   ni  64
好  hao  hc  42
世  shi  ui  84
界  jie  jp  57
hi  —  —   44
─────
64 42 84 57 44
```

比 t9 多一步「拼音 → 小鹤双拼两码」：中 → zhong → 声母 zh=v、韵母 ong=s = `vs` → 键 8、7。**小鹤方案照 [RIME `rime-double-pinyin`](https://github.com/rime/rime-double-pinyin) 的 flypy 规则逐条写死**（声母 zh/ch/sh=v/i/u），不凭记忆，官方实例 `dan → dj → 键3+键5` 被单测钉死。同一句 `中国`：t9 全拼 `94664 486`，spt9 双拼 `87 46`。`--digits`/`--json`；英文按普通 T9、数字原样、标点跳过。**局限**：只做小鹤方案（米旮旯/辜氏等专用九键可日后 `--scheme` 扩）；多音字取常见读音。原理详见 [docs/jdan-spt9.md](docs/jdan-spt9.md)。

### `jdan sp`

把中文翻成**标准 26 键双拼**要按的字母键 —— 每字**恒 2 键**，并支持**多套方案** + `--all` 一次对比。跟 `jdan spt9`（九宫格双拼，出数字键）互补：这个是 26 键，出字母键。

```
$ jdan sp 中文输入法 --all
小鹤      vs wf uu ru fa
自然码    vs wf uu ru fa
微软      vs wf uu ru fa
智能ABC   as wf vu ru fa
拼音加加  vy wr iu ru fa
```

方案（`-s/--scheme`，默认小鹤）：小鹤 `flypy` / 自然码 `ziranma` / 微软 `mspy`（搜狗双拼=此布局，`-s sogou`）/ 智能ABC `abc` / 拼音加加 `pyjj`。**每套规则逐条照 [RIME `rime-double-pinyin`](https://github.com/rime/rime-double-pinyin) 各 schema 抄**，非凭记忆（用一个小解释器按序套用 xform 规则）。`jdan spt9` 的小鹤编码已改为复用同一份实现。同一句 `中国`：t9 `94664 486` → spt9 `87 46` → sp `vs go`。`--codes`/`--json`；英文按字母本身、数字原样、标点跳过。**局限**：零声母字按各方案 RIME 规则、个别或与你的输入法习惯略有出入；多音字取常见读音。原理详见 [docs/jdan-sp.md](docs/jdan-sp.md)。

### `jdan img`

只读图片**文件头**报出尺寸/格式/颜色模型/大小，不解码整张图（`image.DecodeConfig`，对大图也是常数级开销）。0 新依赖（纯 stdlib）。

详细技术文档：[docs/jdan-img.md](docs/jdan-img.md)

**支持**：PNG / JPEG / GIF（stdlib 解码器；WEBP/BMP/TIFF 需外部依赖，故不做）

```bash
$ jdan img logo.png
logo.png
  格式: PNG
  尺寸: 512 x 512
  颜色: NRGBA (含 alpha)
  大小: 24.3 KiB

# 多文件 → 对齐表格
$ jdan img hero.jpg thumb.jpg
hero.jpg   1920x1080  JPEG  340.0 KiB
thumb.jpg  320x180    JPEG   18.0 KiB

$ jdan img < logo.png         # stdin
$ jdan img *.png --json       # JSON 数组
```

批量里某个文件坏/不支持时打一行错误、继续处理其余文件，最后整体 exit 1（不让一个坏文件中断整批）。`--json` 即使全失败也输出合法空数组。

### `jdan ascii-art`

把图片渲染成 **ASCII 字符画**（像 jp2a）。复用已接好的 stdlib 图片解码，**0 新依赖**。是 `img`（只读尺寸）的「画出来」补充。

详细技术文档：[docs/jdan-ascii-art.md](docs/jdan-ascii-art.md)

```bash
$ jdan ascii-art logo.png            # 按终端宽度自动缩放
$ jdan ascii-art photo.jpg -w 60     # 指定列宽
$ jdan ascii-art logo.png --color    # 24-bit 真彩（仅 TTY）
$ jdan ascii-art logo.png --invert   # 反明暗（浅底终端）
$ cat x.png | jdan ascii-art         # stdin
```

算法：解码 → 按列宽切网格、每格**块平均**采样 → 亮度映射到字符 ramp，可选每字符 truecolor 染色。ramp（暗→亮）：`standard`（默认 10 级，width-1 安全）/ `detailed`（70 级）/ `blocks`（`░▒▓█`，但这些是 East-Asian-ambiguous，CJK 终端会按 2 列宽渲染、横向拉伸，会警告）/ 自定义串。默认**单色**（可粘进 README/注释、可管道）；`--color` 加真彩，仅 TTY、管道自动剥离。字符长宽比默认按 0.5 校正（终端字符约 2 倍高），避免纵向拉伸。格式 PNG/JPEG/GIF（GIF 取第一帧）；WebP/HEIC 不支持（需新依赖）。

### `jdan mime`

按文件**内容**（magic bytes）判断真实 MIME / content-type，**不看扩展名**——文件改了名也认得出。0 新依赖（纯 stdlib）。

详细技术文档：[docs/jdan-mime.md](docs/jdan-mime.md)

底座是 stdlib `http.DetectContentType`（~60 种），再加一层精选 magic 表补 stdlib 漏掉的开发格式（ELF / 7z / xz / zstd / bzip2 / tar / SQLite）。

```bash
$ jdan mime logo.png
image/png

# 多文件 → 对齐表格
$ jdan mime a.bin b.bin c.bin
a.bin   application/pdf
b.bin   application/zip
c.bin   text/plain; charset=utf-8

# 改名也认得出，并提示扩展名不符
$ mv photo.png weird.txt
$ jdan mime weird.txt
image/png   (扩展名 .txt 不符)

$ jdan mime < file.bin        # stdin
$ jdan mime *.bin --json      # JSON 数组
```

扩展名不符检测用内置的扩展名→类型表（OS 无关、可复现），故意不回退依赖系统 mime.types 的 stdlib。空文件 → `inode/x-empty`。批量里坏文件不中断整批，最后整体 exit 1。跟 `jdan img`（专看图片尺寸）互补。

### `jdan meta`

抓网页的 `<meta>` / **Open Graph** / **Twitter Card** / canonical / favicon，回答「这链接分享到微信/Twitter/Slack 时长啥样」，顺手做分享/SEO **体检**。复用 `x/net/html`（已在依赖图）+ 现有 http 栈，**0 新依赖**。

详细技术文档：[docs/jdan-meta.md](docs/jdan-meta.md)

```bash
$ jdan meta https://example.com/article
$ jdan meta example.com --json
$ cat page.html | jdan meta          # 离线解析本地 HTML
$ jdan meta page.html                # 解析本地文件
```

抓取约束：跟随重定向报最终 URL；非 `text/html` 拒绝；只读 `<head>` 区（封顶 512 KiB，不下整个大页面）；默认 10s 超时（`--timeout`）。默认伪装常见浏览器 UA（不少站对非浏览器 UA 返回阉割页），`--ua` 可改成模拟某平台爬虫。体检会指出缺 `og:image`/`og:title`/`description`/`canonical` 等关键标签。**只读静态 HTML、不跑 JS**：靠 JS 注入标签的 SPA 抓不到（如实反映，非 bug）。解析用 `x/net/html` 正经 tokenizer，畸形 HTML 也稳。

### `jdan csp`

解析 **Content-Security-Policy** 头成可读表格，并揪出常见弱点（迷你 CSP Evaluator）。0 依赖（纯字符串解析 + 复用现有 http 栈抓取）。

详细技术文档：[docs/jdan-csp.md](docs/jdan-csp.md)

```bash
$ jdan csp https://example.com                                  # 抓 URL 取 CSP 头
$ jdan csp "default-src 'self'; script-src 'self' 'unsafe-inline'"   # 直接给头值
$ echo "default-src 'self'" | jdan csp                          # stdin
$ jdan csp https://example.com --json
```

输入三选一:含空格/分号 → 当头值解析,否则当 URL 抓 `Content-Security-Policy`(缺了再试 `-Report-Only`)。**体检**才是重点:`'unsafe-inline'`/`'unsafe-eval'`、`*` 通配、缺 `default-src`、缺 `object-src 'none'`、`data:` 进脚本源、缺 `frame-ancestors`。**有意不做**全套 Google CSP-Evaluator(几十项 bypass 检测)和「生成更安全的 CSP」(越权易给错建议)。

### `jdan cookie`

解析 **Set-Cookie / Cookie** 头成可读表格,并揪出安全问题。解析直接用 stdlib `http.ParseSetCookie`(Go 1.23+),只加审计层,0 依赖。

详细技术文档：[docs/jdan-cookie.md](docs/jdan-cookie.md)

```bash
$ jdan cookie https://example.com                              # 抓 URL 取全部 Set-Cookie
$ jdan cookie "sid=abc; Path=/; Secure; HttpOnly; SameSite=Lax"   # 直接给一条
$ jdan cookie --request "a=1; b=2; sid=abc"                    # 当请求 Cookie 头(只列 pairs)
$ jdan cookie https://example.com --json
```

输入三选一:含 `=` → 头值,否则当 URL 抓(可多条 Set-Cookie)。**体检**:缺 `Secure`、缺 `HttpOnly`、`SameSite=None` 无 `Secure`(浏览器拒收)、`__Host-`/`__Secure-` 前缀规则、`Domain` 过宽。`--request` 把输入当请求 `Cookie:` 头(只列 name=value 对,不审计)。跟 `jdan csp` 是姊妹(都解析 HTTP 安全头),跟 `jdan jwt`(解 JWT)互补。

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

- `decode` 不验签 —— 验签是独立的 `jdan jwt verify` 子命令（见下）
- 不查 issuer 的 jwks_uri —— 任何网络行为都属于 `verify` 而不是 `decode`
- 不构造 JWT —— 同上

### `jdan jwt verify`

用 HMAC 密钥校验 JWT 签名（HS256/384/512）。`decode` 只看内容、不验签；`verify` 专做验签，需要 `--secret`。

详细技术文档：[docs/jdan-jwt-verify.md](docs/jdan-jwt-verify.md)

```bash
$ jdan jwt verify "$TOKEN" --secret mykey
✓ 签名有效 (HS256)            # 通过 → exit 0

$ jdan jwt verify "$TOKEN" --secret wrong
✗ 签名无效 (HS256)            # 失败 → exit 1（方便脚本 gate）

$ echo "Bearer $TOKEN" | jdan jwt verify --secret mykey   # stdin + 自动剥 Bearer
$ jdan jwt verify "$TOKEN" --secret mykey --json          # {alg, valid}
```

**安全要点 —— 防 alg-confusion**：校验**以 header.alg 为准**。如果 token 是 `RS256`/`ES256` 之类而你给了 `--secret`，会直接**报错拒绝**，绝不把非 HMAC token 当 HMAC 验（这正是经典的 alg 混淆攻击面）。HMAC 比对走 `crypto/hmac.Equal`（常量时间）。

flags：

| flag | 作用 |
|------|------|
| `--secret` | HMAC 密钥（给了才校签，仅 HS256/384/512） |
| `--json` | 结构化输出 `{alg, valid}` |

**有意不做**：RS*/ES* 公钥校验（需读 PEM 公钥 + 各曲线，后续可加 `--key`）；签发 JWT（jdan 是 inspector 取向）。

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

### `jdan entropy`

算一段字符串/文件/stdin 的 **Shannon 熵**（字节分布的信息量，0–8 bits/byte）。高（≥7.5）≈ 加密/压缩/随机，低 ≈ 重复/结构化文本。用来判断「这段数据是不是加密/压缩过」「文件里藏没藏高熵 secret」「能不能压缩」。0 新依赖（纯 `math`）。

详细技术文档：[docs/jdan-entropy.md](docs/jdan-entropy.md)

```bash
$ jdan entropy "hello world"
bytes:    11
entropy:  2.85 bits/byte   (低：文本/结构化)
total:    31.3 bits
distinct: 8 / 256 字节值

$ head -c 4096 /dev/urandom | jdan entropy | head -2
bytes:    4096
entropy:  7.96 bits/byte   (极高：疑似加密/压缩/随机)

# 滑窗 sparkline：一眼看出固件/二进制里被压缩/加密的那段
$ jdan entropy -f firmware.bin --window 512
▁▁▂▃█████▇▆▂▁
峰值 7.97 @ 偏移 0x1A00

# --charset：附加「搜索空间 bits」估算（明确非密码强度评分）
$ jdan entropy "Tr0ub4dour" --charset
charset:  62 符号集 ≈ 59.5 bits（搜索空间，非强度评分）
```

输入：位置参数=字符串 / `-f` 文件 / 无参=stdin；`--json` 结构化。**「熵」锚定在严格的 Shannon 定义**（数据随机性），不冒充密码强度评分——真要测强度该用字典/模式检查（zxcvbn 那套，要引库）。

### `jdan secrets-scan`

扫文件/目录/stdin 里**疑似硬编码的密钥/token**：正则引擎（已知格式，高精度）+ 高熵引擎（未知 token，复用 `entropy`）。像 gitleaks/trufflehog 的精简版。0 新依赖。

详细技术文档：[docs/jdan-secrets-scan.md](docs/jdan-secrets-scan.md)

```bash
$ jdan secrets-scan .
config/prod.env:7   [aws-access-key]  AKIA…J7QF  (high)
src/client.go:42    [generic-assign]  Xy9K…P6dC  (medium)
deploy.sh:3         [high-entropy]    dGhp…YWVo  (low, entropy 4.6)

共 3 处疑似密钥（已脱敏；exit 1）
```

**安全铁律：输出永不含完整 secret**，只给脱敏预览（前 4…后 4）——扫描器自己不能变成一次泄露（跟 `jdan pem` 同原则，有安全测试钉死）。降噪：内嵌 allowlist（UUID/示例占位）、行内 `# pragma: allowlist secret` 豁免、`--min-entropy` 可调、高熵命中标低置信。默认跳过 `.git`/`node_modules`/二进制/lock 文件（`-a` 全扫）。退出码 0 无发现 / 1 有发现（CI 卡门）/ 2 出错。`--json` 也不含完整 secret。git 历史扫描有意未做（v1）。

### `jdan pwned`

查一个密码是否出现在已知数据泄露中（基于 Have I Been Pwned 的 Pwned Passwords），**密码不出本机**。复用 stdlib `crypto/sha1` + `net/http` + `x/term`，**0 新依赖**。

详细技术文档：[docs/jdan-pwned.md](docs/jdan-pwned.md)

**原理（k-匿名）**：本地算 `SHA1(密码)`，只把哈希**前 5 位**发给 `api.pwnedpasswords.com/range/<前缀>`，服务器返回一批同前缀的哈希后缀 + 出现次数，本地再比对后 35 位。服务器只看到 5 位前缀（对应几十万个可能密码），你的明文和完整哈希**从不上网**。

```bash
$ jdan pwned                       # 无回显提示输入（不显示、不进 history）
$ echo -n 'password' | jdan pwned
⚠ 这个密码在已知泄露中出现过 52,372,427 次 —— 强烈建议别再用

$ cat passwords.txt | jdan pwned --batch   # 逐行批量审计
$ echo -n 'pw' | jdan pwned --json
```

输入只走**无回显交互提示**或 stdin（**故意不提供 `-p`**：查泄露的工具不该把密码留进 shell history）。默认带 `Add-Padding: true`（返回定长，连响应大小都不泄露）。退出码 0 干净 / 1 泄露 / 2 出错——可进 CI / pre-commit 当 gate。**有意不做**按邮箱查账号泄露（那个 API 要付费 key 且会把真实邮箱发出去，没有 k-匿名保护）。

### `jdan htpasswd`

生成 / 校验 **Apache·nginx Basic Auth** 的密码哈希行（`.htpasswd` 用）。**0 新依赖**。

详细技术文档：[docs/jdan-htpasswd.md](docs/jdan-htpasswd.md)

**原理**：Basic Auth 读一个 `用户名:密码哈希` 的文件,服务器把请求密码哈希后比对。这命令就是生成那些哈希行。哈希靠前缀分方言:`$2y$`(bcrypt,最安全)、`$apr1$`(Apache MD5-crypt,老系统通吃)、`{SHA}`(无盐,不安全仅兼容)。bcrypt 走 `x/crypto`(已在依赖图),apr1 手写 MD5-crypt(对齐 openssl 金标准),{SHA} 用 `crypto/sha1`。

```bash
$ jdan htpasswd alice                       # 交互输密码（两次确认）→ alice:$2y$...
$ jdan htpasswd alice --apr1                 # 用 apr1
$ printf 'pass\n' | jdan htpasswd alice      # 非 TTY：从 stdin 读密码
$ jdan htpasswd alice -f .htpasswd           # upsert 进文件（替换同名 / 追加新 / 保留其余）
$ jdan htpasswd --verify '$2y$10$...'        # 校验：输密码比对已有 hash
```

默认 bcrypt(`--cost` 调,默认 10);`--apr1` / `--sha` 换算法。密码**只走无回显提示或 stdin,绝不收 `-p`**(同 `jdan pwned`,不进 shell history)。`-f` 时同名用户替换、新用户追加、注释/其余行原样保留。`--verify` 按前缀自动认 bcrypt/apr1/{SHA},匹配退出 0、不匹配退出 1(可进脚本)。**有意不做**:`-p` 明文参数(安全红线)、crypt 传统 DES(古董不安全)、明文条目、htdigest(另一种格式,可后续单独做)。

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

### `jdan grab`

从任意文本 / 日志里把 **URL / email / IP** 捞出来。**0 依赖**。

详细技术文档：[docs/jdan-grab.md](docs/jdan-grab.md)

**原理**：纯正则永远做不完美（URL / email / IPv6 语法太复杂），所以两步走——松正则把「长得像」的候选都抓出来，再用 **stdlib 校验器**留真去假：`netip.ParseAddr`（IP）/ `url.Parse`（URL）/ `mail.ParseAddress`（email）。所以 `999.1.1.1`、`a@@b`、MAC 地址、`12:34:56`（时间）这类会被自动淘汰。这三个校验器项目里都用过，比手写正则可靠得多。

```bash
$ cat access.log | jdan grab -t ip        # 只抽 IP（逐行，可管道）
$ jdan grab -t email < contacts.txt       # 只抽邮箱
$ pbpaste | jdan grab                      # 抽全部，带类型标签
$ jdan grab -t ip --count log.txt         # 带出现次数（默认降序）
$ jdan grab page.html --json              # 按类型分组 JSON
```

`-t` 选类型（`url`/`email`/`ip`，csv，默认全部）。默认去重 + 保留首次出现顺序;`--count` 出现次数;`--sort` 排序。输入：无参读 stdin、参数是已存在文件则读文件、否则当字面文本。IP 归一化（IPv6 压缩零段）。**有意不做**：电话号（locale 太杂）、信用卡（那是 `secrets-scan`）、裸域名 / `www.`（太噪）；MAC 可后续加。

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

### `jdan calc`

命令行算术表达式计算器。手写递归下降解析器，支持四则 / 幂 / 取模 / 括号 / 一元负号 / 函数 / 常量 / 进制操作数。0 新依赖（纯 `math` + `strconv`）。

详细技术文档：[docs/jdan-calc.md](docs/jdan-calc.md)

**为什么单独做一个**：命令行快算别扭——`python3 -c` 启动慢、`bc` 报错难懂没 `^` 幂、shell `$((...))` 只有整数没函数。`jdan calc` 一行搞定。

```bash
$ jdan calc "3 * (4 + 5) / 2"     # 13.5
$ jdan calc "2 ^ 10"              # 1024（^ 右结合，也接受 **）
$ jdan calc "-5 + 3"              # -2（表达式可以负号开头）
$ jdan calc "sqrt(2)"            # 1.4142135623730951
$ jdan calc "max(3, 7, 2)"       # 7
$ jdan calc "pi * 2"             # 6.283185307179586
$ jdan calc "0xFF + 1"           # 256（进制操作数）
$ jdan calc "255 + 1" --hex      # 0x100
$ jdan calc "10 / 3" --precision 2  # 3.33
$ echo "1 + 2 * 3" | jdan calc   # stdin → 7
```

**函数**：sqrt/abs/floor/ceil/round/ln/log10/sin/cos/tan/min/max；**常量**：pi/e/tau（大小写不敏感，可嵌套）。错误带位置信息，比 bc 友好。

**边界**：算术 + 函数归 `jdan calc`；位运算（AND/OR/XOR/shift）归 `jdan num bit`，不重叠。

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

JSON 工具集（**13 个子命令**）。设计目标：常见操作 0 学习曲线，**不替代 jq**。复杂查询请用 jq；jdan json 覆盖日常 80% 高频场景。

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

# flatten ↔ unflatten（嵌套 ↔ 点分键；键格式 = json path 表达式）
$ echo '{"a":{"b":1,"c":[10,20]}}' | jdan json flatten
{"a.b":1,"a.c[0]":10,"a.c[1]":20}
$ echo '{"a.b":1,"a.c":2}' | jdan json unflatten
{"a":{"b":1,"c":2}}

# merge（深度合并，后者覆盖前者；对象递归合而非整体替换）
$ jdan json merge defaults.json prod.json local.json     # 配置分层
$ jdan json merge a.json b.json --arrays append          # 数组拼接而非替换
```

`flatten`/`unflatten` 详见 [docs/jdan-json-flatten.md](docs/jdan-json-flatten.md)：round-trip 还原（空容器保留 + 大整数精度）、`--sep` 改分隔符、稀疏数组补 null、对象/数组冲突检测。`merge` 详见 [docs/jdan-json-merge.md](docs/jdan-json-merge.md)：对象递归合并、`--arrays replace/append`、`-`=stdin、保大整数精度、不改输入。

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

# 聚合：一堆网段 → 最小 CIDR 覆盖集（split 的逆运算，防火墙/路由汇总）
$ jdan ip aggregate 10.0.0.0/25 10.0.0.128/25 10.1.0.0/24
10.0.0.0/24
10.1.0.0/24
(3 in → 2 out)
$ cat routes.txt | jdan ip aggregate           # 也可走 stdin

# 任意起止 IP 区间 → 最小 CIDR 集（range 的反向，iptables/ipset 常用）
$ jdan ip range-cidr 192.168.1.5 192.168.1.20
192.168.1.5/32
192.168.1.6/31
192.168.1.8/29
192.168.1.16/30
192.168.1.20/32
(5 CIDRs)

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

### `jdan cert`

生成本地开发 / 测试用的自签名 TLS 证书。跟 `jdan ssl cert`（检查证书）互补：一个造，一个看。0 新依赖（纯 `crypto/x509` + `crypto/tls`）。

详细技术文档：[docs/jdan-cert.md](docs/jdan-cert.md)

**为什么单独做一个**：本地 HTTPS 调试要自签证书，`openssl req` 的 flag 谁都记不住、还容易漏 SAN（现代浏览器光 CN 不认）。`jdan cert localhost` 一行搞定，默认就带正确 SAN。

> ⚠ 仅限本地开发 / 测试，不要用于生产（生产证书走 ACME / certbot）。

```bash
$ jdan cert localhost
Generated self-signed certificate:
  Cert:        cert.pem
  Key:         cert-key.pem
  Subject:     CN=localhost
  SAN:         DNS:localhost
  Key type:    EC (P-256)
  Valid:       2026-06-16 → 2028-09-18 (825 days)
  Fingerprint: SHA256:...

# SAN 自动推断（IP 字面量进 IP SAN，否则 DNS）
$ jdan cert myapp --ip 127.0.0.1,::1 --san "*.myapp.local"
  SAN:         DNS:myapp, DNS:*.myapp.local, IP:127.0.0.1, IP:::1

# --ca：生成 CA + leaf，信任 CA 一次之后所有它签的都被信任
$ jdan cert localhost --ca
  CA cert:     ca.pem        ← 加这个到信任库（一次）
  ...

# 造 → 看闭环
$ jdan cert localhost && jdan ssl cert -f cert.pem
```

`--key-type ec/rsa/ed25519`，key 文件权限 **0600**，`--stdout` / `--json` 输出。leaf 带 ServerAuth EKU，可直接喂给本地 HTTPS server。

### `jdan pem`

离线检视一个 PEM 文件：把每个 PEM 块拆出来、认出类型、给摘要。**不联网、绝不打印私钥内容**。跟 `jdan ssl cert`（联网抓 host 证书）/ `jdan cert`（生成自签证书）互补——`pem` 读本地文件。0 新依赖（复用 `internal/sslcert` 的证书描述）。

详细技术文档：[docs/jdan-pem.md](docs/jdan-pem.md)

```bash
$ jdan pem fullchain.pem
[1] CERTIFICATE  (1187 bytes)
    subject:  CN=example.com
    issuer:   CN=R3, O=Let's Encrypt
    validity: 2026-01-01 → 2026-04-01  (剩余 64d)
    SAN:      example.com, www.example.com
    key:      RSA 2048
    CA:       false
    sha256:   ab:cd:ef:...

$ jdan pem server.key
[1] PRIVATE KEY  (138 bytes)
    key:      EC P-256   （私钥内容不打印）

# cert + key 合一时，自动比对公钥告诉你是否匹配
$ cat cert.pem key.pem | jdan pem | tail -1
✓ 私钥与证书匹配
```

支持的块型：CERTIFICATE / CERTIFICATE REQUEST(CSR) / 各种 PRIVATE KEY（只给类型+位数）/ PUBLIC KEY / 加密私钥（标记不解密）/ 其它（类型+大小）。多块（fullchain / cert+key）全部列出，单块解析失败行内标注、继续下一块。`--json` 结构化。无 PEM 块 / 文件读不了 → 报错。

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

### `jdan http headers`

拉一个 URL，打印**状态行 + 响应头 + 完整重定向链**（逐跳显示）。比 `curl -I` 好读。0 新依赖（纯 stdlib）。跟 `http timing`（测阶段耗时）互补。

详细技术文档：[docs/jdan-http-headers.md](docs/jdan-http-headers.md)

```bash
$ jdan http headers http://github.com
301 Moved Permanently
  Location: https://github.com/
→ 200 OK
  Content-Type: text/html; charset=utf-8
  Server: github.com
  Strict-Transport-Security: max-age=31536000; includeSubdomains; preload
  ...

$ jdan http headers github.com               # 无 scheme 自动补 https://
$ jdan http headers <url> --max-redirects 0  # 不跟转
$ jdan http headers <url> -H "Authorization: Bearer x"
$ jdan http headers <url> --json
```

**手动跟重定向**（不靠 client 自动跟），逐跳展示每一跳的 status/Location/响应头——自动跟转做不到。默认 GET 但只读响应头、不下载 body（避开 HEAD 的怪行为）。相对 Location 正确解析；重定向循环被 `--max-redirects` 截断；连接失败时已成功的跳照常列出。重定向跳默认只显 Location，`-a` 每跳显全部头。

### `jdan http grade`

给站点的**安全响应头**打分（A+~F），风格同 securityheaders.com。**0 新依赖**（复用 `http headers` 的抓取）。看核心 6 项（HSTS/CSP/X-Content-Type-Options/X-Frame-Options/Referrer-Policy/Permissions-Policy），并对信息泄露头（`Server` 带版本号、`X-Powered-By`）反向扣分。

```
$ jdan http grade github.com
安全响应头评级：B (74/100)  https://github.com

✓ Strict-Transport-Security    max-age=31536000; includeSubdomains; preload
⚠ Content-Security-Policy      含 unsafe-inline（削弱了防护，等于给内联脚本开口子）
✓ X-Content-Type-Options       nosniff
✓ X-Frame-Options              由 CSP frame-ancestors 覆盖
✓ Referrer-Policy              strict-origin-when-cross-origin
✗ Permissions-Policy           缺失
```

解析头**内容质量**而非只看存在：HSTS 的 `max-age` 太短要扣、CSP 含 `unsafe-inline` 降级、X-Frame 被 CSP `frame-ancestors` 覆盖也算过。跨源隔离 COOP/COEP/CORP 默认只提示、`--strict` 才计入。**退出码默认恒 0**（评估报告），只有 `--fail-under B` 且实际更低时才非 0（CI 卡门）——这跟 `net cdn`/`git secrets` 的是/否判定故意不同。**有意不做**：不主动扫漏洞/不发探测 payload、只读一次正常响应头；不代改服务器配置。原理详见 [docs/jdan-http-grade.md](docs/jdan-http-grade.md)。

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

### `jdan net cdn`

给个网址，判断它前面挂没挂 CDN/WAF、挂的是哪家。**0 新依赖**（纯 stdlib）。

```
$ jdan net cdn cloudflare.com
✅ Cloudflare（确定）
   经 LAX 边缘
   最终 URL：https://www.cloudflare.com/

Cloudflare：
   · [header] cf-ray: a131…-LAX ★
   · [header] server: cloudflare
   · [ns] NS jule.ns.cloudflare.com
   · [ip] 104.16.124.96 ∈ 104.16.0.0/13 ★
```

三路**互相独立**的信号，任一命中即报，多路一致定性「确定」：

- **HTTP 响应头指纹** — 各家的铁证头（★）：Cloudflare `CF-RAY`、CloudFront `x-amz-cf-id`、Akamai `x-akamai-request-id`、Fastly `x-fastly-request-id`；国内 CDN：阿里云 `EagleId`/`Server: Tengine`、百度 `Server: bfe`、腾讯 `X-NWS-LOG-UUID`/`stgw`/`tRPC-Gateway`、京东 `Via: (jcs …)`、网宿 `X-Ser`。`CF-RAY` 后缀还是边缘机房的 IATA 机场码，顺手解出来
- **fake-ip 代理友好** — Clash/Surge 那类 fake-ip 模式下 DNS 返回的是 `198.18.0.0/15` 合成 IP，IP 段判定天然失效；jdan 会识别并提示，结论改靠响应头 + NS（穿过代理仍是真实值）
- **DNS NS 记录** — 域名是否托管在该 CDN 的 DNS（如 `*.ns.cloudflare.com`）。从全名往上逐级找委派点，不依赖 PSL
- **IP 段归属** — 解析 IP 落不落在该 CDN 公布的 CIDR 段里（内嵌 Cloudflare 全段，约 15 v4 + 7 v6）。头被删了也藏不住

```
--headers-only      只看响应头，跳过 DNS/IP 解析（快、离线友好）
--json              JSON 输出（顶层带 detected 布尔）
-k, --insecure      跳过 TLS 证书验证
--max-redirects N   最多跟几跳重定向（默认 10，0 = 不跟）
--timeout           单步超时（默认 10s）
```

**退出码**：文本模式检测到 = 0、没检测到 = 非 0（可进 CI）；`--json` 恒 0，脚本读 `.detected`。

**有意不做**：回源 IP 反查 / 揭穿真实后端（攻击性侦察，安全红线，只检测不去匿名化）、WAF 绕过、联网更新 CIDR 段。Fastly 无公开强指纹头，按启发式诚实标「很可能」。原理详见 [docs/jdan-cdn.md](docs/jdan-cdn.md)。

### `jdan net ws`

探测一个 **WebSocket 端点**：发 HTTP Upgrade 握手、验 `101` + `Sec-WebSocket-Accept`（确认是真 WS 端点），再发一个 ping 帧收 pong（证明数据真能通）。**0 新依赖**（纯 stdlib 手搓握手 + 最小 RFC6455 帧）。跟 `net probe`（探到 HTTP 层）互补，再往上探一层。

```
$ jdan net ws echo.websocket.org           # 无 scheme 自动补 wss://
WebSocket 握手：✓ 101 Switching Protocols  (握手 271.0ms)  wss://echo.websocket.org
  Server:   Fly/…
  Ping/Pong: ✓ pong 339.8ms
```

自己按 RFC6455 公式复算 `Sec-WebSocket-Accept` 比对，防「随便回个 101」的假阳性；ping 帧按客户端规则**掩码**，读到 pong 才算数据真通。`--origin`/`--subprotocol`/`-H` 应付按 Origin 校验或要协商子协议的端点；`--no-ping` 只握手；`-k` 跳 TLS 校验。

**退出码**：0 握手成功 / 非0 失败（连不上/非101/Accept 不对/超时），可当 WS 探活进 CI。**ping/pong 只是附加连通性提示**，没收到 pong 不影响退出码（有些服务端不自动回 pong）。**有意不做**：不做交互式 WS 客户端（那是 wscat/websocat）、不压测、不绕鉴权。原理详见 [docs/jdan-net-ws.md](docs/jdan-net-ws.md)。

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

### `jdan ping`

ping 一个主机，但**可用 `--dns` 指定解析域名用的 DNS server**：给了就先用指定 DNS 把域名解析成 IP 再 ping 这个 IP；不给则退化成系统 ping 默认行为。0 新依赖。

详细技术文档：[docs/jdan-ping.md](docs/jdan-ping.md)

系统 `ping <域名>` 只走系统解析器、没有 `--dns` 参数。`jdan ping` 把「指定 DNS 解析 + ping」合一，排查 DNS 劫持/不同 DNS 解析到不同 IP 时能直观看到结果。

```bash
$ jdan ping --dns 8.8.8.8 example.com
example.com → 93.184.216.34 (via 8.8.8.8)     # jdan 加的解析头
PING 93.184.216.34 (93.184.216.34): 56 data bytes   # 系统 ping 原样输出
64 bytes from 93.184.216.34: icmp_seq=0 ttl=56 time=12.1 ms

$ jdan ping example.com                       # 无 --dns → 系统 ping 默认行为
$ jdan ping --doh google example.com          # DoH 别名（自带 bootstrap IP，更防劫持）
$ jdan ping --dns 8.8.8.8 -c 3 example.com --json
$ jdan ping --dns 8.8.8.8 example.com -- -i 0.2 -s 64   # -- 后透传给系统 ping
```

**两种指定解析的方式**：`--dns`（`8.8.8.8` / `host:port` / 完整 DoH URL）和 `--doh`（别名 `google`/`cloudflare`/… / 主机名 / URL），**互斥**。`--doh <别名>` 强在自带 **bootstrap IP**——连解析 DoH endpoint 主机名都绕过本地 DNS，是防劫持首选（实测在 fake-ip 劫持的网络里 `--dns 8.8.8.8` 拿到伪造 IP，`--doh google` 靠 bootstrap 拿到真实 IP）。

**设计**：实际 ICMP 由系统 ping 完成（shell out，像 `jdan git`），jdan 只负责「用指定 DNS 解析成 IP + 构造 argv + 尽力解析汇总行供 `--json`」。关键：指定了解析时一定 ping 解析出的 IP 而非域名，否则 ping 会用系统 resolver 再解析一次绕过你指定的 DNS。`-c` 内置（Linux/macOS 通用），其余高级 flag 用 `--` 透传不翻译。仅 macOS + Linux（IPv6 时 Linux 用 `ping -6`、macOS 用 `ping6`）。

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

### `jdan disk`

像 `df`：列各挂载点的容量/已用/可用/使用率，带使用率条和高占用染色。给路径则只看该路径所在的文件系统。**0 新依赖**（纯 `syscall`）。仅 darwin / linux。

详细技术文档：[docs/jdan-disk.md](docs/jdan-disk.md)

```bash
$ jdan disk
文件系统         容量   已用   可用  使用率          挂载点
/dev/disk3s1s1  1.8Ti  1.6Ti  269Gi  86% ████████░   /
/dev/disk9s2    1.8Ti  1.7Ti   95Gi  95% █████████   /Volumes/m1max-tm

$ jdan disk /        # 只看根分区所在文件系统
$ jdan disk -a       # 含伪文件系统（devfs/tmpfs/map…）
$ jdan disk -i       # 显示 inode 用量
$ jdan disk --bytes  # 原始字节
$ jdan disk --json
```

使用率算法对齐 `df`（`已用/(已用+可用)` 向上取整）。TTY 下使用率 ≥90% 染红、≥75% 染黄；管道/重定向纯文本不插 ANSI。默认隐藏伪文件系统**和 TimeMachine 本地快照**，`-a` 全显。超长设备名/挂载点按终端宽度**中间省略号截断**（只在 TTY；管道/`--json`/`--no-trunc` 全显）。Windows 暂不支持（报清晰错）。

### `jdan size`

扫描目录树按占盘大小排行，带占比条形图。省掉 `du -sh * | sort -hr | head` 这串管道（`sort -hr` 在 BSD 和 GNU 上行为还不一致）。**0 新依赖**。

详细技术文档：[docs/jdan-size.md](docs/jdan-size.md)

```bash
$ jdan size ~/.claude
/Users/quincy/.claude  784.7Mi  （11,039 个文件）

  projects         577.7Mi  █████████████░░░░  73.6%
  plugins           79.3Mi  ██░░░░░░░░░░░░░░░  10.1%
  transcripts       74.8Mi  ██░░░░░░░░░░░░░░░   9.5%
  file-history      49.6Mi  █░░░░░░░░░░░░░░░░   6.3%
  其他 9 项          1.4Mi  ░░░░░░░░░░░░░░░░░   0.2%

用时 36ms

$ jdan size --depth 3        # 展开三层
$ jdan size --files          # 把文件也列出来（找单个大文件）
$ jdan size --apparent       # 按逻辑大小（Finder 那个数字）
$ jdan size --all            # 含隐藏文件
$ jdan size --json | jq      # 全树 JSON
```

**默认量的是实际占盘（`st_blocks × 512`）而不是逻辑大小**，因为你问的是「删掉能腾出多少空间」。两者差得比直觉大且方向常被搞反：500 个 1 字节文件逻辑 500 B、实际占 2 MB（4 KiB 块取整，4000×）；稀疏文件则相反，逻辑 1 GiB、实际 0 B。要 Finder 那个数字用 `--apparent`。

**根总量与 `du -sh` 逐字节一致**，语义全部对齐（硬链接只计一次、默认不跨文件系统、不跟随符号链接、目录自身的块计入）。一处刻意不同：硬链接归属给**字典序最小的路径**而非 `du` 的「先遇到的」，因此并发扫描下同一棵树连跑多次输出逐字节相同，代价是单个子目录数字可能与 `du` 不一致。

并发遍历，实测 11793 个文件热缓存下 `--jobs 8` 比单线程快 6.3×。并发度按存储介质定：SSD 用默认，机械盘建议 `--jobs 2` 到 `4`。权限错误收集不中断，页脚汇总且退出码仍为 0。`--json` 永远输出全树（不受 `--top`/`--depth` 影响）。非 darwin/linux 降级为逻辑大小并在头部提示。

### `jdan wifi`

看当前无线连接，并分析周边 AP 的信道占用，给出换信道建议。回答菜单栏不会告诉你的事：信道撞没撞、SNR 够不够、协商到哪代协议、邻居把哪个频段占满了。**0 新依赖**。仅 macOS。

详细技术文档：[docs/jdan-wifi.md](docs/jdan-wifi.md)

```bash
$ jdan wifi --band 5
en0  802.11ax  信道 36 (5GHz, 80MHz → 占用 36/40/44/48)
     信号 -41dBm / 噪声 -92dBm   SNR 51dB  优
     WPA2/WPA3 Personal   协商 1200 Mbps (MCS 11)
     SSID <已脱敏 — 见末尾说明>

5GHz 信道占用（★ = 本机，3 次采样）
  信道               同信道  邻道噪声
  149    ███░░░░░░░       1         —
  52     █████░░░░░       2         —  DFS
  36 ★   ██████████       4         —

建议：当前信道 36 上有 4 个 BSS 超过载波侦听门限（-82 dBm），每次它们发包你都要退避。
      信道 149 只有 1 个，更空。

$ jdan wifi --samples 5      # 多采样（扫描不稳，默认 3 次）
$ jdan wifi --all-channels   # 列出全部候选信道
$ jdan wifi --json | jq
```

**为什么不是 `airport`**：实测 macOS 26.5.2 上 `airport` 二进制**已被删除**（不是废弃，是文件不存在），而网上几乎所有教程还在教用它；`networksetup -getairportnetwork` 在已连接时**会谎报**「未关联」；`wdutil info` 要 sudo。用 `system_profiler -xml`，单次约 1.7 秒。

**关于 SSID**：macOS 14 起把它归类为位置信息，CLI 拿不到（显示为脱敏，输出里会给授权路径）。但信道、信号、协议、加密和周边全部 AP 的射频数据都不受限制 —— 而信道分析恰好一个都不需要 SSID。

**信道分析给两个量而非一个合成分**：同信道 BSS 数（CSMA/CA 退避，按空口时间算，与相对强度无关）和邻道噪声（不可解码能量当噪声，按线性功率求和）。压成一个分会低估「很多个中等强度同信道 BSS」这种真实很糟的情况。5GHz 的 80/160MHz 按**固定对齐块**展开（`44@80` 占 `{36,40,44,48}` 而非 `{44,48,52,56}`）。

扫描结果不稳定 —— 实测同一时刻连跑 6 次得到 13/14/17/15/17/17 个邻居（31% 波动），所以默认采样 3 次取并集，「0 个 AP」只在 N 次全为 0 时才标记为真空。推荐信道必须能承载当前带宽、且落在 DFS 段时会提示 CAC 静默期（60s，气象雷达段 600s）。

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

### `jdan cal`

打印公历日历，高亮今天。默认本月、**周一起始（ISO）**、中文表头。0 新依赖（纯 `time`）。

详细技术文档：[docs/jdan-cal.md](docs/jdan-cal.md)

```bash
$ jdan cal
    2026 年 6 月
一 二 三 四 五 六 日
 1  2  3  4  5  6  7
 8  9 10 11 12 13 14
15 16 17 18 19 20 21
22 23 24 25 26 27 28
29 30

$ jdan cal 12 2025      # 指定月/年（cal 6 = 今年 6 月，避开 Unix cal 把 "6" 当公元 6 年的坑）
$ jdan cal -y 2026      # 整年（3×4 月块）
$ jdan cal -3           # 上/本/下月三联排
$ jdan cal -w           # 左栏显示 ISO 周数
$ jdan cal 6 2026 -s    # 周日起始

$ jdan cal -l           # 农历月历：每个公历日下显示农历（初一显示月名）
               2026 年 6 月
  一    二    三    四    五    六    日
  1     2     3     4     5     6     7
 十六  十七  十八  十九  二十  廿一  廿二
  ...
  15    16    17    18    19    20    21
 五月  初二  初三  初四  初五  初六  初七
```

今天在 TTY 下**反显**高亮，管道/重定向时输出纯文本（不插 ANSI，可解析）。`--json` 给 `{year, month, week_start, weeks}` 结构化数据。`-l/--lunar` 把农历叠在月历上（每格两行，初一显示农历月名、闰月显示「闰六月」，仅单月；农历表 1900–2100 由 `jdan lunar` 提供）。单日农历查询/转换/节日见独立的 `jdan lunar`。

### `jdan lunar`

公历 ↔ 农历（中国阴历）转换，含**干支纪年、生肖、农历节日**。内嵌 1900–2100 农历表（公开算法、~200 个常量），**0 新依赖**。

详细技术文档：[docs/jdan-lunar.md](docs/jdan-lunar.md)

```bash
$ jdan lunar
公历: 2026-06-26 (周五)
农历: 丙午年 五月十二  (生肖 马)

$ jdan lunar 2024-02-10              # 指定公历日 → 农历
公历: 2024-02-10 (周六)
农历: 甲辰年 正月初一  (生肖 龙)

$ jdan lunar --to-solar 2026 1 1     # 农历 → 公历（今年春节几号）
公历: 2026-02-17 (周二)

$ jdan lunar --to-solar 2025 6 1 --leap   # 闰月（2025 闰六月初一）
$ jdan lunar 2026 --festivals        # 列某年农历节日（春节/元宵/端午/七夕/中秋/重阳/除夕）
$ jdan lunar 2026-06-26 --json
```

**正确性靠真实锚点 + 全程 round-trip 守护**：2024/2025/2026 春节、中秋、端午、闰月（2025 闰六月 / 2023 闰二月 / 2020 闰四月）逐一断言，再对 1900–2100 全程 `公历→农历→公历` 往返自检。范围 1900–2100，越界报错。

干支以正月初一为界（生肖年）。**有意不做**：黄历宜忌（无权威算法）、24 节气（属太阳历，另一套）、第三方农历库（内嵌表足矣）。

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

### `jdan uuid`

检视一个 UUID：版本、variant、v1/v7 内嵌时间戳、字节、URN 形式、nil/max。生成在 `jdan rand uuid`，本命令专做**解析**（`jdan uuid new` 是薄封装，复用同一实现，零逻辑重复）。0 新依赖。

详细技术文档：[docs/jdan-uuid.md](docs/jdan-uuid.md)

```bash
$ jdan uuid 0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b
canonical: 0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b
version:   7 (时间排序)
variant:   RFC 4122
time:      2026-06-26 14:00:00.000 UTC
bytes:     01 90 a1 b2 c3 d4 7e 5f 8a 9b 1c 2d 3e 4f 5a 6b
urn:       urn:uuid:0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b

# 输入容错（urn 前缀 / 花括号 / 无连字符 / 大小写）+ stdin + JSON
$ echo "$U" | jdan uuid --json
$ jdan uuid new --v7 -n 3      # 生成（复用 jdan rand uuid）
```

v7/v1 自动解出内嵌时间戳；nil（全 0）/ max（全 F）特殊标注。非法 UUID 清晰报错。

### `jdan fake`

生成像真实数据的结构化假值，供造测试 fixture、填库、写示例。0 新依赖（内置词库）。跟 `jdan rand`（无意义随机串）互补——`fake` 给的是像真实数据的结构化假值。

详细技术文档：[docs/jdan-fake.md](docs/jdan-fake.md)

**类型**：name / email / uuid / sentence / word / int / date / ip

```bash
$ jdan fake name
Alice Chen

$ jdan fake email -n 3
bob.patel@example.net
amy.wong@test.org
leo.kim@demo.net

# --seed 可复现（造稳定 fixture）
$ jdan fake name --seed 42 -n 2
Zack Walker
Cleo King

$ jdan fake int --min 1 --max 6      # 骰子
$ jdan fake uuid --json -n 5         # JSON 数组

# 无 type + --json → 复合记录
$ jdan fake --json -n 2
[
  {"name":"Bob Patel","email":"bob.patel@example.net","age":74,"ip":"198.51.100.134"},
  {"name":"Zack Thomas","email":"zack.thomas@example.org","age":33,"ip":"203.0.113.175"}
]
```

`--seed N` 切到确定性序列（同 seed 同输出，`date` 用固定窗口不依赖当前日期）；不设则用 `crypto/rand` 真随机。IP 只用 RFC 5737 文档保留段、邮箱用 RFC 2606 示例域名，安全不撞真实资源。`--list` 列出类型。

### `jdan git summary`

仓库一眼看：总 commit 数、分支、tag、年龄、贡献者榜、改动最多的文件（hotspots）。纯只读。jdan 第一个 git 命令，底层 shell out 到 `git`（**0 新 Go 依赖**，只要环境里有 git）。

详细技术文档：[docs/jdan-git-summary.md](docs/jdan-git-summary.md)

```bash
$ jdan git summary
仓库: jdan
commit: 77   分支: 5   tag: 2
年龄: 2026-03-21 起 (约 2 个月)

贡献者 Top 5:
  xunull  77 (100.0%)

改动最多的文件 (hotspots) Top 5:
  README.md            40 次
  go.mod                9 次
  internal/cli/dns.go   4 次

# 指定仓库 / 控制榜单条数 / 结构化输出
$ jdan git summary /path/repo
$ jdan git summary --top 10
$ jdan git summary --json
```

年龄用「首 commit → 末 commit」跨度（不依赖系统时间，可复现）。非 git 仓库 / 空仓库 / 缺 git 都有清晰报错。

### `jdan git changelog`

从最近 tag 到 HEAD 生成 changelog，按 Conventional Commits 分组（feat→Features / fix→Bug Fixes / …，breaking 单独拎出）。跟 `feat()/fix()` 提交风格契合，发版前一键出发布说明。默认输出 markdown，可直接重定向。底层 shell out `git`（**0 新依赖**）。

详细技术文档：[docs/jdan-git-changelog.md](docs/jdan-git-changelog.md)

```bash
$ jdan git changelog
## 未发布 (自 v0.5.2)

### ⚠ Breaking Changes
- (api) drop the v1 endpoint

### Features
- (json) add json merge for deep-merging
- (ping) add jdan ping with --dns

### Bug Fixes
- (figlet) block font UTF-8 panic

# 指定范围 / 结构化输出
$ jdan git changelog --from v0.4.0 --to v0.5.0
$ jdan git changelog > RELEASE.md
$ jdan git changelog --json
```

范围默认「最近 tag → HEAD」（无 tag 取全部历史）；merge commit 默认跳过；不符合规范的 subject 归到 Other（不丢）。非 git 仓库 / 非法 ref 有清晰报错。

### `jdan git commitlint`

按 **Conventional Commits** 规范校验提交信息（`type(scope): subject`）。**0 新依赖**。

```
$ jdan git commitlint origin/main..HEAD     # 校验 PR 分支上的全部提交
✗ d4e5f6  Fixed the login bug.
    · [header-structure] header 不符合 "type(scope): subject" 结构："Fixed the login bug."
✓ a1b2c3  feat(api): 加分页
1/2 条提交不合规 ✗
```

输入来源按优先级：`-m` 字面量 > `-f` 文件（commit-msg hook 用）> revision-range（调 git）> stdin > 默认 `HEAD`。查的规则：type 必填/白名单内/小写、subject 非空且无结尾句号、header 不超长（默认 100，**按 rune 计**，中文不误伤）、scope 小写、body 前空行；`!` 或 `BREAKING CHANGE:` 识别为破坏性。

```
-m "feat: x"        直接校验字面量
-f <file>           读文件（hook：git 把信息文件路径传进来）
--types a,b,c       覆盖 type 白名单
--max-header N       header 上限（默认 100）
--scope-required     强制要有 scope
--json               JSON 输出（顶层 ok 布尔）
--warn               软模式：只报不拦（退出 0）
```

**退出码**：全合规 0、有违规非 0（可直接当 commit-msg hook 拦下）。**当 hook 用**（不内置安装，避免覆盖 husky）：`.git/hooks/commit-msg` 写 `exec jdan git commitlint -f "$1"` 即可。原理与规则详见 [docs/jdan-commitlint.md](docs/jdan-commitlint.md)。

### `jdan git secrets`

扫 git **历史**里是否提交过密钥/凭据（也能扫暂存区），检测交给 **gitleaks**。**0 新 Go 依赖**（运行时需 `git` + `gitleaks`）。跟 `jdan secrets-scan`（零依赖、扫工作区）分工：这个审「过去有没有提交过」。

```
$ jdan git secrets                    # 扫当前仓库全历史
[history] config/app.go:12  aws-access-key  deadbeef  (Bob 2026-01-05)  REDACTED

疑似敏感文件（仅文件名，内容未验证）：
  · deploy/id_rsa   [SSH 私钥]

共 1 处内容命中 + 1 个可疑文件（已脱敏；exit 1）
```

比裸跑 gitleaks 多三样：**默认脱敏**（gitleaks 默认打印明文，jdan 固定传 `--redact=100`，要明文得 `--show-secrets`）、**补一层文件名审计**（抓 gitleaks 漏的 `.env`/`id_rsa`/keystore 这类内容无特征的凭据文件）、**统一退出码 + 友好报错**（没装 gitleaks 给安装指引）。

```
--staged             只扫暂存区（pre-commit 用）
--show-secrets       输出明文（默认脱敏）
--no-filenames       跳过文件名审计层
--json               机读（同样脱敏）
--log-opts=<x>       限范围（如 origin/main..HEAD）
--baseline <f>       忽略已知项（gitleaks baseline）
```

**退出码**：0 干净 / 1 有发现（CI 可卡门）/ 2 环境缺失（没装 gitleaks 或非 git 仓库）。**当 pre-commit hook 用**：`.git/hooks/pre-commit` 写 `exec jdan git secrets --staged`。**有意不做**：不替你改写历史（只检测 + 提示轮换）、不联网验真、不重造规则引擎。原理详见 [docs/jdan-git-secrets.md](docs/jdan-git-secrets.md)。

### `jdan toc`

从 Markdown 标题生成目录（TOC），anchor 跟 **GitHub 渲染规则一致**，可直接贴回 README。0 新依赖（纯 stdlib）。

详细技术文档：[docs/jdan-toc.md](docs/jdan-toc.md)

```bash
$ jdan toc README.md
- [安装](#安装)
  - [方式 1：下载预编译二进制（推荐）](#方式-1下载预编译二进制推荐)
- [命令](#命令)
  - [`jdan qr`](#jdan-qr)
  - [`jdan figlet`](#jdan-figlet)

# 只要某几级 / 回填到文件
$ jdan toc README.md --min 2 --max 3
$ jdan toc README.md --inplace   # 替换 <!-- toc --> 标记之间
```

anchor 算法 lowercase + 删标点（反引号、`#`）+ 空格转连字符 + 重复标题加 `-1`/`-2`，跟 GitHub 一致（已用本 README 验证逐一吻合）。默认从 h2 起（跳过文档大标题）。代码围栏内的 `#` 不会被误当标题。`--inplace` 缺标记会报错、可重复运行（幂等）。

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

**新 clone 后启用提交前防泄密钩子（一次即可）：**

```bash
pip install pre-commit   # 或 brew install pre-commit
pre-commit install
```

`.pre-commit-config.yaml` 配了 [gitleaks](https://github.com/gitleaks/gitleaks)，每次 `git commit` 前扫描暂存区，发现 API key / 密码 / token / 私钥就阻止提交。配置进了版本库，但钩子本身不进，所以**每个 clone 需要跑一次 `pre-commit install`** 才生效。

- 临时放行本次提交：`SKIP=gitleaks git commit`（或 `git commit --no-verify`）
- 误报：命中行尾加 `#gitleaks:allow`，或把 fingerprint 写进 `.gitleaksignore`

设计文档在 `docs/brainstorms/` 与 `docs/plans/` 下按时间排列，每个新子命令通常对应一对 brainstorm + plan。

