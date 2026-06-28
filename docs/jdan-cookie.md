# jdan cookie

解析 **Set-Cookie / Cookie** 头成可读表格，并揪出安全问题。解析直接用 stdlib
`http.ParseSetCookie`（Go 1.23+），只加审计层，**0 依赖**。

## 原理

```
Set-Cookie: sid=abc; Path=/; Secure; HttpOnly; SameSite=Lax; Max-Age=3600; Domain=.x.com
```

- **`Set-Cookie`（响应头）** = `name=value` + 一堆属性：`Path` / `Domain` / `Expires` /
  `Max-Age` / **`Secure`** / **`HttpOnly`** / **`SameSite`**。
- **`Cookie`（请求头）** = 一串 `name=value; name2=value2`，无属性。

**解析的价值在体检**：哪些属性缺了会出安全问题。解析本身交给 stdlib
`http.ParseSetCookie`（返回 `*http.Cookie`，Name/Value/Secure/HttpOnly/SameSite… 都齐），
不重造。

## 用法

```bash
jdan cookie https://example.com                              # 抓 URL 取全部 Set-Cookie
jdan cookie "sid=abc; Path=/; Secure; HttpOnly; SameSite=Lax"   # 直接给一条
echo "sid=abc; Secure" | jdan cookie                         # stdin
jdan cookie --request "a=1; b=2; sid=abc"                    # 当请求 Cookie 头（只列 pairs）
jdan cookie https://example.com --json
```

输入三选一（消歧）：**含 `=` → 当头值解析**；否则当 URL 抓（可多条 Set-Cookie）；无参 → stdin。

## 体检规则

| 命中 | 级别 | 说明 |
|------|------|------|
| 缺 `Secure` | warn | 可能经明文 HTTP 传输被窃听 |
| 缺 `HttpOnly` | warn | JS 可读，XSS 能偷 cookie |
| `SameSite=None` 无 `Secure` | warn | 浏览器直接拒收 |
| 未设 `SameSite` | info | 现代浏览器默认 Lax，建议显式声明 |
| `__Host-` 前缀但不满足 Secure+Path=/+无 Domain | warn | 前缀语义未生效 |
| `__Secure-` 前缀但无 Secure | warn | 前缀语义未生效 |
| `Domain` 以 `.` 开头 | info | 作用于所有子域，范围偏大 |

## 输出

```
$ jdan cookie "sid=abc; Path=/"
sid = abc
  Path=/ Domain=— Secure=false HttpOnly=false SameSite=(未设置)
  ⚠ 缺 Secure → 可能经明文 HTTP 传输被窃听
  ⚠ 缺 HttpOnly → JS 可读，XSS 能偷 cookie
  · 未设 SameSite（现代浏览器默认 Lax，建议显式声明）
```

`--json` 输出 `{url, cookies:[{name,value,secure,http_only,same_site,...,issues}]}`。
`--request` 只列 `name=value` 对（不审计）。

## 实现

```
internal/cookiex/cookiex.go  ParseSetCookie（包 http.ParseSetCookie）+ Audit + SameSiteName
internal/cli/cookie.go       URL→抓全部 Set-Cookie / 原值 / stdin → 解析+审计
```

- 解析靠 stdlib，审计是纯函数：合成弱 cookie 测出预期 issue；合规 cookie
  （`__Host-sid=x; Path=/; Secure; HttpOnly; SameSite=Lax`）断言无体检项。
- 抓取层用 `httptest` 假服务器（含多条 Set-Cookie）测，**不联网**。

## 有意不做

| 不做 | 原因 |
|------|------|
| Cookie 值解密 / 识别 JWT | 另一回事；`jdan jwt` 已能解 JWT |
| 改写成「更安全的 Set-Cookie」 | 越权 |

跟 `jdan csp` 是姊妹（都解析 HTTP 安全头）。
