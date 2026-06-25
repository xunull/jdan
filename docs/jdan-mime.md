# jdan mime

按文件**内容**（magic bytes）判断真实 MIME / content-type，**不看扩展名**——文件改了名也认得出。0 新依赖（纯 stdlib）。

全新「文件类型」品类。跟 `jdan img`（专看图片尺寸）互补——`mime` 覆盖任意文件，只回答「这是什么类型」。

## 它能干什么

```bash
$ jdan mime logo.png
image/png

# 改名也认得出，并提示扩展名不符
$ mv photo.png weird.txt
$ jdan mime weird.txt
image/png   (扩展名 .txt 不符)
```

看下载文件的真实类型、批量核查、脚本里按类型分流，不被伪造/错误的扩展名骗到。

## 检测引擎

stdlib `net/http` 的 `DetectContentType`（实现 WHATWG MIME Sniffing 标准，读前 512 字节）作底座，覆盖 ~60 种：html / xml / pdf / png / jpeg / gif / webp / bmp / ico / mp3 / wav / ogg / mp4 / avi / webm / woff(2)/ttf/otf / **zip / gzip / rar** / wasm 等。

**在它之上加一层精选 magic 表**（stdlib 漏掉的常见开发格式，优先检查）：

| 格式 | magic | MIME |
|------|-------|------|
| ELF 可执行 | `7f 45 4c 46` | `application/x-elf` |
| 7-Zip | `37 7a bc af 27 1c` | `application/x-7z-compressed` |
| xz | `fd 37 7a 58 5a 00` | `application/x-xz` |
| zstd | `28 b5 2f fd` | `application/zstd` |
| bzip2 | `42 5a 68` | `application/x-bzip2` |
| tar | `ustar`（偏移 257）| `application/x-tar` |
| SQLite | `SQLite format 3\0` | `application/vnd.sqlite3` |

> `CA FE BA BE`（Java class / Mach-O 通用二进制）magic 有歧义，第一版不猜，交给 stdlib（更诚实）。

## 用法

```bash
jdan mime <file>...     # 一个或多个文件
jdan mime               # 无参数 → 读 stdin
jdan mime < file.bin    # stdin（重定向）
jdan mime *.bin --json  # JSON 数组
```

### 单文件 → 只打 MIME（干净、可管道）

```bash
$ jdan mime logo.png
image/png
```

### 多文件 → 对齐表格

```bash
$ jdan mime a.bin b.bin c.bin
a.bin   application/pdf
b.bin   application/zip
c.bin   text/plain; charset=utf-8
```

### JSON（供脚本）

```bash
$ jdan mime weird.txt --json
[
  {
    "path": "weird.txt",
    "mime": "image/png",
    "ext": ".txt",
    "ext_mismatch": true
  }
]
```

## 扩展名不符检测

实测类型与扩展名「应有」类型不一致时提示。判断用**内置的扩展名→类型表**（OS 无关、可复现），**故意不回退** stdlib `mime.TypeByExtension`（那依赖系统 `/etc/mime.types`，会引入非确定性）。

只收录**有 magic 的二进制格式 + `.txt`**：png/jpg/jpeg/gif/webp/bmp/pdf/zip/gz/tar/7z/xz/zst/bz2/mp3/mp4/wav。纯文本结构格式（json/csv/md 等会被探成 `text/plain`）不收录，避免误报。

**不报不符的情况**：无扩展名 / 扩展名不在表内 / 实测为 `application/octet-stream`（纯未知）。

## 字段（--json）

| 字段 | 说明 |
|------|------|
| `path` | 文件路径（stdin 为 `<stdin>`）|
| `mime` | 实测 content-type |
| `ext` | 文件扩展名（小写含点，如有）|
| `ext_mismatch` | bool：扩展名应有类型 ≠ 实测 |

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--json` | false | JSON 数组输出 |

## 边界 / 错误处理

- 只读前 **512 字节**（足够覆盖所有 magic，含 tar 的偏移 257）。
- **空文件** → `inode/x-empty`（对齐 `file` 语义），不 panic。
- 批量里坏/读不了的文件 → 打错误行、继续其余、**整体 exit 1**（沿用 `jdan img` 的容错模式）。
- stdin 模式 path 显示 `<stdin>`，无法判扩展名（不提示不符）。
- `--json` 即使全部失败也输出合法空数组 `[]`。

## 内部架构

```
internal/mimetype/
  mimetype.go   Detect(data) string：先查 extraSignatures（含偏移），
                再回落 http.DetectContentType；空数据 → inode/x-empty
                ExtMismatch(path, mime) (ext, mismatch)：只用内置 extExpected 表
internal/cli/mime.go
```

跟 `jdan img` 几乎同构（读文件 / 批量 / stdin / --json / 部分失败 exit 1），复用成熟模式。

## 测试

- `internal/mimetype`：构造 magic 字节 → PNG/PDF/gif/zip/gzip 走 stdlib 对；ELF/7z/xz/zstd/bzip2/SQLite 走 extra 表对；tar 偏移 257 命中；空 → `inode/x-empty`；偏移越界不 panic；extra 表优先级；`ExtMismatch` 各分支（.txt 实为 PNG → 不符 / .png 相符 / 无扩展名 / 未知扩展名 / octet-stream 不报 / 大写扩展名）
- `internal/cli`：单文件只打 mime / 多文件对齐表格 / 扩展名不符提示 / `--json` 可 Unmarshal / stdin / 空文件 / 文件不存在报错 / 批量含坏文件（其余 OK + exit 1）/ 全失败仍输出空数组

## 退出码

| 状况 | exit code |
|------|-----------|
| 全部成功 | 0 |
| 任一文件失败 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| 区分 zip 子类型（docx/xlsx/jar）| 需解 zip 内部结构，超范围 |
| libmagic 全量 magic 库 | 引外部依赖；精选表够覆盖常用 |
| `CA FE BA BE` 歧义猜测 | Java class / Mach-O 撞 magic，不猜更诚实 |
| 回退 stdlib mime.TypeByExtension | 依赖 OS mime.types，非确定性 |

## TL;DR

1. `jdan mime <file>...` —— 按 magic bytes 报真实类型，不看扩展名
2. stdlib `http.DetectContentType` + 精选 extra magic 表（ELF/7z/xz/zstd/tar/SQLite…）
3. 扩展名与实测不符时提示（`.txt` 实为 image/png）
4. 单文件只打 mime（可管道）/ 多文件表格 / stdin / `--json`
5. **0 新依赖**；跟 `jdan img`（看图片尺寸）互补
