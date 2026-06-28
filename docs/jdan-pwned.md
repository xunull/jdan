# jdan pwned

查一个密码是否出现在已知数据泄露中，基于 Have I Been Pwned 的 **Pwned Passwords** API。
最妙的地方：**查得到，却不泄露**——你的明文密码和完整哈希都不离开本机。

## 原理：k-匿名（k-anonymity）

```
1. 本地算 SHA1(password)，40 位十六进制
   "password" → 5BAA6 1E4C9B93F3F0682250B6CF8331B7EE68FD8
                └┬──┘ └──────────────┬──────────────────┘
              前5位 prefix         后35位 suffix

2. 只把【前 5 位】发给服务器：
   GET https://api.pwnedpasswords.com/range/5BAA6

3. 服务器返回所有「SHA1 以 5BAA6 开头」的哈希后缀 + 出现次数（约 500~1000 条）：
   1E4C9B93F3F0682250B6CF8331B7EE68FD8:52372427   ← 你要找的那条
   003D68EB55068C33ACE09247EE4C639306B:3
   ...

4. 在【本地】这堆返回里比对你的 suffix。命中 → 泄露 N 次；没命中 → 未收录。
```

**关键**：服务器只看到 `5BAA6`（20 bit），它对应几十万个可能的密码，服务器无法知道你查的是哪个。明文和完整哈希从不上网。

## 用法

```bash
jdan pwned                              # 无回显提示输入（不显示、不进 history）
echo -n 'password' | jdan pwned         # 从 stdin 读
cat passwords.txt | jdan pwned --batch  # 逐行批量审计
echo -n 'pw' | jdan pwned --json
```

输入只走两条路：**无回显交互提示**（真 TTY 时）或 **stdin**。
**故意不提供 `-p` flag**——一个查泄露的工具，不该反手把你的密码留进 shell history / `ps`。

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--batch` | false | 从 stdin 逐行查（每行一个密码） |
| `--json` | false | 结构化输出 |
| `--no-padding` | false | 关闭 `Add-Padding`（默认开） |

## 输出

```
$ echo -n 'password' | jdan pwned
⚠ 这个密码在已知泄露中出现过 52,372,427 次 —— 强烈建议别再用

$ echo -n 'a-long-unique-passphrase' | jdan pwned
✓ 没在 HIBP 数据集里出现过（注意：不等于绝对安全，只是没被收录）
```

批量：

```
$ cat pw.txt | jdan pwned --batch
⚠ password                 泄露 52,372,427 次
⚠ hunter2                  泄露 65,763 次
✓ a-long-unique-passphrase 干净

3 个里有 2 个已泄露
```

## 退出码（可脚本化 / CI gate）

| 状况 | code |
|------|------|
| 干净（未收录） | 0 |
| 泄露（批量：任一条泄露） | 1 |
| 网络 / 读取 / 空输入等出错 | 2 |

可放进 pre-commit / CI 卡门：泄露的密码进不了仓库。

## 隐私加固

- **只发 5 位前缀**：核心 k-匿名保证。有针对性测试钉死「请求 URL 里只出现 5 位前缀，完整哈希绝不出现」。
- **`Add-Padding: true`（默认开）**：HIBP 会把返回填充到定长，这样网络中间人连「响应大小」都推不出信息。`--no-padding` 可关。
  - 填充条目的 count 为 0，`Lookup` 把 count=0 当作未泄露，自然忽略。
- **不落 history**：无 `-p`，密码只从无回显提示或 stdin 进来。

## 实现

```
internal/pwned/pwned.go   纯函数：SHA1Hex / SplitRange(→5+35) / Lookup(body,suffix)→count
internal/cli/pwned.go     读密码（无回显/stdin）→ 调 API（注入 client+baseURL 可测）→ 输出
                          批量时按 prefix 缓存，少打 API
```

- 哈希、拆分、解析全是纯函数。`SHA1("password")` 是已知常量，断言 `prefix=5BAA6`、能在合成 range body 里查到 suffix。
- API 层注入 `httptest` 假服务器返回 canned body，**测试不联网**；并断言「只发 5 位前缀」「默认带 Add-Padding」。
- 需要网络才能跑（它就是查 HIBP）；无网 → exit 2。

## 有意不做

| 不做 | 原因 |
|------|------|
| 按邮箱查账号泄露 | HIBP 的账号 API 要**付费 key**，且会把你**真实邮箱**发出去（无 k-匿名保护），隐私倒退 |
| 本地离线泄露库 | HIBP 完整库几十 GB，不该塞进一个小工具 |
| NTLM 哈希变体 | 默认 SHA1 够用；NTLM 是 Windows 域场景，小众 |

跟 `jdan secrets-scan`（扫代码里硬编码的密钥）互补：一个查「代码里有没有泄露的 secret」，一个查「这个密码本身是不是早被泄露了」。
