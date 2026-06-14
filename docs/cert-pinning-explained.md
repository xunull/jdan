# Cert pinning 是什么、为什么、怎么做

写给"我知道 HTTPS 是啥，但 cert pinning 第一次听说"的工程师。配合 `jdan ssl pin` 命令使用。

## 一句话

cert pinning = "我不光要 HTTPS，还要求这个 HTTPS server 必须是**我认识的**那个，不许是别的"。

是给**移动 app / 桌面客户端 / 自有 client** 用的 TLS 加强机制。让你的 app 不光信任系统的 CA 列表，**还要求 server 的证书必须等于你"钉死"在 app 里的那个**。

---

## 不做 pinning 的问题

### 普通 HTTPS 的信任链

```
你的 app  →  系统 CA 列表（200+ 个公网 CA）  →  任何一个 CA 签的 cert 都信
```

只要 server 出示一个**任意公网 CA 签的** cert，TLS 握手就过。

### 攻击场景

只要**任何一个 CA 被攻破** / **被胁迫签发了 your-app.com 的假 cert**，攻击者就能做 **中间人攻击 (MITM)**：

```
[你的 app] ───TLS───> [攻击者代理] ───TLS───> [真的 your-app.com]
                          ↑
                  攻击者出示假 cert
                  （被系统 CA 信任）
                  ↓
                  app 以为是真的 server
                  攻击者明文看到所有流量
                  能改请求内容
```

用户看不出问题：浏览器 / app 显示"安全"小绿锁，因为 cert 链验证过了。

### 真实历史案例

| 年份 | 事件 |
|------|------|
| 2011 | **DigiNotar** 被黑，攻击者签发了 google.com / gmail.com 等假 cert，监听伊朗用户 Gmail |
| 2014 | **印度 NIC** 签发了 google.com 假 cert |
| 2015 | **CNNIC** 中介机构 MCS Holdings 签发多个域名假 cert |
| 2016+ | 多家公司 / 国家通过**强加 root cert**做 MITM（中东 / 企业代理 / 校园网） |
| 长期 | 用户被钓鱼自己装恶意 root cert（"装这个证书才能访问 ××"） |

---

## pinning 怎么解决

```
你的 app  →  必须是这个 SPKI hash 的 cert  →  否则拒绝连接
```

攻击者就算搞到系统信任的假 cert 也没用，因为**假 cert 的公钥跟你 pin 的 hash 对不上**：

```
[你的 app] ───TLS───> [攻击者代理]
                          ↑
                  攻击者出示假 cert（公网 CA 签的，但 key 不一样）
                          ↓
              app: SPKI hash mismatch → 拒绝连接 → 报错
```

用户不会被无声中间人——要么连上**真**的 server，要么**根本连不上**。

---

## 关键概念：SPKI hash 不是 cert fingerprint

这是 cert pinning 最容易踩的坑。两个 hash 是不同的东西：

| 概念 | 公式 | 用途 | 哪个工具显示 |
|------|------|------|------|
| **Certificate fingerprint** | `SHA256(整个 cert)` | cert 内容完整性校验、cert 对比 | `jdan ssl cert` 的 SHA256 字段 |
| **SPKI hash** | `SHA256(cert 中的公钥部分)` | **cert pinning** | `jdan ssl pin` |

### 为什么 pinning 用 SPKI 而不是 cert 本身

cert 每 90 天 / 1 年要 **renew** 一次（Let's Encrypt 是 90 天）。每次 renew 出来是**新的 cert**，但通常 **公钥不变**（key 重用是常见做法）。

如果 pin 的是 **cert 本身的 hash**：

```
今天 cert renew  →  cert SHA256 变了  →  app 内置 pin 不匹配  →  全部断网
   ↓
用户必须升级 app 才能继续用  →  灾难
```

如果 pin 的是 **SPKI (公钥) hash**：

```
今天 cert renew，但 key 没换  →  SPKI hash 不变  →  pin 仍然 match
   ↓
app 正常工作，无感知
```

所以 HPKP / Chrome static pins / iOS NSAppTransportSecurity / Android Network Security Config **统一用 SPKI hash**。

### 例子

```
原 cert (2026-01 签发):
  Subject:       CN=api.your-app.com
  Public Key:    [Modulus: AB12CD34...EF]   ← 这部分被 SHA256 = SPKI hash
  Cert SHA256:   xxxxxxxxxxxxxx (整个 cert 的 hash，每次 renew 都变)
  SPKI SHA256:   ✦✦✦✦✦✦✦✦✦✦✦✦   ← pin 这个

renew 后的 cert (2026-04 签发):
  Subject:       CN=api.your-app.com
  Public Key:    [Modulus: AB12CD34...EF]   ← 同 key
  Cert SHA256:   yyyyyyyyyyyyyy (变了)
  SPKI SHA256:   ✦✦✦✦✦✦✦✦✦✦✦✦   ← 没变，pin 仍 match
```

---

## 谁会真的用 pinning

不是所有 app 都该 pin。pinning 通常用在**数据敏感 + 控制权强**的场景：

| 行业 | 用 pinning？ | 原因 |
|------|----------|------|
| Banking / Fintech app | 几乎都 pin | 支付安全 + 监管要求 |
| 加密通信（Signal / WhatsApp） | 必 pin | 端到端隐私 |
| 大公司 mobile app（Twitter / Uber / Instagram） | 大多 pin | 抗国家级 MITM |
| 内网企业 client 连内网服务 | 常 pin | 抗私有 root CA 注入 |
| IoT 设备连云端 | 常 pin | 设备 OEM 控制后端 |
| 普通 web app（浏览器访问） | ✗ | 用户已经能见到 cert 错误，且浏览器自己有 CT log |
| 个人博客 / 公开 API | ✗ | 不值这个运维成本 |

---

## 用 `jdan ssl pin` 的完整流程

### 流程图

```
1. dev 在 mac 上跑命令
   ↓
2. jdan ssl pin api.your-server.com --format okhttp
   ↓
3. 输出 OkHttp 配置代码（含 SPKI hash）
   ↓
4. dev 把代码贴进 Android app 的 OkHttpClient.Builder()
   ↓
5. app 编译发布
   ↓
6. 用户 app 启动后，每次 HTTPS 请求都验证 SPKI hash
   ↓
7. 任何中间人攻击（假 cert / 被劫持 CA / 自签 proxy）→ 用户连不上 → app 报错
```

### 具体步骤

**Android (OkHttp)**：

```bash
jdan ssl pin api.your-server.com --format okhttp
```

输出贴进 `OkHttpClient.Builder().certificatePinner(...)`：

```kotlin
val pinner = CertificatePinner.Builder()
  .add("api.your-server.com", "sha256/AbCd...=")  // leaf SPKI
  .add("api.your-server.com", "sha256/XyZ1...=")  // intermediate SPKI
  .build()

val client = OkHttpClient.Builder()
  .certificatePinner(pinner)
  .build()
```

**iOS (NSAppTransportSecurity)**：

```bash
jdan ssl pin api.your-server.com --format ios
```

输出贴进 `Info.plist` 的 `NSAppTransportSecurity` dict。

**调试 / 命令行验证 (curl)**：

```bash
jdan ssl pin api.your-server.com --format curl
```

输出一个可执行的 curl 命令，直接跑——pin 对了返回 200，pin 错了报：

```
curl: (90) SSL: public key does not match pinned public key!
```

这是验证 pin 配置正确的最快方法（不用真的把 app 装到手机上跑）。

### 默认 pin 哪些 cert

`jdan ssl pin` 默认 pin **leaf + 第一个 intermediate** 两个 hash。这是 Apple / Android / Chromium static pins 的 best practice：

| pin 谁 | 取舍 |
|-------|------|
| **leaf** | 精准。但每次 cert renew + 换 key 就要更新 pin |
| **intermediate** | 容忍 leaf renew（同 CA 重签即可）。万一 CA 倒闭就坏 |
| **root** | 容忍 leaf + intermediate 换。但 root 一改 pin 就完全失效；且攻击者只要拿到同一 CA 任何 cert 都能绕过 |

**推荐：leaf + 第一个 intermediate** 双 pin。

---

## 失败模式：pin 错了会怎样

### 软失败（设计选择）

OkHttp / iOS 默认是 **硬失败**：pin 不 match 直接 reject 连接。用户看到的是"连不上服务"。

如果想 **软失败 + 上报**：自己 wrap TLS handshake error，发到监控系统（Sentry / 自家 backend），先收集数据再决定是否硬阻断。但**最终一定要硬阻断**，否则 pinning 形同虚设。

### "我把 pin 写错了，app 全国断网"

发生过。预防：

1. **提前 1 个 release 加备用 pin**（多 pin 几个 cert 的 SPKI），等老 app 全部升级再换 cert
2. **远程下发 pin 列表**（少数大公司做法，本身要保护 pin 下发通道）
3. **Soft TTL**：app 内置的 pin 带个 expiry，过期后退化到不 pin（"break-glass"）

普通项目：**双 pin（leaf + intermediate）+ 别让所有 cert 同一天到期**，绝大多数场景够用。

### "我的企业代理 / 自家 CA 监控被你 pinning 拦了"

这是 pinning 的**有意行为**——拒绝企业代理意味着拒绝合法的 MITM 监控。如果你的 app 需要被允许走代理：

- 提供"调试模式"按钮关闭 pinning（仅 dev build）
- 在 app 设置里允许用户加自定义 cert（高敏感 app 不该提供）

### "我用 Charles / Fiddler / mitmproxy 调试，被 pinning 拦了"

也是有意行为。调试解决方案：

- 用 dev build（无 pinning 或带 toggle）
- Android: Network Security Config 加 `<debug-overrides>` 允许调试时跳过
- iOS: 用 dev cert + Xcode 关 pinning 的 `OBJ_DEBUG` flag
- 用 frida / objection 等工具运行时禁用 pinning（黑盒调试）

---

## pinning 不是"必须"做

pinning 是**安全 vs 灵活性**的取舍：

**好处**
- 抗 CA 被攻破（DigiNotar 类事件）
- 抗强制注入 root（国家 / 企业 MITM）
- 抗用户被钓鱼装恶意 root

**坏处**
- pin 真换了的时候，老 app 全部断网 → 需要 force-upgrade
- 调试摩擦增加（Charles / mitmproxy 失效）
- 用户的合法企业代理 / 监控被拦
- 运维成本：cert renew 时需要协调 mobile release

适合做 pinning 的场景：

- ✅ 数据敏感（钱、隐私、健康）
- ✅ 你能控制 cert 生命周期（自家 CA / 长 lifetime cert）
- ✅ 你能做 app force-upgrade 当 pin 要换时
- ✅ 用户不需要走企业代理

不适合做 pinning 的场景：

- ❌ 公开 API / 普通 web app（浏览器够用）
- ❌ 用户群在严格企业网（代理强制）
- ❌ 你不能控制 cert 何时换（用了不可控的 CA / 第三方托管）

---

## 与 `jdan` 其他 ssl 命令的关系

| 命令 | 干啥 | 输出的 hash | 用途 |
|------|------|------|------|
| `jdan ssl cert` | 看 cert 详情 | cert fingerprint (SHA256/SHA1) | 看 cert 的 SAN / 过期 / 完整 chain / 验证状态 |
| `jdan ssl scan` | 审计 TLS 配置 | 不输出 hash | TLS 版本 / cipher / HSTS 的 ssllabs 风格评分 |
| `jdan ssl pin` | 生成 pinning 配置 | **SPKI hash** | 给 mobile app 的 CertificatePinner 用 |

⚠ **不要拿 `jdan ssl cert` 的 SHA256 来做 pin**——那是 cert fingerprint，cert renew 后变了 pin 就坏。**必须用 `jdan ssl pin`**，它算的是 SPKI hash（key 不变就 stable）。

---

## TL;DR

1. cert pinning = app 强制 server 用"特定的 cert"，抗 CA 被攻破 / 抗 MITM
2. **必须用 SPKI hash**，不是 cert fingerprint，否则 cert renew 就坏
3. `jdan ssl pin` 一行命令算出所有主流格式（OkHttp / iOS / HPKP / NSS / curl / raw），直接复制到客户端配置
4. 默认 pin leaf + 第一个 intermediate（Apple / Android best practice）
5. 不是所有 app 都该做——是**安全和灵活性**的取舍

适合做 pinning 的人，jdan 是配置工具；不熟悉 pinning 的人，先读这篇文档再决定。
