# jdan ip

IP 地址 & CIDR 计算工具集。7 个子命令覆盖**综合信息 / 网段包含判断 / IP 列表 /
区间→CIDR / 子网划分 / 网段聚合 / IPv6 标准化**。0 新依赖（纯 stdlib `net/netip`），
跟 `jdan whois` / `jdan dns` 配合形成完整网络套件。

## 它解决什么问题

SRE / 网管 / 后端开发的日常窘境：

```bash
# 算 192.168.1.0/24 有几个 host？要打开计算器或脑算
# 判断 10.5.1.2 是否在 10.0.0.0/8 网段？要心算二进制掩码
# 把 /22 切成 4 个 /24？要画 IP 表格
# IPv6 expand 全部 8 段？要数 colon
# 192.168.1.42 的反向 DNS 名是什么？要手动倒序
```

**`jdan ip` 让这一切变成一行命令**：

```bash
jdan ip info 192.168.1.0/24      # 一键全部 CIDR 信息
jdan ip contains 10.0.0.0/8 10.5.1.2 && echo "internal"
jdan ip split 10.0.0.0/22 24     # 4 个 /24
jdan ip normalize 2001:db8::1 --expand  # 8 段完整 IPv6
```

## 子命令一览

| 子命令 | 用途 |
|--------|------|
| `info <ip\|cidr>` | 综合信息（吃 IP 或 CIDR） |
| `contains <cidr> <ip>` | 判断 IP 是否在 CIDR 内（退出码） |
| `range <cidr>` | 列出 CIDR 内的 IP（默认前 16 个） |
| `range-cidr <start> <end>` | 任意起止区间 → 最小 CIDR 集（`range` 的反向） |
| `split <cidr> <new-bits>` | 子网划分 |
| `aggregate [cidr\|ip ...]` | 合并一组网段为最小 CIDR 覆盖集（`split` 的逆运算） |
| `normalize <ipv6>` | IPv6 标准化（compact / expand） |

## info

**最常用** 的命令。自动检测输入是 IP 还是 CIDR，分别给不同输出。

### 单 IPv4

```bash
$ jdan ip info 192.168.1.42
  Address:        192.168.1.42
  Version:        IPv4
  Hex:            0xC0A8012A
  Decimal:        3232235818
  Binary:         11000000.10101000.00000001.00101010
  Reverse DNS:    42.1.168.192.in-addr.arpa

  Classification:
  Private:        yes
  Global unicast: yes
```

### 单 IPv6

```bash
$ jdan ip info 2001:db8::1
  Address:        2001:db8::1
  Version:        IPv6
  Compact:        2001:db8::1
  Expanded:       2001:0db8:0000:0000:0000:0000:0000:0001
  Hex:            0x20010DB8000000000000000000000001
  Decimal:        42540766411282592856903984951653826561
  Binary:         0010000000000001:0000110110111000:0000000000000000:...
  Reverse DNS:    1.0.0.0...8.b.d.0.1.0.0.2.ip6.arpa  (32 nibbles + .ip6.arpa)

  Classification:
  Doc range:      yes
  Global unicast: yes
```

### IPv4 CIDR

```bash
$ jdan ip info 192.168.1.0/24
  CIDR:           192.168.1.0/24
  Version:        IPv4
  Network:        192.168.1.0
  Broadcast:      192.168.1.255
  First host:     192.168.1.1
  Last host:      192.168.1.254
  Netmask:        255.255.255.0
  Wildcard:       0.0.0.255
  Total IPs:      256
  Usable:         254
```

### IPv6 CIDR

```bash
$ jdan ip info 2001:db8::/64
  CIDR:           2001:db8::/64
  Version:        IPv6
  Network:        2001:db8::
  First host:     2001:db8::
  Last host:      2001:db8::ffff:ffff:ffff:ffff
  Prefix len:     64
  Total IPs:      18446744073709551616
```

`--json` 给脚本消费（数字以 string 形式避免 JSON precision 问题）：

```bash
$ jdan ip info 192.168.1.0/24 --json
{
  "prefix": "192.168.1.0/24",
  "version": 4,
  "network": "192.168.1.0",
  "broadcast": "192.168.1.255",
  "first_host": "192.168.1.1",
  "last_host": "192.168.1.254",
  "netmask": "255.255.255.0",
  "wildcard": "0.0.0.255",
  "prefix_len": 24,
  "total_addrs": "256",
  "usable_addrs": "254"
}
```

### Classification 字段

| 标签 | 含义 | RFC |
|------|------|-----|
| Private | RFC 1918 (10/8 / 172.16/12 / 192.168/16) + IPv6 ULA | RFC 1918 / RFC 4193 |
| Loopback | 127/8 / ::1 | - |
| Multicast | 224/4 / ff00::/8 | - |
| Link-local | 169.254/16 / fe80::/10 | - |
| Doc range | TEST-NET-1/2/3 / 2001:db8::/32 | RFC 5737 / RFC 3849 |
| Unique local | IPv6 fc00::/7 | RFC 4193 |
| CGNAT | IPv4 100.64.0.0/10 | RFC 6598 |
| Global unicast | 公网可路由 | - |

多个 label 可能同时为 true（例：multicast + link-local）。

## contains（退出码语义）

判断 IP 是否在 CIDR 内。**默认静默 + 退出码** 让你直接串到 `&&` / `||`：

```bash
# 用退出码做 CI gate / 部署 guardrail
jdan ip contains 10.0.0.0/8 "$IP" && deploy-internal "$IP"
jdan ip contains 192.168.1.0/24 "$IP" || echo "not in subnet"

# --verbose 给人看
$ jdan ip contains 10.0.0.0/8 10.5.1.2 --verbose
yes
$ jdan ip contains 10.0.0.0/8 192.168.1.1 --verbose
no
```

**Family mismatch 防错**：IPv4 CIDR + IPv6 地址会报错（不静默 fall through 给假阴性）：

```bash
$ jdan ip contains 10.0.0.0/8 2001:db8::1
Error: address family mismatch: 10.0.0.0/8 is IPv4, 2001:db8::1 is IPv6
```

## range

列出 CIDR 内的 IP。

```bash
$ jdan ip range 192.168.1.0/29
192.168.1.0
192.168.1.1
192.168.1.2
192.168.1.3
192.168.1.4
192.168.1.5
192.168.1.6
192.168.1.7
(8 total)
```

`--limit`（默认 16）：

```bash
$ jdan ip range 10.0.0.0/8 --limit 3
10.0.0.0
10.0.0.1
10.0.0.2
... (16777216 total, showing first 3; use --limit N or --limit 0 for all)
```

`--limit 0` 列全部，**硬上限 1M 防 OOM**（足够 /12 IPv4）。超过会报错让你显式 `--limit N`：

```bash
$ jdan ip range 10.0.0.0/8 --limit 0
Error: CIDR too large to enumerate (16777216 addresses, max 1048576); use --limit N
```

`--json` 输出 array：

```bash
$ jdan ip range 192.168.1.0/30 --json
{
  "cidr": "192.168.1.0/30",
  "returned": 4,
  "total": "4",
  "addrs": ["192.168.1.0", "192.168.1.1", "192.168.1.2", "192.168.1.3"]
}
```

## split

子网划分。

```bash
$ jdan ip split 10.0.0.0/22 24
10.0.0.0/24
10.0.1.0/24
10.0.2.0/24
10.0.3.0/24
(4 subnets)
```

`new-bits` 必须 >= 父 prefix 长度（不能"反向合并"），且 <= 32 (IPv4) / 128 (IPv6)。

**硬上限 65536 subnets** 防止用户失误（例：`/8 split /25` = 131072 个 → 报错）：

```bash
$ jdan ip split 10.0.0.0/8 25
Error: too many subnets (131072); refusing for sanity (max 65536)
```

IPv6 也支持：

```bash
$ jdan ip split 2001:db8::/62 64
2001:db8::/64
2001:db8:0:1::/64
2001:db8:0:2::/64
2001:db8:0:3::/64
(4 subnets)
```

## aggregate

`split` 的**逆运算**：把一堆 CIDR / IP 合并成最小的 CIDR 覆盖集——重叠或相邻的网段被并起来。防火墙规则合并、路由汇总常用。IPv4 与 IPv6 各自聚合（结果先 v4 后 v6）。

```bash
# 两个相邻 /25 → 一个 /24
$ jdan ip aggregate 10.0.0.0/25 10.0.0.128/25
10.0.0.0/24
(2 in → 1 out)

# 被包含的会被吸收，有洞的不合并
$ jdan ip aggregate 10.0.0.0/25 10.0.0.128/25 10.1.0.0/24
10.0.0.0/24
10.1.0.0/24
(3 in → 2 out)
```

裸 IP 当 `/32`（IPv6 当 `/128`）。参数留空时从 **stdin** 读（空白/换行分隔），方便管道：

```bash
$ cat routes.txt | jdan ip aggregate
$ jdan ip split 10.0.0.0/22 24 | grep / | jdan ip aggregate   # split↔aggregate 往返自洽 → 10.0.0.0/22
```

`--json` 输出 `{in, out, cidrs[]}`。

## range-cidr

`range` 的**反向**：把任意起止 IP 区间（**不必对齐边界**）分解成最小数量的 CIDR。iptables / ipset 里把一段区间转成规则时常用。

```bash
$ jdan ip range-cidr 192.168.1.5 192.168.1.20
192.168.1.5/32
192.168.1.6/31
192.168.1.8/29
192.168.1.16/30
192.168.1.20/32
(5 CIDRs)
```

也支持单参数 `start-end` 写法（IPv4/IPv6 地址本身都不含 `-`，切分安全）：

```bash
$ jdan ip range-cidr 192.168.1.5-192.168.1.20
$ jdan ip range-cidr 2001:db8:: 2001:db8::ff        # IPv6 → 2001:db8::/120
```

起止必须同族且 `start <= end`。`--json` 输出 `{start, end, count, cidrs[]}`。

## normalize

IPv6 标准化（IPv4 是 no-op，原样返回）。

```bash
# Compact（RFC 5952 标准压缩形式）
$ jdan ip normalize 2001:0db8:0000:0000:0000:0000:0000:0001
2001:db8::1

# Expand（完整 8 段）
$ jdan ip normalize 2001:db8::1 --expand
2001:0db8:0000:0000:0000:0000:0000:0001

# IPv4 直接返回
$ jdan ip normalize 192.168.1.1
192.168.1.1
```

`--expand` 在写配置文件（需要规整 8 段）或对接老系统时有用。

## 跟其他 jdan 命令配套

```bash
# WHOIS 拿到 NetRange，转 CIDR 再做计算
jdan whois 8.8.8.8 --json | jdan json path "parsed.netrange" -r

# DNS A 记录拿到 IP，判断是否内部
ip=$(jdan dns lookup myserver.com -t A | tail -1)
jdan ip contains 10.0.0.0/8 "$ip" && echo "internal"

# 把子网划分结果喂给 deployment 脚本
jdan ip split 10.0.0.0/22 24 --json | jdan json path "subnets" \
  | jq '.[]' | while read cidr; do
    deploy-subnet "$cidr"
  done
```

## 内部架构

```
internal/ipx/
  classify.go   Classification + Classify (RFC 1918/3849/4193/5737/6598)
  reverse.go    ReverseDNS (in-addr.arpa / ip6.arpa)
  cidr.go       CIDRInfo + ComputeCIDR + lastAddr/netmask/wildcard helpers
  info.go       AddrInfo + ComputeAddrInfo (hex/decimal/binary/分类)
  range.go      Range (limit > 0 / limit == 0 全列；硬上限 1M)
  split.go     Split (硬上限 65536 subnets)
  normalize.go  Normalize (IPv6 compact/expand)

internal/cli/ip.go    5 个 cobra 子命令
```

**0 新依赖**，全 `net/netip` + stdlib。

## 测试

- 34 unit tests on `internal/ipx`：
  - Classify 各 RFC 类别（private / loopback / multicast / link-local / CGNAT / doc / ULA）
  - ReverseDNS IPv4 + IPv6 nibble 数量边界
  - ComputeCIDR /0, /24, /31 (RFC 3021), /32 边界 + IPv6 /64
  - Range limit 边界 + 大 CIDR 防 OOM 报错
  - Split same prefix / newBits 太小 / 太多 subnets / IPv6
  - Normalize IPv6 compact/expand round-trip + IPv4 no-op
- 21 CLI tests：
  - info 三种输入（IPv4 / IPv4 CIDR / IPv6）+ JSON + invalid input
  - contains 真/假 + verbose + family mismatch
  - range 小 CIDR / 大 CIDR limit / JSON
  - split 基本 / invalid / JSON
  - normalize compact/expand/mutex flags/IPv4 no-op
  - parsePositiveInt 边界

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| `contains` IP 在 CIDR 内 | 0 |
| `contains` IP 不在 CIDR 内 | 1（CI gate 用） |
| 解析失败 / CIDR 太大 / split 输入无效 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| ASN 查询（IP → AS number） | 需外部数据库（IPtoASN / BGP / RouteViews），商业 API 化 |
| GeoIP（IP → 国家/城市） | 依赖 MaxMind 数据库，scope 太大 |
| ARP / MAC 查询 | 系统层、平台相关；jdan 偏纯函数工具 |
| 路由表查询 | 平台特定（`route` vs `ip route`），出 jdan scope |
| CIDR 合并（多个 CIDR → 最小覆盖集合） | 实现复杂，需求频率低 |
| Wildcard mask → CIDR | `0.0.0.255` 已经在 `info` 输出，反向工具用户可手算 |

## TL;DR

1. `jdan ip info <anything>` —— 单 IP / CIDR / IPv4 / IPv6 都吃
2. `jdan ip contains <cidr> <ip>` —— 退出码 0/1 做 CI gate
3. `jdan ip range <cidr>` —— 列 IP，大 CIDR 自动 truncate
4. `jdan ip split <cidr> <new-bits>` —— 子网划分
5. `jdan ip normalize <ipv6>` —— compact / `--expand` 切换
6. **0 新依赖**，纯 `net/netip`
7. 跟 `whois` / `dns` 拼成完整网络套件
