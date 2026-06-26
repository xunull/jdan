# jdan http headers

拉一个 URL，打印**状态行 + 响应头 + 完整重定向链**（逐跳显示）。比 `curl -I` 好读。0 新依赖（纯 stdlib `net/http`）。

跟 `jdan http timing`（测请求各阶段耗时）互补，做成 `jdan http headers`。

## 它能干什么

```bash
$ jdan http headers http://github.com
301 Moved Permanently
  Location: https://github.com/
→ 200 OK
  Cache-Control: max-age=0, private, must-revalidate
  Content-Type: text/html; charset=utf-8
  Server: github.com
  Strict-Transport-Security: max-age=31536000; includeSubdomains; preload
  ...
```

排查「重定向去哪了 / 响应头对不对 / HSTS / CORS / 缓存头 / Set-Cookie」高频。

## 核心设计

1. **手动跟重定向**（不靠 client 自动跟）：`CheckRedirect` 设成 `ErrUseLastResponse`，自己循环——每跳记录 status + 全部响应头，遇 3xx + `Location` 且没超 `--max-redirects` 就解析下一跳。这样能**逐跳**展示每一跳的头，自动跟转做不到。
2. **默认 GET 但只读响应头、不下载 body**：`net/http` 拿到响应头后 body 才惰性下载，拿到 `Header` 后直接 `Body.Close()`——既不下载正文，又避开 `HEAD` 的怪行为（有些服务器 HEAD 返回不一样或 405）。`-X` 可改方法。
3. **相对 / 绝对 `Location` 都正确解析**（`base.Parse(loc)`，相对路径用当前 URL 解析）。

## 用法

```bash
jdan http headers <url>
jdan http headers github.com                  # 无 scheme 自动补 https://
jdan http headers <url> --max-redirects 0      # 不跟转，只看第一跳
jdan http headers <url> -a                     # 每一跳都打全部头
jdan http headers <url> -X HEAD                # 改请求方法
jdan http headers <url> -H "Authorization: Bearer x"
jdan http headers <url> --json
```

## 输出

- **重定向跳**：status + `Location:`（默认只显 Location；`-a` 显全部头）
- **最终跳**：status + **全部响应头**（按 key 字母序，多值头每值一行，像 curl）
- 跳之间用 `→`

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `-X, --method` | GET | 请求方法（GET/HEAD/POST…） |
| `--max-redirects` | 10 | 最多跟几跳重定向（`0` = 不跟） |
| `-a, --all` | false | 每一跳都打全部响应头（默认重定向跳只显 Location） |
| `-H, --header` | — | 加请求头（可重复），如 `-H "User-Agent: x"` |
| `-k, --insecure` | false | 跳过 TLS 证书验证（沿用 `http timing` 的 `-k`） |
| `--json` | false | JSON 数组输出 |
| `--timeout` | 10s | 整体超时 |

## `--json`

```bash
$ jdan http headers http://github.com --json
[
  {"url":"http://github.com","status_code":301,"status":"301 Moved Permanently",
   "location":"https://github.com/","headers":{"Location":["https://github.com/"], ...}},
  {"url":"https://github.com/","status_code":200,"status":"200 OK","headers":{...}}
]
```

`headers` 是 `map[string][]string`（多值头是数组）。空（全失败）时输出合法空数组 `[]`。

## 边界 / 错误

- 无 scheme → 自动补 `https://`。
- **重定向循环 / 超过 `--max-redirects`**：到上限就停，把已走过的跳都列出来（不无限跟）。
- 连接失败 / DNS 失败 / TLS 错误 / 超时 → 清晰报错；**已成功的跳照常列出**（返回 partial hops + err）。
- 3xx 但没 `Location` → 当最终跳处理（不跟）。

## 内部架构 & 可测性

```
internal/httphdr/
  httphdr.go   Fetch(client, url, method, reqHeader, maxRedirects) ([]Hop, error)
                 —— 手动重定向循环；Hop{URL,Status,StatusCode,Header}
               FormatText(hops, showAll) / FormatJSON(hops) / EnsureScheme(url)
internal/cli/http_headers.go   注入 *http.Client，便于测试
```

**注入式可测**：`Fetch` 收 `*http.Client`，单测用 `httptest.Server` 搭真实重定向链（301→200、相对 Location、自循环）断言每跳；CLI 测试注入指向 httptest 的 client。**不打真实外网**。`Fetch` 用浅拷贝设 `CheckRedirect`，不改调用方 client。

## 测试

- `internal/httphdr`：EnsureScheme 补 https / 单 200（1 跳）/ 301→200（2 跳，相对 Location 解析）/ `--max-redirects 0` 不跟 / 自循环被 max 截断（不无限）/ `-X HEAD` 方法 / 请求头送达 / 多值响应头 / 连接错误返回 partial+err / 不改调用方 client；FormatText（重定向跳只显 Location、最终跳全部头、`-a` 每跳全部、`→` 箭头、头排序）/ FormatJSON（合法、location 字段、空数组）
- `internal/cli`：基本 / 重定向链 + 箭头 / `--max-redirects 0` / `--json` / `-H` 加请求头（httptest 回显）/ 非法 `-H` 报错 / 连接错误报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 连接/超时/非法参数 | 非 0（已成功的跳仍会先打印） |

## 有意不做

| 候选 | 原因 |
|------|------|
| 打印 body | 这是 headers 工具；要 body 用 curl / `http timing` |
| 请求体 / 复杂 auth 流 | `-H` 够覆盖常见；复杂的用 curl |
| HTTP/2 帧细节、cookie jar | 超范围 |

## TL;DR

1. `jdan http headers <url>` —— 状态行 + 响应头 + 完整重定向链
2. 手动跟转、逐跳展示，默认只读头不下载 body
3. 重定向跳显 Location、最终跳显全部头、`→` 连接；`-a` 每跳全部
4. `-X` 改方法、`-H` 加请求头、`--max-redirects 0` 不跟、`--json`
5. **0 新依赖**，纯 stdlib；跟 `http timing` 互补
