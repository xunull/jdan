# jdan meta

抓网页的 `<meta>` / Open Graph / Twitter Card / canonical / favicon，回答「这链接分享到
微信 / Twitter / Slack 时长啥样」，顺手做分享 / SEO 体检。

## 用途

- **调试分享卡片**：链接发出去没缩略图 / 标题不对 / 描述空白，一眼看出缺了哪个 `og:` 标签。
- **看 SEO**：title、description、canonical、robots 对不对。
- 命令行直接拿一个页面的元信息，不开浏览器。

## 用法

```bash
jdan meta <url|file>
cat page.html | jdan meta      # stdin
```

三种输入：
- **URL**（`jdan meta example.com`）：抓网络。无 scheme 自动补 `https://`。
- **本地文件**（`jdan meta page.html`）：参数是已存在的文件时直接解析它。
- **stdin**（`cat page.html | jdan meta`）：无参数时读标准输入。

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--json` | false | 结构化输出（`{url, meta, issues}`） |
| `--ua` | 浏览器 UA | User-Agent；想模拟某平台爬虫（如 `facebookexternalhit`）可改 |
| `--timeout` | `10s` | 抓取超时 |

## 输出

```
$ jdan meta https://example.com/article
标题:        如何用 jdan 做一把瑞士军刀
描述:        一篇关于…
canonical: https://example.com/article
charset:   utf-8   lang: zh-CN

[Open Graph]
  og:image        https://example.com/cover.jpg
  og:title        如何用 jdan…
  og:type         article

[Twitter Card]
  twitter:card    summary_large_image

[favicon]
  /favicon.ico

体检:
  ⚠ 缺 og:description → 分享/SEO 无摘要
```

抽取：`<title>`、`<meta name=...>`（description / keywords / author / robots / viewport / charset）、
**Open Graph**（`og:*`）、**Twitter Card**（`twitter:*`）、`<link rel="canonical">`、`<link rel="icon">`、
`<html lang>`。重复的 `og:*`/`twitter:*` 取第一个。

### 体检项

| 缺什么 | 级别 | 后果 |
|--------|------|------|
| `<title>` 且 `og:title` | warn | 分享 / 搜索无标题 |
| `description` 且 `og:description` | warn | 分享 / SEO 无摘要 |
| `og:image` | warn | 分享卡片可能没缩略图 |
| `canonical` | info | 多 URL 收录可能分散权重 |

## 抓取约束（安全 / 健壮）

- **跟随重定向**，报最终 URL。
- **非 `text/html` 拒绝**（图片 / PDF → 只报 content-type，不硬解析）。
- **只读 `<head>` 区**：响应封顶 512 KiB，不下整个大页面 / 大文件。
- **默认超时 10s**（`--timeout`）。
- **默认浏览器 UA**：不少站对非浏览器 UA 返回阉割页；`--ua` 可改成模拟某平台爬虫或诚实标自己。

## 退出码

| 状况 | code |
|------|------|
| 成功（哪怕标签很少） | 0 |
| 抓取失败 / 非 HTML / 解析失败 | 1 |

## 实现

- 解析用 `golang.org/x/net/html`（**已在依赖图**，被 miekg/dns 间接拉入，0 新依赖）的正经
  tokenizer，畸形 HTML 也稳——不用脆正则。
- 解析逻辑 `metascan.ParseMeta(io.Reader)` 是**纯函数**（喂 HTML，不联网），可单测；抓取层
  （`internal/cli/meta.go`）薄，复用 `httphdr.EnsureScheme` 补 scheme。
- 包结构：
  ```
  internal/metascan/metascan.go   ParseMeta / Audit / Meta
  internal/cli/meta.go            抓 URL / 读文件 / stdin → ParseMeta → 文本/JSON
  ```

## 局限（有意不做）

| 不做 | 原因 |
|------|------|
| 跑 JS / 渲染 SPA | 只读静态 HTML；靠 JS 注入标签的 SPA 抓不到（**如实反映**，非 bug） |
| 下载 og:image 校验尺寸 | 只报 URL，不拉图（越权 + 慢） |
| oEmbed / JSON-LD 结构化数据 | 第一版聚焦 meta / OG / Twitter |
| 截图预览分享卡片 | 要渲染引擎，太重 |

跟 `jdan http headers`（看响应头 + 重定向链）互补：一个看 HTTP 层，一个看 HTML 头部语义。
