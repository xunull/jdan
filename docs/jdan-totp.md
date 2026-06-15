# jdan totp

TOTP 2FA 验证码工具（RFC 6238）。3 个子命令覆盖**生成 / 解析 otpauth URI / 验证**。兼容 Google Authenticator / Authy / 1Password。0 新依赖（纯 stdlib `crypto/hmac` + `encoding/base32`），跟 `jdan ssl` / `jdan jwt` / `jdan ssh-key` 拼成 security toolkit。

## 它解决什么问题

每次登录要 2FA 时，得：掏手机 → 打开 Authenticator app → 找到对应条目 → 念 6 位数 → 手动敲进去。如果你的 secret 已经在本机（dotfiles / 密码管理器导出 / CI secret），完全可以 CLI 直接出码：

```bash
$ jdan totp code JBSWY3DPEHPK3PXP
283461   (expires in 17s)
```

秒杀 GUI app 切换。脚本里还能自动填 2FA（CI / 自动化测试）。

## ⚠️ 安全须知（先读这个）

**secret 是长期凭证**，比一次性的 6 位码敏感得多——谁拿到 secret 就能永久生成你的 2FA 码。

直接 `jdan totp code <secret>` 会让 secret：
- 进 **shell history**（`~/.zsh_history` / `~/.bash_history`）
- 进 **进程列表**（别人 `ps aux` 看得到）

所以**直接传 arg 只适合临时/测试**。长期用请走更安全的来源：

```bash
# 1. stdin（不进 history）
$ echo "$SECRET" | jdan totp code -

# 2. 环境变量（从密码管理器/CI secret 注入）
$ JDAN_TOTP_SECRET="$SECRET" jdan totp code

# 3. 不带参数 → 从 stdin 交互读
$ jdan totp code
```

secret 来源优先级：**arg > 环境变量 `JDAN_TOTP_SECRET` > stdin**。

## 子命令一览

| 子命令 | 用途 |
|--------|------|
| `code <secret>` | 生成当前 6 位码 |
| `uri <otpauth://...>` | 解析 otpauth URI 并生成码（扫码得到的格式） |
| `verify <secret> <code>` | 验证一个码是否有效（退出码 0/1） |

## code

```bash
$ jdan totp code JBSWY3DPEHPK3PXP
283461   (expires in 17s)

$ jdan totp code JBSWY3DPEHPK3PXP --json
{
  "code": "283461",
  "expires_in": 17,
  "period": 30,
  "digits": 6
}
```

**base32 secret 容错**：实际复制 secret 时常带噪音，全部自动处理：
- 小写（`jbswy3dp` → 转大写）
- 空格分组（`JBSW Y3DP EHPK 3PXP` —— 这是 Google 的显示格式）
- 缺 padding（base32 标准要 `=`，多数服务省略 → 自动补）

## uri

`otpauth://` URI 是扫二维码得到的标准格式，含 secret + 全部参数。从 GitHub / Google 2FA setup 页面拿到的二维码，解码后就是这个：

```bash
$ jdan totp uri "otpauth://totp/GitHub:quincy?secret=JBSWY3DPEHPK3PXP&issuer=GitHub&digits=6&period=30"
Issuer:    GitHub
Account:   quincy
Algorithm: SHA1
Digits:    6
Period:    30s
Code:      283461   (expires in 17s)
```

URI 里的参数（algorithm / digits / period）会**自动用上**，不用手动指定。`--json` 给脚本消费。

跟 `jdan qr` 配套：你可以反过来把 otpauth URI 生成二维码导入别的 app。

## verify

验证一个码是否有效，**退出码 0/1** 可做脚本 gate：

```bash
$ jdan totp verify JBSWY3DPEHPK3PXP 283461
✓ valid
$ echo $?
0

$ jdan totp verify JBSWY3DPEHPK3PXP 999999
✗ invalid
$ echo $?
1
```

`--window N`（默认 1）容许前后各 N 个时间窗，对齐客户端/服务端的**时钟漂移**。`--window 0` 严格只接受当前窗。

## 参数（对齐 Google Authenticator）

| 参数 | 默认 | flag |
|------|------|------|
| algorithm | SHA1 | `--algo sha1/sha256/sha512` |
| digits | 6 | `--digits 6/8` |
| period | 30s | `--period 30` |

这些默认值跟 Google Authenticator / Authy / 1Password / Microsoft Authenticator 一致，绝大多数服务直接能用。少数用 SHA256 或 8 位码的服务（部分银行）用 flag 覆盖，或直接走 `uri`（URI 里带参数）。

## 跟现有命令的关系

```bash
# 从 2FA setup 二维码拿到 URI → 直接出码
jdan totp uri "$(扫码工具输出的 otpauth URI)"

# 脚本/CI 自动填 2FA
code=$(JDAN_TOTP_SECRET="$TOTP_SECRET" jdan totp code)
curl -X POST ... -d "otp=$code"

# 跟 ssl / jwt / ssh-key 并列成 security toolkit
jdan ssl cert github.com        # TLS 证书
jdan jwt decode "$TOKEN"        # JWT
jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
jdan totp code "$SECRET"        # 2FA
```

## 内部架构

```
internal/totp/
  secret.go   DecodeSecret（base32 容错：小写/空格/缺 padding）
  hotp.go     HOTP（RFC 4226 dynamic truncation）+ Algorithm
  totp.go     Config / GenerateAt / ExpiresInAt / VerifyAt（常量时间比较）
  uri.go      ParseOtpauthURI（标准 otpauth:// 解析）

internal/cli/totp.go   父命令 + 3 子命令；secret 来源 arg/env/stdin
```

**0 新依赖**：`crypto/hmac` + `crypto/sha1/256/512` + `encoding/base32` + `net/url` 都是 stdlib。

## 测试

跟 **RFC 6238 / RFC 4226 官方测试向量 byte-equal**（这是 TOTP 实现的金标准）：

- 22 unit tests on `internal/totp`：
  - HOTP RFC 4226 Appendix D（counter 0-9 的标准码）
  - TOTP RFC 6238 Appendix B（SHA1/SHA256/SHA512 在指定时间戳的 8 位码）
  - base32 解码容错（小写 / 空格 / 缺 padding / round-trip）
  - otpauth URI 解析（完整 / 缺省参数 / issuer-from-label / 非法 / HOTP 拒绝）
  - verify 时间窗（当前 / ±1 窗 / window=0 严格）
- 17 CLI tests（注入固定时间断言精确 RFC 码）：
  - code RFC 向量 / 8 位 / SHA256 / JSON / env / stdin / 无 secret 报错
  - uri 完整 / JSON / 非法
  - verify valid（exit 0）/ invalid（exit 1）/ window 容错

## 退出码

| 状况 | exit code |
|------|-----------|
| code / uri 成功 | 0 |
| `verify` 码有效 | 0 |
| `verify` 码无效 | 1（脚本 gate 用） |
| secret 解析失败 / URI 非法 / 无 secret | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| secret 加密存储（vault） | scope 大，需要 key 管理；jdan 偏无状态工具，secret 由用户的密码管理器管 |
| HOTP（counter-based）单独命令 | TOTP 是 99% 场景；HOTP 是内部实现细节 |
| 生成 otpauth URI / 二维码 | `jdan qr` 已能出二维码；URI 拼接用户可手写 |
| Steam Guard / 自定义字母表 | 小众；标准 TOTP 覆盖绝大多数服务 |
| `--watch` 持续刷新 | 第一版省略（`expires_in` 已够判断）；未来可加 |

## TL;DR

1. `jdan totp code <secret>` —— 当前 2FA 码 + 剩余秒数
2. `jdan totp uri "otpauth://..."` —— 解析扫码得到的 URI，参数自动用上
3. `jdan totp verify <secret> <code>` —— 退出码 0/1 做 gate，`--window` 容时钟漂移
4. **secret 走 stdin / `JDAN_TOTP_SECRET` 环境变量**，别直接传 arg（进 history + ps）
5. 默认对齐 Google Authenticator（SHA1 / 6 位 / 30s）；base32 secret 容错
6. RFC 6238/4226 官方向量 byte-equal；**0 新依赖**
