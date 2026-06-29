# jdan htpasswd

生成 / 校验 **Apache·nginx Basic Auth** 的密码哈希行(`.htpasswd` 文件用)。**0 新依赖**。

## 原理

HTTP Basic Auth 的服务端读一个文件,每行 `用户名:密码哈希`:

```
alice:$2y$10$8GFZQm1xCx0NmDJssLCKeO08XSnx7.NbDHracQtoc4gtnlli9TISG
bob:$apr1$2vhWsRG.$WFJEZmwYhcHhTJCf0Y3pC1
```

请求带来明文密码 → 服务器用同样算法哈希 → 跟文件里那行比对。`htpasswd` 就是**生成这些哈希行**的工具。哈希**靠前缀区分方言**:

| 前缀 | 算法 | 评价 | 本实现 |
|------|------|------|--------|
| `$2y$` | bcrypt | 加盐慢哈希,**最安全** | `x/crypto/bcrypt`(已在依赖图) |
| `$apr1$` | Apache MD5-crypt(1000 轮 MD5) | 老但通吃 | 手写,对齐 openssl 金标准 |
| `{SHA}` | base64(sha1) | 无盐,**不安全**,仅兼容 | `crypto/sha1` |

### bcrypt 的 `$2y$` 坑

Go 的 `bcrypt` 生成 `$2a$` 前缀,而 Apache `htpasswd` 生成 `$2y$`。两者**算法完全相同**,只是版本标记不同(历史上 PHP 的一个修复),nginx/apache 都认。但 **Go 的 `bcrypt.CompareHashAndPassword` 不认 `$2y$`**。所以本实现:
- 生成时把 `$2a$` 改写成 `$2y$`(跟 `htpasswd` 输出一致);
- 校验时把 `$2y$`/`$2x$` 规整回 `$2a$` 再喂给 Go(同算法,验得过)。

## 用法

```bash
jdan htpasswd alice                       # 交互输密码（两次确认）→ 打印 alice:$2y$...
jdan htpasswd alice --apr1                 # 用 apr1
jdan htpasswd alice --sha                  # 用 {SHA}（不安全）
jdan htpasswd alice --cost 12              # bcrypt cost 调高
printf 'pass\n' | jdan htpasswd alice      # 非 TTY：从 stdin 读密码
jdan htpasswd alice -f .htpasswd           # upsert 进文件
jdan htpasswd --verify '$2y$10$...'        # 校验：输密码比对 hash
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--apr1` | false | 用 Apache apr1（MD5-crypt） |
| `--sha` | false | 用 `{SHA}`（无盐，不安全，仅兼容） |
| `--cost` | 10 | bcrypt cost（4–31） |
| `--file` `-f` | — | upsert 进 htpasswd 文件 |
| `--verify` | — | 校验模式：传已有 hash，再输密码比对 |

### 密码输入(安全红线)

**只走无回显交互提示或 stdin,绝不收 `-p` 明文参数**——跟 `jdan pwned` 一脉相承:
密码不该留进 shell history。TTY 下生成会要求**输两次确认**;非 TTY(管道/脚本)从 stdin 读一行。

### `-f` upsert 语义

写进文件时:**同名用户那行被替换**、**新用户追加**、**注释和其余行原样保留**。文件权限 `0600`。

### `--verify` 退出码

按前缀自动认 bcrypt/apr1/{SHA},**匹配退出 0、不匹配退出 1**——可进脚本 / CI。

## 实现

```
internal/htpasswdx/htpasswdx.go
  Bcrypt(pw, cost) / APR1(pw, salt) / SHA1(pw)   生成
  Verify(hash, pw)                                按前缀分派校验（constant-time 比对）
  Upsert(content, user, line)                     文件内容 upsert
internal/cli/htpasswd.go                           CLI：无回显输入 + 算法选择 + 文件/校验
```

- **纯函数好测,且钉死正确性**:apr1 / {SHA} 用 **openssl 生成的金标准向量**做断言(不是自洽往返),bcrypt 生成后 Verify 往返,Upsert 断言替换/追加/保留。
- 字符串比对用 `crypto/subtle.ConstantTimeCompare`(防时序侧信道)。
- apr1 salt 用 `crypto/rand` 取 8 字符(crypt(3) 字母表);测试里注入固定 salt 对金标准。

## 有意不做

| 不做 | 原因 |
|------|------|
| `-p` 明文密码参数 | 安全红线:会进 shell history（同 `jdan pwned`） |
| crypt（传统 DES） | 古董、8 字符上限、不安全、无 stdlib |
| 明文条目 | 不安全 |
| htdigest（`user:realm:MD5(user:realm:pw)`） | 另一种格式/工具,可后续单独做 |

跟 `jdan pwned`(查密码是否泄露)、`jdan secrets-scan`(扫硬编码密钥)同属安全凭据一类。
