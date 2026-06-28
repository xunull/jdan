# jdan csp

解析 **Content-Security-Policy** 头成可读表格，并揪出常见弱点。一个迷你 CSP Evaluator。
**0 依赖**：纯字符串解析 + 复用现有 http 栈抓取。

## 原理

CSP 是浏览器响应头，告诉浏览器「这页只准从这些来源加载脚本/样式/图片…」，是**防 XSS /
注入**的主力。格式是**分号分隔的指令**，每条 `指令名 来源1 来源2…`：

```
default-src 'self'; script-src 'self' https://cdn.x.com 'nonce-abc'; object-src 'none'
```

- 常见指令：`default-src`（兜底）、`script-src`、`style-src`、`img-src`、`connect-src`、
  `frame-ancestors`、`base-uri`、`form-action`、`report-uri/to`…
- 来源关键字：`'self'`、`'none'`、`'unsafe-inline'`、`'unsafe-eval'`、`'nonce-…'`、
  `'sha256-…'`、`https:`、`*` 通配、主机名。

**解析的价值不在拆开，在揪弱点**：一行 CSP 难读，关键是它强不强。

## 用法

```bash
jdan csp https://example.com                                  # 抓 URL 取 CSP 头
jdan csp "default-src 'self'; script-src 'self' 'unsafe-inline'"   # 直接给头值
echo "default-src 'self'" | jdan csp                          # stdin
jdan csp https://example.com --json
```

输入三选一（消歧）：**含空格或分号 → 当头值解析**；否则当 URL 抓
`Content-Security-Policy`（缺了再试 `Content-Security-Policy-Report-Only`，标 `(Report-Only)`）；
无参 → stdin。

## 体检规则（高价值子集）

| 命中 | 级别 | 说明 |
|------|------|------|
| 缺 `default-src` | warn | 未声明的资源类型不受限 |
| `script/default/style-src` 含 `'unsafe-inline'` | warn | 内联脚本/样式可执行，CSP 几乎失效 |
| 含 `'unsafe-eval'` | warn | `eval()` 可用，XSS 防护削弱 |
| 含 `*` 通配 | warn | 允许任意来源，等于没限制 |
| 脚本源含 `data:` | warn | 可注入 `data:` URI 脚本 |
| `object-src` 非 `'none'`（或缺失且无兜底） | info | 建议 `object-src 'none'` 禁插件注入 |
| 缺 `frame-ancestors` | info | 建议加（防点击劫持，比 `X-Frame-Options` 强） |

## 输出

```
$ jdan csp "default-src 'self'; script-src 'self' 'unsafe-inline' *"
CSP 指令:
  default-src            'self'
  script-src             'self' 'unsafe-inline' *

体检:
  ⚠ script-src: 含 'unsafe-inline' → 内联脚本/样式可执行，CSP 几乎失效
  ⚠ script-src: 含 * 通配 → 允许任意来源，等于没限制
  · frame-ancestors: 缺 frame-ancestors → 建议加（防点击劫持）
```

`--json` 输出 `{url, directives, issues}`。

## 实现

```
internal/cspx/cspx.go    Parse(value)→Policy + Audit(Policy)→[]Issue   纯函数
internal/cli/csp.go      URL→fetchResponseHeader 取 CSP 头 / 原值 / stdin → 解析+审计
internal/cli/header_fetch.go  fetchResponseHeader（复用 httphdr.Fetch + EnsureScheme）
```

- `Parse` / `Audit` 是纯函数，合成弱 CSP 测出确定的 issue 集；强 CSP（`default-src 'none';
  script-src 'self'; object-src 'none'; frame-ancestors 'none'`）断言无体检项。
- 抓取层注入 client / `httptest` 假服务器，**不联网**测。

## 有意不做

| 不做 | 原因 |
|------|------|
| 全套 Google CSP-Evaluator | 几十项 bypass 检测 + CDN JSONP 黑名单，需维护数据；先做高价值子集 |
| 生成「更安全的 CSP」 | 越权，容易给错建议 |

跟 `jdan cookie` 是姊妹（都解析 HTTP 安全头）。
