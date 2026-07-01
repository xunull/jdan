# jdan git secrets

扫 git 仓库**历史**里是否提交过密钥/凭据(也能扫暂存区),检测交给 **gitleaks**。**0 新 Go 依赖**(运行时需要 `git` + `gitleaks`)。

跟 `jdan secrets-scan` 的分工:

| | `jdan secrets-scan` | `jdan git secrets` |
|---|---|---|
| 扫什么 | 工作区文件 / stdin | git 全历史 / 暂存区 |
| 引擎 | jdan 内置(正则 + 高熵) | gitleaks |
| 依赖 | 无(纯 stdlib) | 需装 gitleaks |
| 定位 | 随手扫当前文件 | 审「过去有没有提交过」 |

## 原理

你手头那种「扫 git 历史找泄露」的 shell(`gitleaks detect` + 敏感文件名审计 + 关键字 grep)产品化成一个子命令。检测本身**不重造轮子**——交给 gitleaks(150+ 规则、多年打磨的误报库)。jdan 只做三件事,都是 gitleaks 裸跑不给你的:

1. **默认脱敏**。gitleaks 默认会把明文 secret 打印到终端、写进报告文件躺在磁盘上。jdan 跑它时固定传 `--redact=100`,输出和 `--json` 都只有 `REDACTED`;要看明文得显式 `--show-secrets`。
2. **补一层文件名审计**。gitleaks 扫内容,抓不到「内容无特征」的凭据文件(加密 keystore、全低熵的 `.env`)。jdan 用 `git log --all --diff-filter=A --name-only` 找历史里**新增过**的敏感文件名(`.env`/`id_rsa`/`*.p12`/`.npmrc`/`serviceaccount` 等),单列一节。
3. **统一 UX + 退出码 + 友好报错**。没装 gitleaks 给安装指引而非一句晦涩报错;跨平台(那段 bash 在 Windows 跑不了,jdan 是 Go 二进制)。

调用等价于:

```
gitleaks git . --report-format json --report-path - --redact=100 --no-banner --exit-code 1
```

报告走 stdout(`--report-path -`),jdan 解析 JSON,合上文件名审计,渲染。

### 扫描范围：默认就是「所有分支的所有 commit」

不用加任何参数。gitleaks `git` 默认底层跑的是:

```
git log -p -U0 --full-history --all --diff-filter=tuxdb
```

那个 `--all` 覆盖**所有 ref**(本地分支 `refs/heads`、tag、远程跟踪分支 `refs/remotes`),`--full-history` 保证不漏合并进来的历史。文件名审计层同样用 `git log --all`,两层一致。所以一个只提交在某条侧分支(HEAD 不可达)上的密钥,默认也照样扫得到。

⚠️ 别用 `--log-opts=--all` 去「加全分支」——`--log-opts` 会**替换**掉上面那套聪明默认(变成裸的 `git log -p --all`,丢了 `--full-history` 和 diff-filter),是反效果。`--log-opts` 只在你要**缩小**范围时用,比如 `--log-opts=origin/main..HEAD` 只看某个 range。

唯一扫不到的:**你本地没 fetch 下来的远程分支**——那些 commit 根本不在本地对象库里,任何本地工具都无能为力,先 `git fetch --all` 再扫。

## 用法

```bash
jdan git secrets                               # 扫当前仓库全历史
jdan git secrets /path/to/repo                 # 扫指定仓库
jdan git secrets --staged                      # 只扫暂存区（pre-commit 用）
jdan git secrets --log-opts=origin/main..HEAD  # 限定范围
jdan git secrets --json                        # 机读（同样脱敏）
jdan git secrets --show-secrets                # 输出明文（默认脱敏）
jdan git secrets --baseline known.json         # 忽略已知项（gitleaks baseline）
```

输出示例(默认脱敏):

```
[history] config/app.go:12  aws-access-key  deadbeef  (Bob 2026-01-05)  REDACTED

疑似敏感文件（仅文件名，内容未验证）：
  · deploy/id_rsa   [SSH 私钥]

共 1 处内容命中 + 1 个可疑文件（已脱敏；exit 1）
提醒：如确为泄露，先轮换对应凭据；是否清理历史（git-filter-repo/BFG）由你决定，jdan 不代改。
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--staged` | false | 只扫暂存区(pre-commit),而非全历史 |
| `--show-secrets` | false | 输出明文密钥(默认脱敏) |
| `--no-filenames` | false | 跳过敏感文件名审计层 |
| `--json` | false | 结构化输出(默认同样脱敏) |
| `--log-opts` | — | **缩小**范围的 git log 选项(如 `origin/main..HEAD`);默认已扫全分支,此项会替换该默认 |
| `--baseline` | — | gitleaks baseline 文件(忽略已知项) |

### 退出码

`0` 干净 / `1` 有发现(CI 可卡门) / `2` 环境缺失(没装 gitleaks 或非 git 仓库)。跟你原来那段 shell 的约定一致。

### 当 pre-commit hook 用

不内置 hook 安装(避免覆盖 husky 等)。`.git/hooks/pre-commit`:

```sh
#!/bin/sh
exec jdan git secrets --staged
```

(`chmod +x .git/hooks/pre-commit`)。之后每次 `git commit` 前自动扫暂存区,有泄露就拦下。

## 实现

```
internal/gitleaksx/gitleaksx.go   ParseReport(json) + AuditFilenames(paths) + 渲染   纯函数
internal/cli/git_secrets.go       驱动 gitleaks + git，注入式 runner，退出码
```

- **纯函数好测**:`ParseReport` 解析 gitleaks JSON、`AuditFilenames` 匹配文件名,都不碰真进程;CLI 把 `gitleaksFunc` 和 `gitx.Runner` 灌假实现,测试不跑真 gitleaks/git。
- **脱敏在源头**:`--redact=100` 让 gitleaks 自己不吐明文,jdan 只忠实呈现,双保险。

## 有意不做

| 不做 | 原因 |
|------|------|
| 重造 gitleaks 的规则引擎 | 选它就是不重复造轮子;检测交给专业工具 |
| 替你改写历史(`git-filter-repo`/BFG) | 只检测 + 提示轮换,绝不动用户的 git 对象 |
| 联网验真(拿 key 去 HIBP/GitHub 校验) | 那等于把密钥泄出去;纯本地 |
| 包装成「有 gitleaks 就用、没有退回自研」 | 定位混乱;要零依赖的随手扫用 `jdan secrets-scan` |

深度取证审计(精确到作者/commit、CI baseline 联动)直接用 gitleaks / trufflehog;jdan 这层是「随手一审 + 默认脱敏 + 文件名补盲」。跟 `jdan git summary`、`jdan git changelog`、`jdan git commitlint` 同属 git 辅助一类。
