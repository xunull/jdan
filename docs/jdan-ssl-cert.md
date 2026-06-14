# jdan ssl cert

看 HTTPS 证书的命令。把"取一个 host 的证书 → 解析 → 验证 → 检查吊销"压成一行命令 + 漂亮 box drawing 输出，替代 `openssl s_client -connect host:443 -showcerts < /dev/null | openssl x509 -text -noout` 这种调用链。

## 它解决什么问题

日常运维 / 调试中常遇到的场景：

1. **"这个 cert 什么时候过期？"** —— 监控、告警、续签提醒
2. **"它包了哪些域名？"** —— SAN 列表，多域名 cert 排查
3. **"完整 chain 给我看下"** —— 排查 "missing intermediate cert" 这类经典问题
4. **"是谁签的？trusted 吗？"** —— 自签 / 私有 CA / 公网 CA 区分
5. **"fingerprint 是什么？"** —— cert pinning 验证
6. **"我有个 PEM 文件，看看里面写了啥"** —— 不联网，纯本地解析

`openssl` 能干这些，但要记两条不同的命令链，输出是面向机器的密文表格。`jdan ssl cert` 输出面向人类。

## 用法

### 基本

```bash
jdan ssl cert example.com               # 默认 :443
jdan ssl cert example.com:8443          # 显式端口
jdan ssl cert https://example.com/path  # 完整 URL 也接受
```

### 虚拟主机（指定 SNI）

server 上有多个 cert，需要通过 SNI 告诉它"我要哪个"：

```bash
jdan ssl cert lb.example.com --sni www.actual-domain.com
```

### 本地 PEM 文件

不联网，从磁盘读：

```bash
jdan ssl cert -f cert.pem
jdan ssl cert -f /etc/ssl/private/server-chain.pem
```

PEM 文件可以含多个 cert（chain），第一个被当作 leaf。

### 监控脚本

阈值内过期 → exit 1，让 shell 脚本能触发告警：

```bash
if ! jdan ssl cert myapp.example.com --expires-in 30d; then
    slack-send "cert for myapp expires in less than 30 days"
fi
```

支持 `30d` / `720h` / `15m` 这种 duration 字符串。

### 输出格式控制

```bash
jdan ssl cert example.com --full   # 展开 extensions / KeyUsage / OCSP/AIA URL
jdan ssl cert example.com --json   # 结构化 JSON 给 jq 等消费
jdan ssl cert example.com --pem    # 标准 PEM 给管道（openssl 等下游）
jdan ssl cert example.com --no-ocsp  # 跳过 OCSP，节省 300-500ms
```

## 输出形态

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
  ✓ CN=github.com                                       OCSP good
  ✓ CN=Sectigo Public Server Authentication CA DV E36   OCSP good
```

每个块的含义：

| 块 | 内容 |
|----|------|
| `╭─ leaf ─╮` | leaf cert 的核心字段；`--full` 时展开更多 |
| `Chain` | 完整 chain（leaf → intermediate → root），每行带过期倒计时 |
| `Verification` | 3 项独立验证：信任链 / hostname / expiry |
| `OCSP` | 每个非 root cert 的吊销状态（good / revoked / unknown / error） |

### 进度条

`Days left: █████░░░░░ 50 days` —— 一眼看出剩多少寿命。过期则显示 `EXPIRED 5 days ago`。

### 失败时

```
Verification:
  ✗ chain NOT trusted: x509: certificate signed by unknown authority
  ✗ hostname mismatch: x509: certificate is valid for foo.com, not bar.com
  ✗ EXPIRED

OCSP:
  ✗ CN=revoked.example.com  REVOKED at 2026-06-01 (reason: keyCompromise)
```

### Cross-signing 场景

server 发的 root 和系统 trust 的 root 是不同的 cert（cross-signing 是 PKI 真实场景）时，chain 会同时显示两个：

```
Chain:
  ▸ leaf:        CN=foo.com
  ▸ intermediate: CN=Some CA G3
  ▸ intermediate: CN=Some Root R1  (exp in 4237d)   ← cross-signed by 旧 root
  ▸ root:        CN=Some Root R1  (exp in 7221d, self-signed)  ← 系统 trust 来的真 root
```

这不是 bug——cross-signing 是真实存在的 PKI 配置（让老客户端走旧 chain，新客户端走新 chain）。

## flags 完整列表

| flag | 默认 | 作用 |
|------|------|------|
| `-f` / `--file` | 无 | 从本地 PEM 文件读，不联网 |
| `--sni` | host | TLS 握手发的 server_name |
| `--full` | false | 展开 extensions / KeyUsage / EKU / OCSP URL / CRL URL |
| `--json` | false | 结构化 JSON（含 leaf / chain / verification / ocsp 四段） |
| `--pem` | false | 输出标准 PEM 给管道 |
| `--no-ocsp` | false | 跳过 OCSP 查询 |
| `--timeout` | 5s | 整体超时 |
| `--expires-in` | 无 | 如 `30d` / `720h`，leaf 在此期内过期则 exit 1 |

## 关键设计取舍

### `InsecureSkipVerify=true` 取 cert

要"看证书"就**不能因为 cert 不可信直接拒绝**——这恰恰是用户最想看的场景之一（"为什么浏览器报 untrusted"）。fetch 阶段无视信任拿完整 chain，verify 阶段单独跑系统 trust store + hostname + expiry，结果作为 **report** 报给用户。

### `jdan ssl` 而不是 `jdan tls`

技术上 SSL 3 已弃用，TLS 才准确。但用户搜索习惯仍是 SSL（`openssl s_client`、curl `--cacert`、Apple `security` 命令都还用 SSL 这词）。**用户体验 > 技术纯洁**。

### 2-level 子命令

`jdan ssl cert <host>` 而不是 `jdan ssl <host>` —— 留 `pem` / `diff` / `ct` 子命令位置给未来。

### OCSP，不做 OCSP stapling / CRL

- **OCSP**：用 `golang.org/x/crypto/ocsp`（quasi-stdlib，Go 团队维护）。cert 没 OCSP responder URL 时静默跳过（root cert 常见情况）。网络失败带 ⚠ 警告但不拒绝命令
- **不做 OCSP stapling 解析**（从 TLS 握手抓 stapled response）：复杂度高、覆盖率低，直查 responder 更直接
- **不做 CRL**：CRL 文件可能几 MB，下载慢且场景窄；OCSP 已经覆盖 95% revocation 场景

### 进度条 + 倒计时

`█████░░░░░ 50 days` 是这个命令的"whoa moment"。`openssl x509 -text -noout` 把 NotAfter 当一个 ISO 时间戳吐出来，你得心算"今天到那天还有多少天"。进度条直接显示。

### `--expires-in` 用 exit code

让监控脚本能 `if ! jdan ssl cert host --expires-in 30d; then alert; fi`，比解析 JSON 输出找字段更简单。

## 内部架构

```
internal/sslcert/                  # 独立 package，不依赖 internal/netprobe
  bundle.go      Bundle { Chain, VerifiedChains } + FullChain() (dedup root)
  fetch.go       FetchFromHost(host, port, sni, timeout) → *Bundle
                 用 InsecureSkipVerify 故意不验
                 ParseTarget(s) 拆 "host" / "host:port" / "https://host/path"
  parse.go       ParsePEMFile / ParsePEMBytes / EncodePEM
  describe.go    *x509.Certificate → Summary（JSON-friendly 字段）
                 ShortName(c) 给 chain 列表用
                 keyAlgorithmString 识别 RSA / EC / Ed25519
  verify.go      Verify(bundle, hostname) → *VerificationReport
                 3 项：trust store + VerifyHostname + NotAfter/NotBefore
                 同步填充 bundle.VerifiedChains 让 RootFromTrust 能找到 root
  ocsp.go        CheckOCSP(cert, issuer) → OCSPStatus
                 CheckChainOCSP 遍历 chain
                 OCSPHTTPClient 可注入 mock，测试用 500 响应验 error 路径

internal/cli/
  ssl.go             jdan ssl 命名空间
  ssl_cert.go        jdan ssl cert 子命令 + flags 解析 + 调度
                     sslCertExitErr 让 --expires-in 失败时 exit 1
                     parseDurationWithDays("30d") 因为 time.ParseDuration 不支持 "d"
  ssl_cert_render.go box drawing + 进度条 + chain + Verification + OCSP 渲染
                     truncateMiddle / padRight / visibleLen 小工具
```

数据流：

```
target / -f file
       │
       ▼
┌─────────────────┐
│ ParseTarget /   │
│ ParsePEMFile    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ FetchFromHost / │      ← InsecureSkipVerify=true
│ ParsePEMBytes   │        (验证作为独立步骤)
└────────┬────────┘
         │
   *Bundle (Chain []*x509.Certificate)
         │
         ├──→ Verify(bundle, hostname) → *VerificationReport
         │                              (filling VerifiedChains 让 FullChain
         │                               能 surface 系统 trust 来的 root)
         │
         ├──→ CheckChainOCSP(ctx, chain) → []OCSPStatus
         │                                 (可选；cert 无 OCSP URL 时空)
         │
         ▼
   render (text box / JSON / PEM)
         │
         ▼
   checkExpiresIn(bundle, "30d") → 满足 → exit 1
```

## 与其他 jdan 命令的关系

- **`jdan net probe`**：TLS 阶段已经吐了一行 cert 摘要（version / cipher / leaf subject / issuer / NotAfter）。**它是诊断顺路打的**，不是专门看 cert。当用户问的是"网络层卡哪里"，net probe；当问的是"这个 cert 的细节"，jdan ssl cert
- **未来可能**：`internal/netprobe/tls.go` 可以反过来用 `internal/sslcert.Describe()` 升级输出 SAN 列表，零额外工作量

## 测试覆盖

35 个测试（22 sslcert package + 13 cli ssl_cert）：

| 类别 | 覆盖 |
|------|------|
| ParseTarget | host / host:port / https:// / 拒绝 http:// / 边界端口 |
| PEM round-trip | ParsePEMFile / ParsePEMBytes / EncodePEM 三角对环 |
| Describe | RSA / EC / Expired / KeyAlgorithm 识别 |
| Verify | 自签不可信 / hostname mismatch / expired / 空 hostname skip |
| FetchFromHost | httptest.NewTLSServer / 真 tls.Listen 端到端 |
| CheckOCSP | 无 OCSP URL fast-path / HTTP 500 错误路径 |
| Bundle.FullChain | dedup 系统 trust 来的 root |
| Now() | 可替换让 expires-in 测试 deterministic |
| CLI: `-f` | PEM 文件 → text/json/pem 三种输出 |
| CLI: `--expires-in` | 时间阈值 exit code 行为 + 无效 duration 错误 |
| `daysProgressBar` | 67/90 中间状态 / 0/90 空 / EXPIRED 负数 |

## 有意不做的事

| 候选功能 | 不做原因 |
|---------|---------|
| `jdan ssl diff a b` 对比两个 host 的 cert | scope 窄，可以下次单独做 |
| `jdan ssl watch` 持续监控 | `jdan ssl cert --expires-in` + cron / 脚本能覆盖 |
| `jdan ssl ct` Certificate Transparency 查询 | 需要外部 CT log API，复杂 |
| CRL revocation 检查 | OCSP 已覆盖 95% 场景，CRL 大文件慢 |
| OCSP stapling 解析（从 TLS 握手抓 response） | 复杂，直查 responder 更稳 |
| DSA 公钥单独识别 | 现代 cert 几乎不用 DSA，fallback 到 PublicKeyAlgorithm.String() 即可 |

## 依赖

- `golang.org/x/crypto/ocsp` —— Go 官方维护的 ocsp 解析库（quasi-stdlib）

## 退出码

| 状况 | exit code |
|------|-----------|
| 命令正常执行（即使 cert 不可信 / OCSP 失败 / 已过期） | 0 |
| `--expires-in <duration>` 且 leaf 在阈值内过期 | 1 |
| 参数错误 / 文件读不到 / TLS 握手失败 | 1 |

设计意图：单纯的"cert 看完了"不应该让脚本中断；想做 health gate 用 `--expires-in`。
