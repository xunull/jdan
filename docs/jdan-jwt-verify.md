# jdan jwt verify

用 HMAC 密钥校验 JWT 签名（HS256/384/512）。`jdan jwt decode` 只看内容不验签；`verify` 专做验签。0 新依赖（`crypto/hmac` + stdlib，复用 `internal/jwtdecode` 解析结构与 alg）。`jdan jwt` 下的子命令。

## 它能干什么

```bash
$ jdan jwt verify "$TOKEN" --secret mykey
✓ 签名有效 (HS256)            # 通过 → exit 0

$ jdan jwt verify "$TOKEN" --secret wrong
✗ 签名无效 (HS256)            # 失败 → exit 1
```

## 安全要点 —— 防 alg-confusion

校验**以 header.alg 为准**：
- `HS256/384/512` → 用 `--secret` 做 HMAC 校验。
- `RS*/ES*/PS*` → **报错拒绝**（这些要公钥；绝不把它们当 HMAC 验）。
- `none` / 空 alg → 报错（无签名可校验）。

这正是经典的 **alg 混淆攻击**面：服务端若拿一个 RS256 token、却用「公钥字符串」当 HMAC secret 去验，攻击者就能伪造。`jdan jwt verify` 永远不会走这条路——非 HMAC token + `--secret` 一律拒绝。

HMAC 比对走 `crypto/hmac.Equal`（**常量时间**，防时序侧信道）。

## 用法

```bash
jdan jwt verify <token> --secret <key>
echo "$TOKEN" | jdan jwt verify --secret k       # stdin
jdan jwt verify "Bearer $TOKEN" --secret k       # 自动剥 Bearer
jdan jwt verify "$TOKEN" --secret k --json       # {alg, valid}
```

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--secret` | "" | HMAC 密钥（原始字节）；必须给，仅 HS256/384/512 |
| `--json` | false | 结构化输出 `{"alg":"HS256","valid":true}` |

输入：token 参数或 stdin；自动 trim + 剥 `Bearer `。

## 退出码

| 状况 | exit code |
|------|-----------|
| 校签通过 | 0 |
| 校签失败 / 不支持的 alg / 缺 `--secret` / 非法 JWT | 1 |

校签失败 exit 1，方便脚本 `jdan jwt verify "$T" --secret "$K" && deploy`。

## 内部架构

```
internal/jwtverify/jwtverify.go
  Verify(token, secret) (alg, ok, error)   —— 复用 jwtdecode 解析 + HMAC 校验
internal/cli/jwt.go
  newJWTVerifyCommand(...)                  —— verify 子命令挂到 jdan jwt
```

复用现有 `internal/jwtdecode.Decode` 拿结构与 alg，只补 HMAC；不另起一套 JWT 解析。

## 测试

- `internal/jwtverify`：HS256 对密钥 → 有效 / 错密钥 → 无效 / HS384 / HS512 / `alg:none` 报错 / **RS256 + secret 拒绝**（防 alg 混淆）/ 篡改 payload → 无效 / Bearer 前缀剥除 / 非法 token 报错
- `internal/cli`：verify 通过(exit0) / 失败(exit1) / `--json` / stdin+Bearer / 缺 `--secret` 报错

## 有意不做

| 候选 | 原因 |
|------|------|
| RS*/ES*/PS* 公钥校验 | 需读 PEM 公钥 + 各曲线；第一版聚焦最常见的 HMAC；后续可加 `--key pub.pem` |
| `--secret-b64`（base64 密钥） | 第一版原始字节够用 |
| 校 exp/nbf 时间窗 | 那是 `decode` 的活（已显示过期/生效）；`verify` 只管签名 |

## TL;DR

1. `jdan jwt verify <token> --secret <key>` —— HMAC 校签（HS256/384/512）
2. 通过 exit 0、失败 exit 1，✓/✗ 直观，`--json` 给 `{alg, valid}`
3. **防 alg-confusion**：RS*/ES*/none + secret 一律拒绝
4. 常量时间比对，stdin + 自动剥 Bearer
5. **0 新依赖**，复用 `jwtdecode`，跟 `jdan jwt decode` 互补
