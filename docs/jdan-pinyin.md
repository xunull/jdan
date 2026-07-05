# jdan pinyin

把中文转成**拼音**,多种声调样式,非汉字原样穿插保留。

这是 [`jdan t9`](jdan-t9.md) / [`jdan sp`](jdan-sp.md) / [`jdan spt9`](jdan-spt9.md) 三个命令**共同的第一步**单独成命令 —— 它们内部都先做「中文→拼音」,再往数字键/双拼码上映射;`jdan pinyin` 就是把这一步直接暴露出来。

## 原理

底层是 [go-pinyin](https://github.com/mozillazg/go-pinyin) 的 ~4 万条 Unihan 读音表(离线、纯查表)。汉字→拼音是纯数据(`中=zhong` 不可推导、只能查),所以这依赖是必需的(跟 t9/sp 共用同一份)。

go-pinyin 默认会**丢弃**没有读音的字符,所以 `jdan pinyin` 自己分词:连续的非汉字归成一段字面 token 原样保留,只把汉字送去查表。汉字读音也归到共享的 `internal/pinyinx`,`t9`/`sp`/`spt9` 的 `realPinyin` 都委托它。

```
jdan pinyin 中文  →  zhōng wén      （查表，不再往键盘映射）
```

## 用法

```bash
jdan pinyin 中文                   # zhōng wén
jdan pinyin 中文 --style plain     # zhong wen
jdan pinyin "Hello 世界 2024"      # Hello shì jiè 2024（非汉字穿插保留）
jdan pinyin 银行 --heteronym       # yín xíng/háng/…（多音字列全部读音）
jdan pinyin 中文 --sep -           # zhōng-wén
echo 你好世界 | jdan pinyin        # 管道
jdan pinyin 中A --json             # 逐字结构化
```

### 声调样式（`-s/--style`,默认 `tone`）

| 样式 | `中文` 输出 | 用途 |
|------|------|------|
| `tone` **默认** | `zhōng wén` | 带调符,真·拼音 |
| `num` | `zhong1 wen2` | 数字调,纯 ASCII |
| `plain` | `zhong wen` | 无调,文件名/变量名友好 |
| `initials` | `zh w` | 只声母(零声母字为空,如「文/王」) |
| `first` | `z w` | 只首字母(缩写) |

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `-s, --style` | `tone` | 声调样式(见上表) |
| `--heteronym` | false | 多音字列出全部读音(用 `/` 连) |
| `--sep` | 空格 | 拼音音节之间的分隔符 |
| `--json` | false | 逐字结构化(`tokens[]` + `result`) |

无参数时从 stdin 读。

## 细节

- **非汉字原样**:字母、数字、标点、空格都保留在原位。分隔符只加在**连续拼音音节之间**,非汉字段自带间隔——所以 `中文abc` → `zhōng wénabc`(忠于输入),`中文 abc` → `zhōng wén abc`。
- **多音字**:默认取最常见读音,逐字处理**不按词消歧**(`银行` 的「行」默认可能不是 háng)。要看全部读音用 `--heteronym`。这是与 t9/sp 一致的已知局限。
- **零声母**:`initials` 样式下,以 w/y/元音开头的字(文、王、爱)声母为空——这是 go-pinyin 的真实数据,不是 bug。

## 跟同源命令的关系

「中文→拼音」这一步在四个命令里是**共享**的(都走 `internal/pinyinx`):

| 命令 | `中国` 输出 | 干嘛的 |
|------|------|------|
| `jdan pinyin` | `zhong guo` | 拼音本身(罗马化) |
| `jdan t9` | `94664 486` | 全拼九宫格按键 |
| `jdan sp` | `vs go` | 26 键双拼字母 |
| `jdan spt9` | `87 46` | 双拼九宫格按键 |

## 有意不做

- **不做 slug**(小写+去标点+合并连字符)—— 那是单独命令;这里 `--sep -` 只是近似。
- 不做注音/威妥玛等其它罗马化系统(只汉语拼音)。
- 不做繁简转换(go-pinyin 对繁简都直接给读音)。
- 不做分词消歧多音字。

## 实现

```
internal/pinyinx/pinyinx.go   分词 + 样式 + 多音字 + 渲染（薄封装 go-pinyin）
internal/cli/pinyin.go        命令；t9/sp/spt9 的 realPinyin 已收敛到 pinyinx.Plain
```

跟 `jdan t9` / `jdan sp` / `jdan morse` / `jdan alpha` 同属「文字 ↔ 编码」一类。
