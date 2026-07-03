# jdan http grade

给一个站点的**安全响应头**打分（A+~F），风格同 [securityheaders.com](https://securityheaders.com) / Mozilla Observatory。**0 新依赖**（复用 `http headers` 的抓取）。

## 原理

抓一次站点的响应头，看安全相关的头**有没有、配得好不好**，按加权 checklist 算总分（0-100）映射成字母等级，并逐项给 pass/warn/fail + 修复建议。全程**被动**：只读一次正常响应，不发任何探测 payload。

评级看核心 6 项（满分合计 100）：

| 头 | 权重 | 看什么 |
|----|------|--------|
| `Strict-Transport-Security` (HSTS) | 20 | 有 + `max-age≥180天` + `includeSubDomains` 才满分；太短/缺子域降级；非 HTTPS 直接 fail |
| `Content-Security-Policy` (CSP) | 25 | 有就好，但含 `unsafe-inline`/`unsafe-eval` 降级（等于给内联脚本开口子） |
| `X-Content-Type-Options` | 15 | 必须是 `nosniff` |
| `X-Frame-Options` | 15 | `DENY`/`SAMEORIGIN`，或被 CSP `frame-ancestors` 覆盖 |
| `Referrer-Policy` | 12 | 有且非 `unsafe-url` |
| `Permissions-Policy` | 13 | 有就好；只有已废弃的 `Feature-Policy` 降级 |

**信息泄露头**反向扣分：`Server` 带版本号 −5、`X-Powered-By` −5、`X-AspNet(Mvc)-Version` −3（方便攻击者按版本找 CVE）。分数不低于 0。

**跨源隔离** COOP/COEP/CORP 默认只**提示**、不计入评级（大多数站没配，纳入会一片红）；`--strict` 才计入（各缺 −5）。

等级映射：`≥95 A+ / ≥85 A / ≥70 B / ≥55 C / ≥40 D / else F`。

## 用法

```bash
jdan http grade github.com               # 无 scheme 自动补 https://
jdan http grade https://example.com --json
jdan http grade example.com --strict     # 把跨源隔离纳入评级
jdan http grade example.com --fail-under B   # 低于 B 则退出非 0（CI 卡门）
jdan http grade example.com -k           # 跳过 TLS 证书验证
```

输出示例：

```
安全响应头评级：B (74/100)  https://github.com

✓ Strict-Transport-Security    max-age=31536000; includeSubdomains; preload
⚠ Content-Security-Policy      含 unsafe-inline（削弱了防护，等于给内联脚本开口子）
                               建议：去掉 unsafe-inline，改用 nonce/hash
✓ X-Content-Type-Options       nosniff
✓ X-Frame-Options              由 CSP frame-ancestors 覆盖
✓ Referrer-Policy              strict-origin-when-cross-origin
✗ Permissions-Policy           缺失
                               建议：加 Permissions-Policy 关掉不用的浏览器特性

提示（不计入评级）：
· Cross-Origin-Opener-Policy   缺失（跨源隔离，默认不计入评级）
· Server                       github.com（未带版本号）
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--strict` | false | 把 COOP/COEP/CORP 跨源隔离纳入评级 |
| `--fail-under` | — | 低于此等级退出非 0（如 `B`），默认不卡门 |
| `--json` | false | 结构化输出 |
| `--insecure` `-k` | false | 跳过 TLS 证书验证 |
| `--max-redirects` | 10 | 最多跟几跳（评级看最终页） |
| `--timeout` | 10s | 整体超时 |

### 退出码

**默认恒 0** —— 它是评估报告，不是守门人。只有设了 `--fail-under <grade>` 且实际等级更低时才非 0（这时报告照常打印，再退出非 0 供 CI 卡门）。

> 这跟 `jdan net cdn`（检测=0/非0）、`jdan git secrets`（发现=非0）**故意不同**：那两个是「是/否」判定，这个是「评估打分」，默认不该用退出码惩罚。

## 实现

```
internal/sechdrx/sechdrx.go   Grade(http.Header, isHTTPS, opts) Report   纯函数评分
internal/cli/http_grade.go    复用 httphdr.Fetch 拿最终响应头 + 退出码
```

- **纯函数好测**：`Grade` 只吃 `http.Header`，各头的解析（HSTS max-age、CSP 削弱项、X-Frame 值）逐条判定；CLI 用 `httptest` TLS server 灌各种头组合。
- **复用抓取**：`httphdr.Fetch` 手动跟重定向，取最终一跳的响应头来评级。

## 有意不做

| 不做 | 原因 |
|------|------|
| 主动扫漏洞 / 发探测 payload / fuzz / 绕 WAF | 只读一次正常响应头，被动、非攻击性。跟 `net cdn`「只检测不揭穿源站」同一条线 |
| 代改服务器配置（生成 nginx/apache 片段） | 只评估 + 给建议，不动用户配置（可后续单独做） |
| 解析 HTML（下整页看 `<meta>` CSP） | 只看 HTTP 响应头，省带宽、边界清晰 |
| 联网拉「最佳实践库」 | 评分规则内嵌，纯本地 |

跟 `jdan http headers`（看头 + 重定向链）、`jdan http timing`（测阶段耗时）同属 `http` 套件；`headers` 给你原始头，`grade` 在其之上做安全评估。
