# jdan git commitlint

按 **Conventional Commits** 规范校验提交信息。**0 新依赖**(纯 stdlib;range 模式调 git)。

## 原理

Conventional Commits 规定提交信息的 header 长这样:

```
<type>(<scope>)<!>: <subject>
        ↑可选    ↑可选破坏性标记
<空行>
<body>
<空行>
<footer：如 BREAKING CHANGE: …、Refs: #123>
```

校验就是把 header 按这个结构拆开,逐规则查:

| 规则 | 说明 |
|------|------|
| `header-structure` | header 必须是 `type(scope): subject` 结构(冒号后一个空格) |
| `type-empty` / `type-enum` / `type-case` | type 必填、在白名单内、小写 |
| `scope-case` / `scope-empty` | scope 小写;`--scope-required` 时必填 |
| `subject-empty` / `subject-full-stop` | subject 非空、结尾无句号 |
| `header-max-length` | header 不超长(默认 100,**按 rune 计**,中文不误伤) |
| `body-leading-blank` | header 与 body 之间要空一行 |

默认 type 白名单(对齐 `@commitlint/config-conventional`):
`feat fix docs style refactor perf test build ci chore revert`。

`!` 或 `BREAKING CHANGE:` footer 会被标记为破坏性变更(对应 major bump)。

纯字符串解析 + 正则,纯函数好测。range 模式靠 `git log` 取信息,复用 `gitx.Runner`(可注入,测试不碰真 git)。

## 用法

输入来源按优先级:`-m` > `-f` > revision-range > stdin > `HEAD`。

```bash
jdan git commitlint                          # 校验 HEAD（最后一条提交）
jdan git commitlint origin/main..HEAD        # 校验 PR 分支上的全部提交
jdan git commitlint -m "feat(api): 加分页"    # 校验一条字面量
git log -1 --format=%B | jdan git commitlint # 管道
jdan git commitlint --json                   # 机读
jdan git commitlint --warn HEAD~10..HEAD     # 软模式：只报不拦（退出 0）
```

输出示例:

```
✗ d4e5f6  Fixed the login bug.
    · [header-structure] header 不符合 "type(scope): subject" 结构："Fixed the login bug."

1/1 条提交不合规 ✗
```

### Flags

| flag | 默认 | 说明 |
|------|------|------|
| `--file` `-f` | — | 从文件读(commit-msg hook 用) |
| `--message` `-m` | — | 直接校验一条字面量 |
| `--types` | 内置白名单 | 覆盖允许的 type(逗号分隔) |
| `--max-header` | 100 | header 长度上限(按 rune) |
| `--scope-required` | false | 强制要有 scope |
| `--json` | false | JSON 输出(顶层带 `ok` 布尔) |
| `--warn` | false | 软模式:报违规但仍退出 0 |

### 退出码

全合规 `0`、有违规非 `0`(可直接当 commit-msg hook 拦下不合规提交,同 `htpasswd --verify`)。`--warn` 恒 `0`。`--json` 也遵循退出码,脚本另可读 body 里的 `.ok`。

### 当 commit-msg hook 用

不内置 hook 安装(避免覆盖 husky 等),手动挂一行即可。`.git/hooks/commit-msg`:

```sh
#!/bin/sh
exec jdan git commitlint -f "$1"
```

(`chmod +x .git/hooks/commit-msg`)。git 提交时把信息文件路径作为 `$1` 传入,不合规则退出非 0、提交被拦。

## 实现

```
internal/commitlint/commitlint.go   Parse(msg) + Lint(commit, opts)   纯校验，不碰 git
internal/cli/commitlint.go          CLI：取输入（-m/-f/range/stdin）+ 调 Lint + 渲染/退出码
```

- **纯函数好测**:`Parse` 拆结构、`Lint` 逐规则查,golden 消息串覆盖每条规则;CLI 的 range 模式把 `gitx.Runner` 灌假实现,测试不跑真 git。
- **剥 git 噪声**:`Parse` 先去掉 `#` 注释行与 verbose 模式 scissors(`# ---- >8 ----`)之后的 diff,所以 hook 模式喂 `.git/COMMIT_EDITMSG` 也干净。
- **中文友好**:header 长度按 `utf8.RuneCountInString` 算,不按字节,中文提交不会被误判超长。

## 有意不做

| 不做 | 原因 |
|------|------|
| 解析 commitlint 的 JS 配置(`.commitlintrc.js` / cosmiconfig) | 那是 node 生态,引入即破坏 0-dep;用 flags 覆盖足够 |
| 自动安装 git hook(`--install-hook`) | 避免覆盖 husky / 已有 hook;文档给一行脚本手动挂更安全 |
| 交互式提交构造器(commitizen / `git cz`) | 另一类工具,可后续单独做 |
| 改写 / 自动修复提交信息 | 只 lint 不 rewrite history,不动用户的提交 |
| 自定义规则插件系统、强制 gitmoji | 过度工程;内置规则 + 几个 flag 覆盖绝大多数 |

跟 `jdan git summary`、`jdan git changelog` 同属 git 辅助一类;`changelog` 也按 Conventional Commits 分组,两者规范一致。
