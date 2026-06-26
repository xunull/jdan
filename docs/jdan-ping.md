# jdan ping

ping 一个主机，但**可以用 `--dns` 指定解析域名用的 DNS server**：给了 `--dns` 就先用指定 DNS 把域名解析成 IP，再 ping 这个 IP；不给则退化成系统 ping 默认行为。0 新依赖。

## 为什么需要它

系统 `ping <域名>` 走的是**系统解析器**（`/etc/resolv.conf`、`nsswitch`），ping 本身没有 `--dns` 这类参数。要「用某个 DNS 解析再 ping」只能先 `dig @server` 拿 IP 再手动 ping。`jdan ping` 把这两步合一：

```bash
$ jdan ping --dns 8.8.8.8 example.com
example.com → 93.184.216.34 (via 8.8.8.8)     ← jdan 加的解析头
PING 93.184.216.34 (93.184.216.34): 56 data bytes   ← 系统 ping 原样输出
64 bytes from 93.184.216.34: icmp_seq=0 ttl=56 time=12.1 ms
...
```

排查「DNS 劫持 / 不同 DNS 解析到不同 IP」时特别有用——能直观看到指定 DNS 把域名解析到了哪个 IP，再 ping 它。

## 设计：为什么 shell out 系统 ping

实际的 ICMP 由**系统 ping 完成**（shell out，像 `jdan git` 调 `git`），jdan 只负责：
1. 用指定 DNS 把域名解析成 IP（复用 `internal/dnslookup`，含 DoH）
2. 构造 ping 的 argv
3. 尽力解析 ping 汇总行供 `--json`

排除的方案：自己实现 ICMP 要么引 `golang.org/x/net/icmp` 外部依赖，要么手搓 ICMP 包 + 原始 socket（要 root，且非特权 ICMP 在 Linux/macOS/Windows 行为各异）。shell out 给真 ICMP、免 root（系统 ping 是 setuid）、0 ICMP 代码。

> **关键正确性**：`--dns` 生效的前提是 **ping 解析出的 IP 而不是域名**。如果直接 `ping <域名>`，ping 会用系统 resolver 再解析一次，指定的 DNS 就被绕过了。所以有 `--dns` 时 jdan 一定 ping IP。

## 用法

```bash
jdan ping example.com                       # 普通 ping（系统解析）
jdan ping --dns 8.8.8.8 example.com         # 用 8.8.8.8 解析再 ping IP
jdan ping --dns 8.8.8.8:5353 example.com    # 自定义端口
jdan ping --dns https://dns.google/dns-query example.com   # DoH 解析
jdan ping --dns 8.8.8.8 -c 3 example.com    # 发 3 个包
jdan ping --dns 8.8.8.8 example.com -- -i 0.2 -s 64   # -- 后透传给系统 ping
jdan ping --dns 8.8.8.8 -c 3 example.com --json
```

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--dns` | 无 | 解析域名用的 DNS server：`8.8.8.8` / `8.8.8.8:5353` / DoH URL。不给则退化成系统 ping |
| `-c, --count` | 0 | 发送的包数（`-c`，Linux/macOS 通用；JSON 模式无此值时自动补 4） |
| `-4` | true | 解析 A / ping IPv4（默认） |
| `-6` | false | 解析 AAAA / ping IPv6 |
| `--json` | false | JSON 输出（解析事实 + 尽力解析的汇总） |
| `--dns-timeout` | 5s | DNS 解析超时 |
| `-- <args>` | — | `--` 之后的参数**原样透传**给系统 ping |

### 选项怎么传：`-c` 内置 + `--` 透传

只内置 `-c`（Linux 和 macOS 的 ping **都用 `-c N`**，唯一真正通用的 flag）。其余高级参数（`-i`/`-s`/`-W` 等 Linux/macOS 语义不同的）用 `--` 原样透传，**不翻译**——避免平台分歧地狱。

```bash
jdan ping --dns 8.8.8.8 example.com -- -i 0.2 -s 64 -t 5
```

## `--json` 输出

带**我们控制的解析事实** + **尽力解析的 ping 汇总行**：

```bash
$ jdan ping --dns 8.8.8.8 -c 3 example.com --json
{
  "host": "example.com",
  "resolved_ip": "93.184.216.34",
  "dns_server": "8.8.8.8",
  "ip_version": 4,
  "transmitted": 3,
  "received": 3,
  "loss_pct": 0,
  "rtt_min_ms": 11.8,
  "rtt_avg_ms": 12.0,
  "rtt_max_ms": 12.1,
  "exit_code": 0
}
```

汇总行（`X packets transmitted, Y received, Z% loss` + `rtt/round-trip min/avg/max`）在 iputils 和 BSD ping 上格式够稳。**解析不出就置 null，JSON 永远合法**——不会因为某个平台格式不认而崩。无 `--dns` 时不带 `dns_server` / `resolved_ip`。

> JSON 模式必须有限次数才能拿到汇总（否则 ping 默认一直发不退出），所以无 `-c` 时自动补 `-c 4`。

## 边界 / 平台

- **仅 macOS + Linux**（沿用 jdan 现状）。Windows 的 ping 输出/flag 差异太大，第一版不做。
- **IPv6**：Linux 用 `ping -6`，macOS 回落独立的 `ping6` 二进制。
- host 本身是 IP → 不解析，直接 ping，`--dns` 忽略。
- 环境没 `ping`（或 v6 没 `ping6`）→ 清晰报错。
- `-4` 和 `-6` 同时给 → 报错。
- **无 shell 注入**：`exec.Command` 传参数数组、不过 shell（host 用户可控也安全）。
- ping 非 0 退出（丢包/不可达）→ jdan 也非 0 退出；`--json` 里 `exit_code` 带真实退出码。

## 内部架构 & 可测性

```
internal/pingx/
  pingx.go   BuildCommand(target, opts, goos) —— 构造 ping argv（纯函数）
             Resolve(resolver, host, server, v6) —— 复用 dnslookup 解析成 IP
             ParseSummary(stdout) —— 解析汇总行（纯函数）
             Runner / ExecRunner —— 跑系统 ping，返回退出码（注入式）
internal/cli/ping.go   注入 Runner + Resolver，便于测试
```

跟 `jdan git` 一样**注入式**：单测注入假 Runner（断言 argv 构造对：ping IP 还是 host、`-c N`、`--` 透传、v4/v6 选 ping/ping6）+ 假 Resolver（喂 miekg/dns 构造的应答），**不真发包、不需 root/网络**。汇总解析是纯函数，喂 Linux + macOS 真实汇总样例断言。

## 测试

- `internal/pingx`：BuildCommand（基本/`-c`/v6 Linux 用 `-6`、v6 macOS 用 `ping6`/`--` 透传/target 永远在末尾）；Resolve（A/AAAA/跳过错类型记录/无记录报错/查询错误传播）；ParseSummary（Linux 格式/macOS 格式/丢包/垃圾输入不误判）
- `internal/cli`：默认无 `--dns` 不解析无头/有 `--dns` ping 解析出的 IP + 解析头/`-c`/`--` 透传/host 是 IP 不解析/`--json`（解析事实 + rtt + 自动补 count）/无 `--dns` 的 JSON 不带 dns 字段/v6 macOS 用 ping6/`-4 -6` 冲突报错/多 host 报错/非 0 退出码报错

## 退出码

| 状况 | exit code |
|------|-----------|
| ping 成功 | 0 |
| 解析失败 / 参数错 / ping 非 0 退出 | 非 0 |

## 有意不做

| 候选 | 原因 |
|------|------|
| 自己实现 ICMP | 要 x/net 依赖或手搓 + root/权限地狱 |
| 翻译 `-W/-i/-t` 等 flag | Linux/macOS 语义不同；用 `--` 透传 |
| Windows | ping 输出/flag 差异大，第一版不做 |
| 解析逐包 rtt 进 --json | 汇总行够用且更稳 |

## TL;DR

1. `jdan ping --dns <server> <host>` —— 用指定 DNS 解析域名再 ping 出来的 IP
2. 不给 `--dns` → 退化成系统 ping 默认行为
3. 底层 shell out 系统 ping（真 ICMP、免 root），仅 macOS + Linux
4. `-c` 内置、其余 `--` 透传；`--json` 给解析事实 + 尽力解析的 rtt
5. 复用 `dnslookup`（含 DoH），**0 新依赖**；排查 DNS 劫持神器
