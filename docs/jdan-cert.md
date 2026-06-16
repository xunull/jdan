# jdan cert

生成本地开发 / 测试用的自签名 TLS 证书。0 新依赖（纯 stdlib `crypto/x509` + `crypto/tls`）。跟 `jdan ssl cert`（检查证书）互补：一个造，一个看。

> ⚠ **仅限本地开发 / 测试**，不要用于生产。生产证书走 ACME / certbot / Let's Encrypt。

## 它解决什么问题

本地 HTTPS 调试要自签证书，`openssl req` 的 flag 谁都记不住：

```bash
# 谁记得住这一长串？而且漏了 SAN 现代浏览器还不认
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem \
  -days 365 -nodes -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

`jdan cert localhost` 一行搞定，**默认就带正确的 SAN**（现代浏览器要求 SAN，光 CN 不行）：

```bash
$ jdan cert localhost
Generated self-signed certificate:
  Cert:        cert.pem
  Key:         cert-key.pem
  Subject:     CN=localhost
  SAN:         DNS:localhost
  Key type:    EC (P-256)
  Valid:       2026-06-16 → 2028-09-18 (825 days)
  Fingerprint: SHA256:SBHHayJXJqmvul4JtBRmUDriUDgFz7rc996uGk46NHA

⚠ Self-signed: browsers will warn. Add cert.pem to your trust store,
  or use --ca to generate a CA you can trust once. Local dev only.
```

默认输出 `cert.pem` + `cert-key.pem`（key 文件权限 `0600`）。

## SAN 自动推断

主参数自动进 SAN：**是 IP 字面量进 IP SAN，否则进 DNS SAN**。

```bash
$ jdan cert myapp --ip 127.0.0.1,::1 --san "*.myapp.local"
  ...
  SAN:         DNS:myapp, DNS:*.myapp.local, IP:127.0.0.1, IP:::1
```

- `--san` 额外 DNS SAN（csv，支持 `*.example.com` 通配）
- `--ip` IP SAN（csv，`127.0.0.1,::1`）
- 自动去重，保持顺序

## --ca 模式（本地信任更方便）

自签证书每个都要单独信任。`--ca` 生成一个 **CA + 用它签的 leaf**：你只需把 **CA** 加进信任库**一次**，之后所有这个 CA 签的 cert 都被信任。

```bash
$ jdan cert localhost --ca
Generated CA + leaf certificate:
  CA cert:     ca.pem        ← 加这个到信任库（一次）
  CA key:      ca-key.pem
  Leaf cert:   cert.pem
  Leaf key:    cert-key.pem
  ...
```

输出 4 个文件：`ca.pem` / `ca-key.pem` / `cert.pem` / `cert-key.pem`。把 `ca.pem` 加进系统/浏览器信任库后，所有用它签的 leaf 都不再弹警告。

> CA key（`ca-key.pem`）= 能签任意证书，妥善保管，别提交到 git。

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--san` | 含主参数 | 额外 DNS SAN（csv，支持通配） |
| `--ip` | 无 | IP SAN（csv） |
| `--days` | 825 | 有效期天数（825 是浏览器接受的 leaf 上限） |
| `--key-type` | ec | `ec`（P-256，快/小）/ `rsa`（2048）/ `ed25519` |
| `--out-dir` | `.` | 输出目录 |
| `--prefix` | cert | 文件名前缀（→ `<prefix>.pem` / `<prefix>-key.pem`） |
| `--ca` | false | 同时生成 CA 并用它签发 |
| `--stdout` | false | 输出到 stdout（cert 在前，key 在后） |
| `--json` | false | 输出元信息 JSON |

## 密钥类型

| `--key-type` | 算法 | 特点 |
|--------------|------|------|
| `ec`（默认） | EC P-256 | 快、密钥小，现代首选 |
| `rsa` | RSA 2048 | 兼容性最好 |
| `ed25519` | Ed25519 | 最现代，但部分老客户端不支持 |

PKCS#8 PEM 编码（`-----BEGIN PRIVATE KEY-----`）。

## 输出模式

```bash
# 默认：写文件
$ jdan cert localhost

# stdout（cert 在前，key 在后，方便管道）
$ jdan cert localhost --stdout > combined.pem

# JSON 元信息（路径 / fingerprint / SAN / 有效期）
$ jdan cert localhost --json
{
  "cert": "cert.pem",
  "key": "cert-key.pem",
  "subject": "localhost",
  "san": "DNS:localhost",
  "key_type": "EC (P-256)",
  "not_before": "2026-06-16T...",
  "not_after": "2028-09-18T...",
  "fingerprint": "SHA256:...",
  "self_signed": true
}
```

## 跟 jdan ssl cert 配套（造 → 看闭环）

```bash
# 生成
jdan cert localhost

# 用 ssl cert 检查刚生成的（完整闭环）
jdan ssl cert -f cert.pem

# 给本地 HTTPS 服务器用
your-server --tls-cert cert.pem --tls-key cert-key.pem
```

`jdan cert` 造，`jdan ssl cert` 看。fingerprint 两边对齐。

## 安全设计

- 私钥文件权限 `0600`（只有 owner 能读）
- CA key 同样 0600
- 输出明确警告：**仅限本地开发/测试**
- leaf 证书带 `ServerAuth` EKU + `KeyUsageDigitalSignature`，符合 TLS server 要求
- CA 带 `IsCA` + `MaxPathLenZero`（不能再签子 CA）

## 内部架构

```
internal/certgen/
  keys.go      KeyType + generateKey（EC/RSA/Ed25519）+ PKCS#8 PEM 编码
  san.go       BuildSANs（主参数 IP vs DNS 推断 + 去重）
  generate.go  GenerateSelfSigned / GenerateCA / CA.SignLeaf；x509 模板
  info.go      FingerprintSHA256 / SANString

internal/cli/cert.go
```

**0 新依赖**，全 stdlib crypto。

## 测试

- 14 unit tests on `internal/certgen`：
  - ParseKeyType / BuildSANs（IP vs DNS / 去重）
  - GenerateSelfSigned 解析回来 / 三种 key type keypair 配对 / 有效期（注入时间）/
    默认 825 天 / ServerAuth EKU
  - GenerateCA IsCA / SignLeaf 能被 CA `Verify`
  - **端到端**：用生成的 cert 起真实 `tls.Listen` + client 握手成功
  - Fingerprint / SANString
- 9 CLI tests：文件生成 / **key 权限 0600** / IP+SAN / --ca（leaf 由 CA 签）/
  三种 key type / 非法 key type / --stdout / --json / 默认渲染

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 非法 key type / 写文件失败 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| CSR 生成 + 提交真 CA | 生产证书走 certbot / ACME |
| ACME / Let's Encrypt 客户端 | 单独大 scope；certbot 已经很好 |
| 证书续期 / 轮换 | 有状态；本地开发重新生成即可 |
| 自动装进系统信任库 | 平台特定（security add-trusted-cert / update-ca-certificates） |
| PKCS#12 / .pfx 导出 | 第一版 PEM 够用；未来可加 |

## TL;DR

1. `jdan cert localhost` —— 一行出 cert+key，默认带正确 SAN（替代记不住的 openssl）
2. `--san` / `--ip` 加 SAN，主参数自动推断 IP vs DNS
3. `--ca` 生成 CA，信任一次之后所有它签的都被信任
4. `--key-type ec/rsa/ed25519`，key 文件权限 0600
5. 跟 `jdan ssl cert` 闭环：造 → 看；**仅限本地开发**
6. **0 新依赖**，纯 stdlib crypto
