# jdan secrets-scan

扫文件 / 目录 / stdin 里**疑似硬编码的密钥/token**:正则引擎(已知格式,高精度)+ 高熵引擎(未知 token,复用 `entropy`)。像 gitleaks/trufflehog 的精简版。0 新依赖(`regexp` stdlib + `entropyx`)。

## 它能干什么

```bash
$ jdan secrets-scan .
config/prod.env:7   [aws-access-key]  AKIA…J7QF  (high)
src/client.go:42    [generic-assign]  Xy9K…P6dC  (medium)
deploy.sh:3         [high-entropy]    dGhp…YWVo  (low, entropy 4.6)

共 3 处疑似密钥（已脱敏；exit 1）
```

## 用法

| 形式 | 含义 |
|------|------|
| `jdan secrets-scan <path...>` | 扫文件/目录(递归) |
| `cat x \| jdan secrets-scan` | 扫 stdin |
| `-a, --all` | 不跳过 `.git`/`node_modules`/二进制/lock 文件 |
| `--no-entropy` | 只用正则引擎(关高熵) |
| `--min-entropy <f>` | 高熵阈值(bits/byte),默认 4.0,越高越保守 |
| `--json` | 结构化输出(同样不含完整 secret) |

## 安全铁律(命门)

**输出永不含完整 secret**,只给脱敏预览(前 4…后 4)。一个把完整密钥打到 stdout / CI 日志 / 终端回滚的扫描器,本身就是一次泄露。跟 `jdan pem` 同原则,有**安全测试**塞真实格式假密钥、断言完整串不出现在任何输出(text/json)。

`--json` 也只给 `redacted` 字段,绝无完整值。

## 两个引擎

- **正则引擎(高精度)**:约 18 条内嵌规则覆盖常见提供方 —— AWS `AKIA…`、GitHub `ghp_…`/fine-grained、GitLab `glpat-…`、Slack `xox[baprs]-…`/webhook、Stripe `sk_live_…`、Google `AIza…`/OAuth、Twilio、SendGrid、npm、OpenAI `sk-…`、Anthropic `sk-ant-…`、私钥头 `-----BEGIN … PRIVATE KEY-----`、JWT、URL 内嵌 `user:pass@`、泛化赋值 `api_key = "…"`。每条 regexp 编译一次。
- **高熵引擎(高召回)**:不匹配已知格式、但「长得像随机串」的 base64/hex token —— 复用 `entropyx.Shannon`,默认阈值 4.0 bits/byte、最小长度 20、且要求字母+数字混合(排除纯单词/纯数字/版本号/IP),命中标 **low** 置信。

## 降噪(假阳是头号敌人)

- **内嵌 allowlist**:UUID、示例占位(`AKIAIOSFODNN7EXAMPLE`、`changeme`、`your-key`、`<...>`、`xxxx`)直接跳过。
- **行内豁免**:行里含 `# pragma: allowlist secret` → 整行跳过(gitleaks/detect-secrets 惯例)。
- **泛化赋值降噪**:`password = "iloveyou"` 这种全小写无数字的赋值多半是占位,跳过。
- **去重**:已被正则命中的 token 不再被高熵重复报。
- **`--min-entropy` 调高** / **`--no-entropy` 关掉** 高熵引擎。

## 扫描范围 & 跳过

走目录默认跳过:`.git node_modules vendor dist build .next target .venv venv __pycache__ .idea`;lock 文件(`package-lock.json`/`yarn.lock`/`go.sum`/`Cargo.lock`…);**二进制文件**(头部探到 NUL);**超大文件**(>5 MiB)。`-a` 全扫。显式指定的文件照扫(只守大小/二进制)。

## 退出码(CI 友好)

| 状况 | exit |
|------|------|
| 无发现 | 0 |
| 有发现 | 1(CI 可据此卡门,像 linter) |
| 出错 | 2 |

## 内部架构 & 可测性

```
internal/secretscan/secretscan.go
  Rule{Name, Re, Confidence, ValueGroup} + 内嵌规则表
  ScanBytes(data, opts) []Finding          —— 正则 + 逐行高熵，纯函数
  Finding{Rule, Line, Col, Redacted, Entropy, Confidence}
  Redact / isAllowlisted / looksWeak / looksLikeSecret / splitTokens
  复用 entropyx.Shannon
internal/cli/secrets_scan.go                —— walk/stdin、跳过规则、退出码(注入 exit)、--json
```

`ScanBytes` 纯函数全测;CLI 把 `os.Exit` 做成可注入的 `exit func(int)`,测试捕获退出码而不真退出。

## 测试

- `internal/secretscan`:每条主要规则一个正向样例;**安全测试(脱敏后绝不含完整 secret)**;Redact(前4…后4 / 短串整体打码);allowlist(UUID/示例/changeme);pragma 豁免;泛化赋值跳弱口令;高熵引擎(命中 low / 纯单词不报 / `--no-entropy` 关闭);正则↔高熵去重
- `internal/cli`:文件命中 exit 1、干净 exit 0、stdin、`--json` 合法且不泄露、跳过 node_modules(`-a` 才扫)、跳过二进制

## 有意不做

| 候选 | 原因 |
|------|------|
| 验证密钥是否「活的」(打 API 试) | 要联网 + 拿你的密钥发请求,危险且越权;只做静态识别 |
| git 历史扫描 | v1 defer(遍历每个 commit 复杂度大),后续 `--history` |
| 自动修复/吊销 | 越权;只报告 |
| SARIF 输出 | 重;先简单 JSON |

## TL;DR

1. `jdan secrets-scan .` 扫目录;`cat x \| jdan secrets-scan` 扫 stdin
2. 正则(已知格式)+ 高熵(未知 token,复用 entropy)双引擎
3. **输出永不含完整 secret**(前4…后4),扫描器不能变成泄露(安全测试钉死)
4. 降噪:allowlist + 行内 pragma + 弱口令跳过 + 去重 + `--min-entropy`
5. exit 0/1/2(CI 卡门);默认跳 `.git`/`node_modules`/二进制/lock,`-a` 全扫;**0 新依赖**
