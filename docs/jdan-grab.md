# jdan grab

从任意文本 / 日志里把 **URL / email / IP** 捞出来。一个命令搞定三类(`-t` 选)。**0 依赖**。

> 命名：`jdan extract` 已是解压命令，所以文本提取用 `grab`(抓取)，避免混淆。

## 原理：松正则 + stdlib 校验

纯正则永远做不完美——URL(RFC 3986)、email(RFC 5322)、IPv6 的语法复杂到正则写全会变噩梦。
所以两步走：

```
第 1 步  用一个【松】正则，把"长得像"的候选都抓出来（宁可多抓）
第 2 步  用 stdlib 校验器把候选过一遍，留真去假，顺手归一化
```

| 类型 | 校验器 | 淘汰的假货举例 |
|------|--------|----------------|
| IP | `netip.ParseAddr` | `999.1.1.1` / `256.0.0.1`(越界)、`aa:bb:cc:dd:ee:ff`(MAC)、`12:34:56`(时间) |
| URL | `url.Parse`（要有 host） | 缺 host 的残串 |
| email | `mail.ParseAddress` | `a@@b`、缺 TLD 的 `x@y` |

**关键**：正则负责「找」、stdlib 负责「判真」。这三个校验器项目里都用过(`ip` 用 netip、`httphdr`
用 url.Parse)，比手写正则可靠得多。

## 用法

```bash
jdan grab [text|file]
```

```bash
cat access.log | jdan grab -t ip        # 只抽 IP（逐行，可管道给 sort/uniq）
jdan grab -t email < contacts.txt       # 只抽邮箱
pbpaste | jdan grab                      # 抽全部，带类型标签
jdan grab -t url --count log.txt        # 带出现次数（默认降序）
jdan grab page.html --json              # 按类型分组 JSON
jdan grab "contact me@here.io"          # 参数非文件 → 当字面文本
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--type` `-t` | `url,email,ip` | 抽取类型(csv)：`url` / `email` / `ip` |
| `--count` | false | 显示每个值的出现次数(默认按次数降序) |
| `--sort` | false | 结果按字典序排序 |
| `--json` | false | 按类型分组 JSON |

### 输入与输出

- **输入**：无参 → stdin；参数是已存在文件 → 读文件；否则当字面文本。
- **输出**：默认去重 + 保留首次出现顺序，逐行(grep 式可管道)。
  - **单类型** → 不带标签(`1.2.3.4`)，方便管道；
  - **多类型** → 带标签(`url: …` / `email: …` / `ip: …`)。
- IP 归一化为 `netip` 标准形(IPv6 压缩零段：`2001:0db8:…:0001` → `2001:db8::1`)。

## 实现

```
internal/grabx/grabx.go   URLs/Emails/IPs(text) []string —— 松正则候选 + stdlib 校验，纯函数
                          Dedup([]string)  去重保序
internal/cli/grab.go      jdan grab：读 stdin/file/arg → 按 -t 抽 → 去重/计数/JSON
```

- 抽取是**纯函数**，好测：喂一段「真假混杂 + 噪声」的文本，断言**只**抽出真的——
  `999.1.1.1`/`256.0.0.1`/MAC/时间被 netip 淘汰、`a@@b` 被 mail 淘汰。这正是「松正则会误抓、
  靠 stdlib 兜底」的验证点。
- CLI 注入 out/in 可测。

## 有意不做

| 不做 | 原因 |
|------|------|
| 电话号 | locale 格式太杂，误报率高 |
| 信用卡号 | 那是 `jdan secrets-scan` 的活（且有 Luhn 校验等） |
| 裸域名 / `www.` 开头 | 太噪、误报多；要 URL 就带 scheme |
| MAC 地址 | 可后续加 `-t mac`（同样松正则 + 校验套路） |

跟 `jdan secrets-scan`(扫密钥)、`jdan url`(单个 URL 编解码)互补：`grab` 专做「从一堆文本里
批量捞出结构化的东西」。
