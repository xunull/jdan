# jdan net cdn

判断一个网址前面挂没挂 **CDN/WAF**、挂的是哪家。给个网址,它告诉你结论 + 证据。**0 新依赖**(纯 stdlib)。

## 原理

CDN 把流量代理在自己边缘节点上,所以"它在不在某家 CDN 后面"有三路**互相独立**的判据,可靠性递增。任一命中即报,多路一致就定性「确定」。

### 信号 1 — HTTP 响应头指纹(最快、最直接)

边缘节点会盖一批专属头。其中有些是**铁证级**(★),只有那家发:

| Provider | 铁证头(★) | 辅助头 |
|----------|-----------|--------|
| Cloudflare | `CF-RAY` | `Server: cloudflare`、`CF-Cache-Status`、`cf-mitigated` |
| Amazon CloudFront | `x-amz-cf-id` | `x-amz-cf-pop`、`Via: …cloudfront` |
| Akamai | `x-akamai-request-id`、`akamai-grn` | `Server: AkamaiGHost` |
| Fastly | `x-fastly-request-id` | `x-served-by: cache-…`、`Via: …varnish` |
| 阿里云 CDN(淘宝/天猫) | `EagleId`、`X-Swift-Cachetime` | `Server: Tengine`、`X-Cache: HIT` |
| 百度 BFE | — | `Server: bfe` / `Server: BWS/1.1`(单路弱信号,标"很可能") |
| 腾讯云 CDN | `X-NWS-LOG-UUID` | `Server: NWS_…` |
| 京东 CDN | — | `Via: …(jcs …)`、NS `*.jdcache.com` |
| ChinaCache(蓝汛) | `Powered-By-ChinaCache` | — |

`CF-RAY` 形如 `8a1f2c3d4e5f6789-LAX`,**后缀是 Cloudflare 边缘机房的 IATA 机场码**(`LAX` = 洛杉矶),顺手解出来告诉你走的哪个 colo。

> Fastly 基于 Varnish,没有公开稳定的强指纹头(`x-served-by`/`via varnish` 在别的 Varnish CDN 上也会出现),所以按**启发式**判,诚实标「很可能」。

### 信号 2 — DNS NS 记录(基础设施层)

域名的 `NS` 记录指向 `*.ns.cloudflare.com`(像 `kim.ns.cloudflare.com`),说明这域**托管在 Cloudflare DNS** 上 —— 橙云代理就是这么挂的。`net.LookupNS` 拿。NS 记录在区顶,子域查不到,所以从全名往上逐级试,命中第一个有 NS 的层级(委派点),不依赖 PSL。

### 信号 3 — IP 段归属(网络层,最不会骗人)

把 host 解析成 A/AAAA,看 IP 落不落在 **Cloudflare 公布的 CIDR 段**里(`104.16.0.0/13`、`172.64.0.0/13`、`2606:4700::/32` 等,约 15 个 v4 + 7 个 v6)。IP 在段里 = 流量确实穿过 Cloudflare,**头被删了也藏不住**。`net/netip` 的 `Prefix.Contains` 一句话。段稳定,**内嵌**(0 依赖,同 lunar 嵌表路子),目前只有 Cloudflare 填满整段。

> **fake-ip 代理环境**:Clash/Surge 这类工具在 fake-ip 模式下,DNS 返回的是 `198.18.0.0/15`(RFC2544 保留段)的合成 IP,不是真实公网 IP —— 这一路在你机器上天然失效。jdan 会识别出这种合成/内网 IP 并在输出里点一句「IP 段判定已失效」,此时结论只基于响应头 + NS(这两路穿过代理仍是真实值)。

### 判定逻辑

强信号(`CF-RAY` / IP 落段 / 各家铁证头)单条命中 → **确定**;两路弱信号一致(如 `Server: cloudflare` + NS)→ **确定**;只有单路弱信号 → **很可能**;全不中 → **没看到已知 CDN**。

## 用法

```bash
jdan net cdn example.com              # 无 scheme 自动补 https://，跟重定向到最终 host
jdan net cdn https://x.com --json     # 机读
jdan net cdn x.com --headers-only     # 只看响应头，跳过 DNS/IP 解析（快、离线友好）
jdan net cdn x.com -k                 # 跳过 TLS 证书验证
```

输出示例:

```
✅ Cloudflare（确定）
   经 LAX 边缘
   最终 URL：https://www.cloudflare.com/

Cloudflare：
   · [header] cf-ray: a131…-LAX ★
   · [header] server: cloudflare
   · [ns] NS jule.ns.cloudflare.com
   · [ip] 104.16.124.96 ∈ 104.16.0.0/13 ★
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--headers-only` | false | 只看响应头,跳过 DNS NS / IP 段解析 |
| `--insecure` `-k` | false | 跳过 TLS 证书验证 |
| `--json` | false | JSON 输出(顶层带 `detected` 布尔) |
| `--max-redirects` | 10 | 最多跟几跳重定向(0 = 不跟) |
| `--timeout` | 10s | 单步超时 |

### 退出码

**文本模式**:检测到 CDN = `0`,没检测到 = 非 `0` —— 可进 CI / 脚本(同 `jdan htpasswd --verify` 的纪律)。
**`--json` 模式**:恒 `0`,脚本读 body 里的 `.detected` 字段判。
真正的网络/解析失败(连不上、NXDOMAIN)在两种模式下都返回错误。

## 实现

```
internal/cdnx/cdnx.go        Detect()/Render()/FormatJSON()/ColoFromCFRay()  纯判定，不联网
internal/cdnx/providers.go   DefaultProviders() + 内嵌 Cloudflare CIDR 段
internal/cli/net_cdn.go      CLI：HTTP 拉头（复用 httphdr.Fetch）+ DNS NS/IP 采集，喂给 Detect
```

- **纯函数好测**:`Detect` 吃「已采集的头/NS/IP」吐结果,核心逻辑脱网快测;CLI 只管网络采集 + 调 `Detect`,三个采集函数是注入点,测试灌桩数据不走真网。
- **嵌表纪律**:CIDR 段内嵌 + `TestCloudflareRanges_AllParse` 往返保证每条都能 `ParsePrefix`,落段命中用真实 IP 断言。
- **复用积木**:HTTP 拉头复用 `httphdr.Fetch`(手动跟重定向、逐跳带头、不下 body),scheme 补全复用 `httphdr.EnsureScheme`。

## 有意不做

| 不做 | 原因 |
|------|------|
| **回源 IP 反查 / 揭穿真实后端**(绕过 CDN 找源站) | 攻击性侦察手法,**安全红线** —— 只做"检测",绝不做"去匿名化"。同 `jdan pwned`/`htpasswd` 的安全姿态 |
| WAF 挑战绕过 / 指纹规避 | 同上,只识别不规避 |
| 联网拉取 / 自动更新 CIDR 段 | 内嵌即可,段很稳;联网反而引入失败面 |
| 完整 IATA 机场码 → 城市解码 | 表大价值低,显示原始 3 字母码即可,可后做 |
| 认遍所有 CDN | 收敛到主流四家(Cloudflare/CloudFront/Akamai/Fastly),引擎是可扩展表,后续加 provider 只是补一条 |

跟 `jdan net probe`(逐阶段探查)、`jdan http headers`(看响应头)、`jdan whois` 同属网络探查一类。
