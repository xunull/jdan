# jdan extract

通用归档解压命令。一个命令按文件扩展名自动识别 8 种格式，不用记 `tar xzvf` vs `unzip` vs `bzip2 -d` 哪个该用。

## 它解决什么问题

下载一个文件，解压时常见的窘境：

```bash
$ ls
release.tar.gz   data.zip   logs.tar.bz2   backup.gz

# 4 个 archive，3 套命令语法：
$ tar -xzvf release.tar.gz
$ unzip data.zip
$ tar -xjvf logs.tar.bz2     # 或 -xjf 还是 --bzip2？
$ gunzip backup.gz           # 或 bzip2 -d 还是 zcat？
```

每次都得查 flag。Linux / macOS 之间还略有差异。**`jdan extract` 用一个命令吃所有**：

```bash
$ jdan extract release.tar.gz
$ jdan extract data.zip
$ jdan extract logs.tar.bz2
$ jdan extract backup.gz
```

不用记语法 + 自动按扩展名识别。

## 自动按扩展名识别

`jdan extract` **完全自动按文件扩展名**选择 extractor。你不需要 `--format zip` 之类的 flag。

```bash
$ jdan extract data.zip          # → archive/zip
$ jdan extract data.tar.gz       # → compress/gzip + archive/tar
$ jdan extract data.tgz          # → 同上（.tgz 是 .tar.gz 简写）
$ jdan extract data.tar.bz2      # → compress/bzip2 + archive/tar
$ jdan extract single.gz         # → compress/gzip（单文件，去掉 .gz 后缀）
```

大小写不敏感：`FILE.ZIP` / `Data.Tar.GZ` 也认。

### 识别表

| 文件扩展名 | 解压器 | 输出 |
|-----------|--------|------|
| `.zip` | `archive/zip` | 多文件 → 子目录 |
| `.tar` | `archive/tar` | 多文件 → 子目录 |
| `.tar.gz` / `.tgz` | gzip + tar | 多文件 → 子目录 |
| `.tar.bz2` / `.tbz2` / `.tbz` | bzip2 + tar | 多文件 → 子目录 |
| `.gz` | gzip | 单文件（去掉 `.gz` 后缀的文件） |
| `.bz2` | bzip2 | 单文件（去掉 `.bz2` 后缀的文件） |
| 其他（`.rar` / `.7z` / `.xz` 等） | ✗ | 报错 |

### 不识别时的报错

```
$ jdan extract mystery.rar
Error: unknown archive format: mystery.rar
       (supported: .zip .tar .tar.gz .tgz .tar.bz2 .tbz2 .gz .bz2)
```

清晰告诉你支持哪些 + 你给的是啥。

### 只看扩展名，不看 magic bytes

注意：**只看文件名扩展名，不看文件内容的 magic bytes**。如果有人把一个 zip 文件改名叫 `mystery.tar.gz`，`jdan extract` 会按 tar.gz 试着解，然后报：

```
Error: extract: gzip: invalid header
```

**为什么这个 tradeoff**：

- magic bytes 检测要 read 文件头几个字节，慢一点
- 扩展名错的场景极少（archive 工具通常老老实实命名）
- macOS / Linux 自带 `file` 命令是 magic 检测，可以配合用：`file data.bin` 看真实格式后改名

简单清晰，如果你真撞到改名场景，自己 `mv` 一下即可。

## 用法

### 基本

```bash
jdan extract release.tar.gz
# → ./release/  (默认解压到当前目录的 <archive-name>/ 子目录)
```

子目录名是去掉**双后缀**的 base：`release.tar.gz` → `release/`、`data.tbz2` → `data/`。

### 指定输出目录

```bash
jdan extract data.zip -o /tmp/out
# → /tmp/out/  (jdan 会 mkdir -p)
```

### 解压到当前目录（不创建子目录）

```bash
jdan extract docs.zip --here
# → 文件直接在 cwd
```

当你已经在专门的目录里、不想再嵌一层时用。

### 只列内容不解压

```bash
$ jdan extract data.zip --list
archive: data.zip  (5 entries, 1.2MB total)

           1.2KB  README.md
           300KB  bin/foo
  d            -  bin/
           950KB  data.json
            12KB  config.yaml
```

类似 `tar -tzvf` / `unzip -l` 的功能。`d` 标 directory entry。

### JSON 输出

```bash
$ jdan extract data.zip --list --json
{
  "archive": "data.zip",
  "entries": [
    {"name": "README.md", "size": 1234, "is_dir": false, "mode": "-rw-r--r--"},
    ...
  ]
}
```

给脚本消费。

## flags 完整列表

| flag | 默认 | 作用 |
|------|------|------|
| `-o` / `--output` | `<archive-name>/` 子目录 | 解压目标目录 |
| `--here` | false | 解压到 cwd（不创建子目录）|
| `--list` | false | 只列内容，不实际解压 |
| `--json` | false | 结构化输出 |

`--here` 和 `-o` 互斥（`--here` 优先）。

## 安全

archive 解压是历史上最常见的 CVE 来源（zip slip）。jdan extract 加了 4 层防护：

### 1. 拒绝 directory traversal（zip slip）

```
$ # 一个恶意 zip，entry name 是 "../../etc/passwd"
$ jdan extract evil.zip
Error: extract ../../etc/passwd: entry "../../etc/passwd" contains '..' (directory traversal)
```

**选 reject 而不是 silent sanitize**：silent sanitize 会把 `../../etc/passwd` 改成 `etc/passwd` 写到 root 下，看起来安全但用户不知道这 archive 是恶意的。reject 让用户立刻看到问题。

### 2. 拒绝绝对路径 entry

```
$ jdan extract evil.zip
Error: extract /etc/passwd: entry "/etc/passwd" has absolute path
```

archive 里 entry 名应该是相对路径。绝对路径几乎都是恶意构造。

### 3. tar symlink 跳过

tar 格式支持 symlink entry。一个经典攻击：先放一个 symlink `link → /tmp`，再放一个 file entry `link/x.txt`，恶意 archive 能写文件到 `/tmp/x.txt`（symlink-then-write）。

`jdan extract` 直接**跳过 tar symlink entry**——不创建链接，不写文件。

### 4. 4 GiB 单 entry 上限（防 zip bomb）

每个 entry 最多读 4 GiB 后报错：

```
Error: entry exceeds 4294967296 bytes (refusing zip-bomb-shape)
```

zip bomb 攻击：小小一个 archive 解压出几 PB 数据填满磁盘。4 GiB 单 entry 上限封住了这条路。

## 与 `jdan hash` 配合（典型用法）

下载一个 release tarball 的标准 verify-then-extract 流程：

```bash
# 1. 拿 release + 官方 checksums
curl -LO https://example.com/release-v1.2.0.tar.gz
curl -LO https://example.com/release-v1.2.0.tar.gz.sha256

# 2. 校验
jdan hash --check release-v1.2.0.tar.gz.sha256
# → release-v1.2.0.tar.gz: OK

# 3. 校验过了再解压
jdan extract release-v1.2.0.tar.gz
# → ./release-v1.2.0/
```

`jdan hash --check` 失败 → exit 1 → `&&` 短路。整个流程 fail-safe。

## 内部架构

```
internal/extract/
  extract.go   DetectFormat / Extract / safeJoin (核心安全) /
               copyLimited (zip bomb 防护) / DefaultOutDir
```

主流程：

```
file path
   │
   ▼
DetectFormat(path)            ← 按 lowercase 后缀
   │
   ├──→ FormatZip       → extractZip       → archive/zip
   ├──→ FormatTar       → extractTar       → archive/tar
   ├──→ FormatTarGz     → compress/gzip → extractTar
   ├──→ FormatTarBz2    → compress/bzip2 → extractTar
   ├──→ FormatGz        → single file gzip
   └──→ FormatBz2       → single file bzip2

   每个 entry 写文件前都走：
     safeJoin(root, entryName)  ← 拒绝 `..` / 绝对路径 / 逃出 root
     copyLimited(dst, src)      ← 4 GiB 上限
```

## 退出码

| 状况 | exit code |
|------|-----------|
| 解压成功 | 0 |
| `--list` 列出成功 | 0 |
| 文件不存在 / 扩展名不支持 / 损坏 archive / 安全检查失败 | 1 |

可以用 shell `&&` 串接到下游命令：

```bash
jdan extract release.tar.gz && cd release && make install
```

## 有意不做的事

| 候选格式 | 不做原因 |
|---------|---------|
| `.7z` | 外部 lib（`github.com/saracen/go7z`）复杂；调用 7zz 二进制又引外部 dep |
| `.tar.xz` | Go stdlib 无 lzma；引 `github.com/ulikunitz/xz` 是新 dep，等用户真要再加 |
| `.rar` | 专利问题，开源实现有限 |
| 压缩（反向操作） | 用 `jdan zip` 做 zip；tar 等留给系统 `tar` |
| 加密 archive 解码 | scope 太大，需要 key 管理 |
| Magic bytes 检测 | 扩展名错的场景极少；用户可配合 `file` 命令 |

## 与其他 jdan 命令的关系

| 命令 | 干啥 |
|------|------|
| `jdan extract <archive>` | **本命令**：通用解压 |
| `jdan zip <files>` | 反向：把文件打成 zip（已有命令） |
| `jdan hash <file>` | 配合：解压前校验 archive 完整性 |

## TL;DR

1. `jdan extract <anything>` 按扩展名自动选 zip/tar/tar.gz/tar.bz2/gz/bz2
2. 默认解压到 `./<archive-name>/`，`--here` 解到 cwd，`-o` 自定义
3. `--list` 只看内容不解压；`--json` 给脚本消费
4. **4 层安全防护**：zip slip / 绝对路径 / tar symlink / zip bomb
5. **0 新依赖**，全 Go stdlib
6. 不支持 `.7z` / `.tar.xz` / `.rar`（外部 lib 或专利问题）
