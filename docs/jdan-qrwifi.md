# jdan qrwifi

生成 **WiFi 入网二维码**：手机相机 / 微信扫一下直接连网，不用念密码、不用手输。
是 `jdan qr` 的「特定 payload」封装，渲染完全复用 `qr` 管线，**0 新依赖**。

## 为什么单独做（而不是 jdan qr 手搓）

WiFi 二维码的 payload 是个标准串：

```
WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;
```

你完全可以 `jdan qr "WIFI:T:WPA;S:foo;P:bar;;"` 手搓——**但只要 SSID 或密码里有
`\ ; , " :` 这几个字符，不转义就生成错的码，手机扫了静默连不上、还不报错**。
`qrwifi` 的核心价值就是**把这几个保留字符正确反斜杠转义**，加一个友好的 flag 接口。

## 用法

```bash
jdan qrwifi [ssid]
```

```bash
jdan qrwifi MyNetwork -p 's3cr3t'              # 终端二维码
jdan qrwifi --ssid "Cafe Guest" --auth nopass  # 开放网络，无密码
jdan qrwifi Home -p pw --hidden                # 隐藏网络
jdan qrwifi Home -p pw -o wifi.png             # 存 PNG（贴墙上）
jdan qrwifi Home --password-stdin <<< 'pw'     # 密码走 stdin，不进 shell history
jdan qrwifi Home -p pw --json                  # 结构化
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `[ssid]` / `--ssid` `-s` | — | 网络名（位置参数或 flag，二选一） |
| `--password` `-p` | — | 密码（`nopass` 时忽略） |
| `--password-stdin` | false | 从 stdin 读密码（避免进 shell history / `ps`） |
| `--auth` `-a` | `wpa` | 认证类型：`wpa` / `wep` / `nopass` |
| `--hidden` | false | 隐藏网络 → `H:true` |
| `--ecc` | `M` | 纠错级别 `L/M/Q/H`（继承 `qr`） |
| `--invert` | false | 反色（白底终端） |
| `--full-block` | false | 全角 `██`（兼容老终端） |
| `--output` | 终端 | 按扩展名写 `.png` / `.svg` |
| `--png-size` | 256 | PNG 像素尺寸 |
| `--svg-module` | 8 | SVG 每模块像素数 |
| `--json` | false | `{ssid, auth, hidden, payload, qr}` |

## 认证类型

| `--auth` | payload `T:` | 说明 |
|----------|-------------|------|
| `wpa`（默认，含 `wpa2`/`wpa3`） | `WPA` | de-facto 标准没有单独 WPA3 token，一档覆盖 |
| `wep` | `WEP` | 老旧网络 |
| `nopass`（或 `open`/`none`） | `nopass` | 开放网络，**省略 `P:` 字段** |

**WPA/WEP 空密码会直接报错**：忘了给密码就生成「空密码」的码，手机扫了会拿空密码去连、静默连不上。所以 `jdan qrwifi MyNet`（无 `-p`）报错提示二选一——给密码，或 `--auth nopass`（真开放网络）。空密码只在 `nopass` 下合法。

## 转义规则（正确性核心）

SSID 和密码里这几个字符必须反斜杠转义：

| 字符 | 转义后 |
|------|--------|
| `\` | `\\` |
| `;` | `\;` |
| `,` | `\,` |
| `"` | `\"` |
| `:` | `\:` |

例：SSID `Cafe;Guest`、密码 `p:a,s"s\x` →
`WIFI:T:WPA;S:Cafe\;Guest;P:p\:a\,s\"s\\x;;`

中文/Emoji 等非保留字符**不转义**，UTF-8 原样进 payload（二维码本身是 UTF-8 安全的）。

## 密码与安全

二维码本质就是给人扫的、**会暴露密码**——这是设计使然。所以这里的安全顾虑只在
「**密码别泄露到 shell history / `ps` 输出**」这一层：

- `-p <pw>` 方便，但会进 shell history。
- `--password-stdin` 从标准输入读一行（保留空格，只去行尾换行），不落 history。
- 终端只画二维码；**密码明文只在 `--json` 的 `payload` 字段里出现**（要拿去管道才显示）。

不做过度设计（不加密、不藏），因为对一个「贴墙上给人扫」的码而言没意义。

## 退出码

| 状况 | code |
|------|------|
| 成功 | 0 |
| SSID 为空 / **WPA·WEP 密码为空** / 认证类型非法 / 位置参数与 `--ssid` 同时给 / `-p` 与 `--password-stdin` 同时给 / 不支持的输出扩展名 | 非 0 |

## 实现

```
internal/wifiqr/wifiqr.go   Payload(Config) + escape() + ParseAuth() —— 纯函数，转义/认证/隐藏
internal/cli/qr_wifi.go     jdan qrwifi：拼 payload → emitQR 渲染（与 qr 共用）
internal/cli/qr.go          抽出 emitQR（output-dispatch）供 qr 与 qrwifi 共用
internal/qrcode/            既有二维码渲染（skip2/go-qrcode），qrwifi 全量复用
```

- `wifiqr.Payload` 是纯函数，转义是唯一正确性关键点，针对性测试逐字符钉死。
- `emitQR` 把「`--json` / `--output` 写文件 / 终端」三态分发从 `qr.go` 抽成共享函数，
  qrwifi 复用，零行为变化。

## 有意不做

| 不做 | 原因 |
|------|------|
| 企业级 802.1X / WPA2-EAP | 要 identity 等额外字段，payload 复杂、扫码端支持差 |
| 读系统已保存的 WiFi 密码 | 要抠 macOS 钥匙串 / Linux NetworkManager / Windows，越权且平台分裂 |
| `qr vcard` 等其它 payload | 同模式可后续单独加，本命令只管 WiFi |

跟 `jdan qr`（通用二维码）互补：`qr` 编码任意文本，`qrwifi` 专管 WiFi payload 的拼装与转义。
