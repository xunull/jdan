# jdan pem

离线检视一个 PEM 文件：把每个 PEM 块拆出来、认出类型、给摘要。**不联网、绝不打印私钥内容**。0 新依赖（复用 `internal/sslcert` 的证书描述）。

## 它能干什么

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
```

## 与现有命令的关系

| 命令 | 干啥 |
|------|------|
| `jdan ssl cert <host>` | **联网**抓 host 的 TLS 证书 |
| `jdan cert` | **生成**自签证书 |
| **`jdan pem <file>`** | **离线**读本地 PEM 文件，认每个块是啥 |

CERTIFICATE 块复用 `sslcert.Describe`，不重写证书解析。

## 各块型

| PEM 块 | 摘要 |
|--------|------|
| `CERTIFICATE` | subject / issuer / 有效期 + 剩余天数 / SAN / key 类型 / CA / SHA-256 指纹（复用 `sslcert.Describe`） |
| `CERTIFICATE REQUEST`（CSR） | subject / SAN / key 类型 / 签名算法 |
| `RSA/EC/PRIVATE KEY`（PKCS1/PKCS8/EC） | **只给** key 类型 + 位数（RSA 2048 / EC P-256 / Ed25519），**绝不输出密钥字节** |
| `PUBLIC KEY` | key 类型 + 位数 |
| 加密私钥（DEK-Info / `ENCRYPTED PRIVATE KEY`） | 标「已加密」，不解密、不问口令 |
| 其它（EC/DH PARAMETERS、CRL…） | 类型 + 字节数 |
| 解析失败的块 | 行内标「解析失败：原因」，**继续**下一块 |

## key ↔ cert 匹配检查

文件里**正好 1 个叶子证书 + 1 个私钥**时，比对公钥给结论：

```bash
$ cat cert.pem key.pem | jdan pem | tail -1
✓ 私钥与证书匹配          # 不配则 ✗ 私钥与证书不匹配
```

运维高频问题（「这个 key 是配这个 cert 的吗」）。公钥比对走各 key 类型自带的 `Equal`（RSA/ECDSA/Ed25519），不碰私钥材料。

## 用法

```bash
jdan pem <file>          # 检视
cat cert.pem | jdan pem  # stdin
jdan pem bundle.pem --json
```

| flag | 作用 |
|------|------|
| `--json` | 结构化输出（blocks 数组 + key_matches_cert） |

## 安全

- **绝不打印私钥材料**：私钥块只暴露算法 + 位数，输出（text & JSON）里没有任何密钥字节。有专门测试断言输出中不含长 base64 串。
- 加密私钥只识别「已加密」，不尝试解密、不提示口令。

## 退出码

| 状况 | exit code |
|------|-----------|
| 有 ≥1 个 PEM 块（即使个别块解析失败、证书过期） | 0 |
| 无 PEM 块 / 文件读不了 | 1 |

inspector 取向：过期、单块解析失败都是信息不是错，不翻 exit。

## 内部架构 & 可测性

```
internal/pemx/pemx.go
  Inspect(data) (Result, error)        —— pem.Decode 循环 + 按块型分发（纯函数）
  (Result).FormatText / .FormatJSON
  pubKeyAlg / privKeyAlg               —— RSA/EC/Ed25519 类型+位数（不碰私钥字节）
  keyMatch                             —— 叶子证书 + 私钥公钥比对
internal/cli/pem.go                     —— jdan pem [file]
```

测试用 `internal/certgen` 现造 cert+key（RSA/EC/Ed25519），不硬编码大块 PEM。

## 测试

- `internal/pemx`：单证书 / fullchain 多证书 / RSA·EC·Ed25519 私钥（只给类型）/ 公钥 / CSR / 旧式加密(DEK-Info)·PKCS8 加密私钥 / 其它块 / 坏证书块容错 / 无 PEM 报错；**key↔cert 匹配·不匹配·歧义时不判**；**安全断言：输出绝无私钥 base64 体**；FormatJSON 合法
- `internal/cli`：text / `--json` / 无 PEM 报错 / **CLI 输出无私钥泄漏**

## 有意不做

| 候选 | 原因 |
|------|------|
| 解密加密私钥（问口令） | 第一版只识别「已加密」；解密引入交互 + 口令处理 |
| `--type` 过滤块型 | 第一版全列；需要再加 |
| 完整解析 CRL 条目 | 第一版只给类型+大小 |
| DER ↔ PEM 转换 | 那是 openssl 的活；本命令专注检视 |

## TL;DR

1. `jdan pem <file>` —— 离线拆 PEM，认每个块（证书/CSR/私钥/公钥/加密/其它）
2. CERTIFICATE 复用 `sslcert.Describe`，摘要齐全
3. **绝不打印私钥**，私钥块只给类型+位数
4. cert+key 合一时自动判 **key↔cert 是否匹配**
5. **0 新依赖**，跟 `ssl cert`(联网)/`cert`(生成) 互补
