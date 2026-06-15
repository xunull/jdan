# jdan ssh-key

SSH 公钥/私钥解析工具。3 个子命令覆盖**综合信息 / fingerprint / 公钥提取**。0 新依赖（`golang.org/x/crypto/ssh` 已是 jdan 的 direct dep），跟 `jdan ssl` 套件并列成"密钥/证书检查"工具。

## 它解决什么问题

`ssh-keygen` 能做这些，但语法零碎要记：

```bash
ssh-keygen -lf key.pub            # fingerprint SHA256
ssh-keygen -lf key.pub -E md5     # fingerprint MD5
ssh-keygen -y -f id_ed25519       # 私钥 → 公钥
# 想一次看到"类型 + 位数 + fingerprint + comment"？没有单命令，要拼
```

`jdan ssh-key` 提供统一接口：
- **一个 `info` 命令** 出全部关键字段（类型/算法/位数/双格式 fingerprint/comment）
- **自动识别公钥 vs 私钥**，不用记 `-f` / `-y` / `-l` 哪个该用
- **fingerprint 跟 ssh-keygen byte-equal**，能交叉验证 GitHub / server

## 子命令一览

| 子命令 | 用途 |
|--------|------|
| `info <key>` | 综合信息（吃公钥或私钥） |
| `fingerprint <key>` | 只出 fingerprint（SHA256 默认，`--md5` 切换） |
| `pubkey <privkey>` | 从私钥提取公钥（= `ssh-keygen -y`） |

**输入来源**（所有子命令统一）：文件路径 / `-` stdin / 直接粘贴公钥字符串。

## info

**最常用** 的命令。自动识别公钥还是私钥。

### 公钥

```bash
$ jdan ssh-key info ~/.ssh/id_ed25519.pub
Type:         ssh-ed25519
Algorithm:    Ed25519
Bits:         256
Comment:      quincy@macbook
Fingerprint:  SHA256:Hk8x...（跟 GitHub / ssh-keygen 对齐）
MD5:          MD5:43:51:43:a1:...（legacy 格式，跟老 server 对齐）
```

RSA 公钥额外显示真实位数（从 modulus 算）：

```bash
$ jdan ssh-key info ~/.ssh/id_rsa.pub
Type:         ssh-rsa
Algorithm:    RSA
Bits:         4096
...
```

### 私钥

```bash
$ jdan ssh-key info ~/.ssh/id_ed25519
Type:         OpenSSH private key
Algorithm:    Ed25519
Bits:         256
Comment:      quincy@macbook
Fingerprint:  SHA256:Hk8x...
MD5:          MD5:43:51:...
Public key:   ssh-ed25519 AAAAC3Nz... quincy@macbook
```

私钥的 fingerprint 跟它对应的公钥**完全一致**（fingerprint 只跟 public key material 有关）。

> **comment 从哪来**：OpenSSH 私钥 blob 里存了 comment，但 Go 的 `x/crypto/ssh`
> 不暴露解析它的 API。所以 `jdan ssh-key` 在私钥同目录找 `<name>.pub` 文件回退
> 读取 comment。没有 `.pub` 时 comment 为空（功能不受影响）。

### 加密私钥（passphrase 保护）

不解密时只识别，不泄露任何 key material：

```bash
$ jdan ssh-key info ~/.ssh/id_ed25519   # 假设有 passphrase
Type:         OpenSSH private key
Encrypted:    yes (passphrase-protected; cannot derive public key without it)
```

给 `--passphrase` 解密后看完整信息：

```bash
$ jdan ssh-key info ~/.ssh/id_ed25519 --passphrase 'mypass'
Type:         OpenSSH private key
Algorithm:    Ed25519
...
```

### JSON 输出

```bash
$ jdan ssh-key info ~/.ssh/id_rsa.pub --json
{
  "kind": "public",
  "type": "ssh-rsa",
  "algorithm": "RSA",
  "bits": 4096,
  "comment": "quincy@macbook",
  "fingerprint_sha256": "SHA256:...",
  "fingerprint_md5": "MD5:..."
}
```

### 直接粘贴公钥

```bash
$ jdan ssh-key info "ssh-ed25519 AAAAC3Nz... someone@host"
```

以 `ssh-` / `ecdsa-` / `sk-` 开头的参数当作 inline 公钥而非文件路径。

## fingerprint

格式跟 `ssh-keygen -lf` **byte-equal**，能交叉验证。

```bash
$ jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
256 SHA256:Hk8x... quincy@macbook (ED25519)

$ jdan ssh-key fingerprint ~/.ssh/id_rsa.pub --md5
4096 MD5:43:51:... quincy@macbook (RSA)
```

私钥也吃（导出公钥后算）：

```bash
$ jdan ssh-key fingerprint ~/.ssh/id_ed25519
256 SHA256:Hk8x... quincy@macbook (ED25519)
```

**用途**：验证本地 key 跟 GitHub / GitLab / server 上注册的是同一把：

```bash
$ jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
256 SHA256:Hk8x...
# ← 跟 GitHub Settings → SSH keys 页面显示的 SHA256 对比
```

## pubkey

从私钥导出公钥（authorized_keys 格式），= `ssh-keygen -y`：

```bash
$ jdan ssh-key pubkey ~/.ssh/id_ed25519
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... quincy@macbook
```

加密私钥需要 passphrase：

```bash
$ jdan ssh-key pubkey ~/.ssh/id_ed25519 --passphrase 'mypass'
```

**用途**：丢了 `.pub` 文件但还有私钥时重建公钥；或把私钥的公钥部分贴到 server。

## 支持的密钥类型

| key type | Algorithm | Bits |
|----------|-----------|------|
| `ssh-ed25519` | Ed25519 | 256 |
| `ssh-rsa` | RSA | 从 modulus 算（2048/3072/4096） |
| `ecdsa-sha2-nistp256` | ECDSA | 256 |
| `ecdsa-sha2-nistp384` | ECDSA | 384 |
| `ecdsa-sha2-nistp521` | ECDSA | 521 |
| `sk-ssh-ed25519@openssh.com` | Ed25519 | 256（FIDO/U2F 硬件密钥） |
| `sk-ecdsa-sha2-nistp256@openssh.com` | ECDSA | 256（FIDO/U2F 硬件密钥） |

FIDO/U2F 硬件密钥（`sk-*`）会额外标 `Hardware: yes`。

## flags

| 子命令 | flag | 用途 |
|--------|------|------|
| `info` | `--json` | 结构化输出 |
| `info` | `--passphrase` | 解密加密私钥看完整信息 |
| `fingerprint` | `--md5` | MD5 colon-hex 格式（默认 SHA256 base64） |
| `fingerprint` | `--json` | 结构化输出 |
| `fingerprint` | `--passphrase` | 加密私钥的口令 |
| `pubkey` | `--passphrase` | 加密私钥的口令 |

## 跟其他 jdan 命令的关系

```bash
# jdan ssl cert 看 TLS 证书；jdan ssh-key 看 SSH 密钥 —— 并列的"密钥/证书检查"
jdan ssl cert github.com           # 服务器 TLS 证书
jdan ssh-key info ~/.ssh/id_rsa    # 本地 SSH 密钥

# 验证推到 server 的 key 跟本地一致（fingerprint 交叉对比）
jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
```

## 内部架构

```
internal/sshkey/
  parse.go        IsPrivateKey / ParsePublicKey / ParsePrivateKey /
                  IsEncryptedPrivateKey（自动识别 + 加密检测）
  fingerprint.go  FingerprintSHA256 / FingerprintMD5（ssh.Fingerprint* 包装）
  info.go         KeyInfo + 算法/位数提取 + String() 渲染
  pubkey.go       PublicKeyLine（私钥 → authorized_keys 行）
  testdata/       真实密钥对 fixture（ed25519/rsa2048/ecdsa256 + 加密版）

internal/cli/ssh_key.go   父命令 + 3 子命令
```

**0 新依赖**：`golang.org/x/crypto/ssh` 之前就是 jdan 的 direct dep（被 sslcert 用）。

## 测试

- 17 unit tests on `internal/sshkey`：
  - fingerprint SHA256/MD5 byte-equal 对齐 ssh-keygen（ed25519/rsa/ecdsa 三种）
  - 算法/位数提取（RSA modulus → 2048，ECDSA 曲线 → 256）
  - 加密私钥识别 + 正确/错误 passphrase 解密
  - 私钥导出公钥 byte-equal 对齐 `ssh-keygen -y`
- 19 CLI tests：
  - info 公钥/私钥/加密/JSON/inline-paste/stdin
  - fingerprint SHA256/MD5/私钥/加密报错/JSON
  - pubkey 正常/拒绝公钥/加密 passphrase

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| key 解析失败 / 文件不存在 | 1 |
| 加密私钥没给 passphrase（fingerprint/pubkey） | 1 |
| pubkey 输入是公钥而非私钥 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| 密钥生成（keygen） | `ssh-keygen` 已经很好；jdan 偏"检查/解析"不偏"生成" |
| 格式转换（PEM ↔ OpenSSH ↔ PKCS8） | scope 大，需求频率低 |
| known_hosts 管理 | 系统层操作，出 jdan 纯函数 scope |
| SSH CA 证书签名 | 企业场景，scope 太大 |
| 私钥 comment 直接解析 | x/crypto 不暴露 API；回退读 .pub 已覆盖大多数场景 |

## TL;DR

1. `jdan ssh-key info <key>` —— 公钥/私钥都吃，一键出类型/位数/双 fingerprint
2. `jdan ssh-key fingerprint <key>` —— byte-equal 对齐 ssh-keygen，验证 GitHub/server
3. `jdan ssh-key pubkey <privkey>` —— 私钥重建公钥（= ssh-keygen -y）
4. 加密私钥识别 + `--passphrase` 解密，不泄露 key material
5. 支持 Ed25519 / RSA / ECDSA + FIDO/U2F 硬件密钥（sk-*）
6. **0 新依赖**，纯 x/crypto/ssh
