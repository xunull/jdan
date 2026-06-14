# jdan whois

WHOIS 查询命令（RFC 3912）。自动检测 domain vs IP，自动路由到正确的 server，
跟随 IANA / ARIN referral 到最终响应，默认输出解析后的字段表。

## 它解决什么问题

系统的 `whois` 命令存在分裂：

| 系统 | 状态 |
|------|------|
| macOS | `whois` 自带，但是 BSD 版本，TLD 映射表过时（很多新 gTLD 不识别） |
| Linux | 多数发行版要 `apt install whois`，Debian 版本较好但仍需查阅 server |
| Windows | 没有原生 `whois`，要装第三方 |

跨系统不一致 + 输出原始文本要靠人脑 grep。`jdan whois` 解决三件事：
1. **跨平台一致**：单二进制行为完全相同
2. **自动路由**：自动选 TLD/RIR 的 server，跟 referral，0 配置
3. **结构化输出**：parser 抽出 expiry/registrar/nameservers 等关键字段，CLI 表格直观

## 用法

### 默认：parsed 表

```bash
$ jdan whois example.com
Target:    example.com (domain)
Server:    whois.verisign-grs.com

  Domain:         EXAMPLE.COM
  Registrar:      RESERVED-Internet Assigned Numbers Authority
  Created:        1995-08-14 04:00 UTC  (31 years ago)
  Updated:        2026-01-16 18:26 UTC  (5 months ago)
  Expires:        2026-08-13 04:00 UTC  (in 2 months)
  Registry ID:    2336799_DOMAIN_COM-VRSN
  DNSSEC:         signedDelegation
  Status:         clientDeleteProhibited
                  clientTransferProhibited
                  clientUpdateProhibited
  Nameservers:    elliott.ns.cloudflare.com
                  hera.ns.cloudflare.com
```

时间用 `humanized` 后缀（"in 2 months" / "31 years ago"）—— 域名快过期一眼看出来。

### IP（自动跟随 referral）

```bash
$ jdan whois 193.0.0.1
Target:    193.0.0.1 (ipv4)
Server:    whois.ripe.net
Chain:     whois.arin.net -> whois.ripe.net

  Range:          193.0.0.0 - 193.0.7.255
  Net name:       RIPE-NCC
  Org:            Reseaux IP Europeens Network Coordination Centre (RIPE NCC)
  Country:        NL
  Abuse email:    abuse@ripe.net
  Registered:     2003-03-17  (23 years ago)
```

ARIN 看到 IP 不归自己管时返回 `ReferralServer: whois://whois.ripe.net`；
`jdan whois` 自动跟随，最多 3 跳。`Chain:` 行展示完整链路。

### `--raw`：只看原文

```bash
$ jdan whois example.com --raw
% Target: example.com (domain)
% Server: whois.verisign-grs.com

   Domain Name: EXAMPLE.COM
   Registry Domain ID: 2336799_DOMAIN_COM-VRSN
   ...
```

parser 失败（不识别的 schema 比如 `.br`）或者需要 debug 时用。

### `--full`：parsed + 原文

```bash
$ jdan whois example.com --full
[parsed 表]
...
--- Raw WHOIS response ---
[原始响应]
```

### `--json`：结构化输出

```bash
$ jdan whois example.com --json
{
  "target": "example.com",
  "kind": "domain",
  "server": "whois.verisign-grs.com",
  "raw": "...",
  "parsed": {
    "domain_name": "EXAMPLE.COM",
    "registrar": "RESERVED-Internet Assigned Numbers Authority",
    "creation_date": "1995-08-14T04:00:00Z",
    "expiry_date": "2026-08-13T04:00:00Z",
    "status": ["clientDeleteProhibited", ...],
    "nameservers": ["elliott.ns.cloudflare.com", ...]
  }
}
```

字段顺序按 RFC 6902（json 字段 key 字母序）—— 跟其他 jdan json 输出一致。
喂给 `jq` 或 `jdan json path` 直接消费：

```bash
$ jdan whois example.com --json | jdan json path "parsed.expiry_date" -r
2026-08-13T04:00:00Z
```

### `--server`：覆盖路由

```bash
$ jdan whois example.com --server whois.iana.org
```

绕过 TLD 映射 + IANA fallback，直接用指定 server。debug 路由 / 用未知 TLD 时用。

## 路由策略

WHOIS 没有 DNS 那样的统一根。每个 TLD/RIR 自己一个 server。`jdan whois` 用两套机制：

### 1. 内置 TLD → server 映射（~53 个常用 TLD）

| 类别 | 覆盖 |
|------|------|
| Legacy gTLD | com / net / org / info / biz / edu / gov / mil |
| 新 gTLD | io / ai / app / dev / xyz / co / me / tv / cc / tech / online / site / store / us |
| Asia | cn / jp / kr / tw / hk / sg / in / id / vn / th |
| Europe | uk / de / fr / nl / es / it / ru / ch / se / no / pl / be / at / eu |
| Americas | ca / br / mx / ar / cl |
| Oceania / Africa | au / nz / za |

代码在 `internal/whois/routing.go:tldServers`。

### 2. IANA root fallback

`tldServers` 没命中 → 走 `whois.iana.org`，从 response 里解析 `whois:` 行
得到真实 TLD WHOIS server，再发起一次查询。

```
jdan whois example.weirdtld
  ↓
whois.iana.org (查 "weirdtld")
  ↓ response 含 "whois: whois.real.example"
whois.real.example (查 "example.weirdtld")
  ↓ 真实响应
```

`Chain:` 行展示这条链。

### 3. IP referral（ARIN → RIR）

```
jdan whois 8.8.8.8           # 在 ARIN 管辖区
  → whois.arin.net 直接返回详情
  
jdan whois 193.0.0.1         # 在 RIPE 区
  → whois.arin.net
  ↓ response 含 "ReferralServer: whois://whois.ripe.net"
  → whois.ripe.net 返回真实详情
```

最多跟 3 跳（防恶意 server 制造环）。

## Parser 设计（schema 不一致正面回应）

不同 registrar 输出格式差异很大：

| 字段 | Verisign | RIPE | DENIC |
|------|----------|------|-------|
| 域名 | `Domain Name:` | n/a | `domain:` |
| 注册商 | `Registrar:` | n/a | n/a |
| 创建 | `Creation Date:` | `created:` | n/a |
| 过期 | `Registry Expiry Date:` | n/a | n/a |
| 状态 | `Domain Status: ... <URL>` | `status:` | `status:` |
| Nameservers | `Name Server:` | n/a | `nserver:` |

`jdan whois` 用**别名映射** + **大小写不敏感**的 line-prefix parser：

```go
{field: "expiry", keys: []string{
    "registry expiry date",
    "registrar registration expiration date",
    "expiration date",
    "expiry date",
    "expires on",
    "expires",
    "expire",
}},
```

任何一个别名命中都映射到同一逻辑字段。日期支持 6 种格式（`RFC3339` /
`2006-01-02 15:04:05` / `2006-01-02` / `02-Jan-2006` / `2006.01.02` 等），按
严格→宽松顺序尝试。Status 行尾 URL（Verisign 风格）自动剥除。

### Parser 兜底策略

如果 parser 一个字段都没抓到（schema 完全不识别）：
1. 默认模式 → **自动回退显示 raw**（不留空白屏幕）
2. `--json` 模式 → `parsed` 字段省略，`raw` 永远有

**raw 永远拿得到** 是 design principle —— parser 是优化层，不是必需层。

## 退出码

| 状况 | exit code |
|------|-----------|
| 查询成功（任何模式） | 0 |
| 网络错误（dial / read 失败 / 超时） | 1 |
| 目标格式错（既不是 domain 也不是 IP） | 1 |
| 过多 referral 跳数（>3） | 1 |

## 跟其他 jdan 命令的关系

```bash
# 域名快速体检：TLS cert + WHOIS 都看
jdan ssl cert example.com    # cert 过期时间
jdan whois example.com       # domain 过期时间

# 监控 pipeline：JSON → 提字段
jdan whois example.com --json | jdan json path "parsed.expiry_date" -r

# 批量查 .com 域名注册情况
for d in example.com test.com mysite.com; do
  jdan whois "$d" --json | jdan json path "parsed.registrar" -r
done
```

## 内部架构

```
internal/whois/
  whois.go      Query (TCP:43 协议) / Lookup / LookupWithServer / lookupChain
  routing.go    detectKind / extractTLD / tldServers map / RoutingFor /
                ParseIANAReferral / ParseReferral
  parser.go     scan / ParseDomain / ParseIP / parseWhoisTime / Parsed.IsEmpty
  types.go      Result / Hop / Kind
  testdata/     真实 WHOIS response fixtures (verisign / arin / ripe / denic)

internal/cli/whois.go
  默认 parsed 表渲染 / --raw / --full / --json
  humanizeAgo (in 2 months / 5 days ago)
```

## 测试

- routing: 16 unit tests（kind 检测 / TLD 提取 / 映射查找 / IANA & ReferralServer 解析）
- protocol: 6 mock-server tests（round-trip / 端口补齐 / timeout / referral 跟随）
- parser: 14 fixture-based tests（Verisign/ARIN/RIPE/DENIC schema + 日期格式 + scan）
- cli: 12 渲染 + 模式 tests（parsed 表 / raw / full / json / mutex flags / humanizeAgo）

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| 国际化 query 语法（`.de` 用 `-T dn,ace`） | 第一版 raw mode 足够，用户能拿到原文 |
| 历史 WHOIS（之前的注册人变更记录） | 商业 API（DomainTools 等），不适合 CLI |
| WHOIS 反查（按 owner 找域名） | 同上，商业 API |
| 隐私去识别（GDPR redacted 字段处理） | server 已经 redact 了，CLI 端无需重复 |
| RDAP（WHOIS 的 JSON 继任者） | 双协议支持是单独 scope；RDAP 部署不均匀，保留扩展 |

## TL;DR

1. `jdan whois <target>` —— 自动检测 domain/IP，默认输出 parsed 表
2. 53 个内置 TLD + IANA root fallback + IP referral 跟随
3. 4 模式：默认（parsed）/ `--raw` / `--full` / `--json`
4. parser 失败 **自动回退到 raw**（永远有内容显示）
5. `humanizeAgo` 让 "in 2 months" 这种 expiry 警告一眼可见
6. 0 新依赖，全 Go stdlib
