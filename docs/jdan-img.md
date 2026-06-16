# jdan img

只读图片**文件头**报出尺寸/格式/颜色模型/大小，不解码整张图（用 `image.DecodeConfig`，快且省内存）。0 新依赖（纯 stdlib）。

全新「媒体」品类，跟现有命令零重叠。

## 它能干什么

```bash
$ jdan img logo.png
logo.png
  格式: PNG
  尺寸: 512 x 512
  颜色: NRGBA (含 alpha)
  大小: 24.3 KiB
```

看图片尺寸、批量核查资源、脚本里取宽高，不用打开图片编辑器。

## 支持格式

stdlib `image` 包只读 header 拿配置，注册了 stdlib 解码器：

| 格式 | 来源 |
|------|------|
| PNG | `image/png` |
| JPEG | `image/jpeg` |
| GIF | `image/gif` |

> WEBP / BMP / TIFF 需要外部依赖 `golang.org/x/image`，第一版不做——坚持 0 新依赖。后续要可加。

## 用法

```bash
jdan img <file>...        # 一个或多个文件
jdan img                  # 无参数 → 读 stdin
jdan img < logo.png       # stdin（重定向）
jdan img *.png --json     # JSON 数组
```

### 单文件 → 详细块

```bash
$ jdan img logo.png
logo.png
  格式: PNG
  尺寸: 512 x 512
  颜色: NRGBA (含 alpha)
  大小: 24.3 KiB
```

### 多文件 → 对齐表格

```bash
$ jdan img hero.jpg thumb.jpg
hero.jpg   1920x1080  JPEG  340.0 KiB
thumb.jpg  320x180    JPEG   18.0 KiB
```

### JSON（供脚本）

```bash
$ jdan img logo.png --json
[
  {
    "path": "logo.png",
    "format": "png",
    "width": 512,
    "height": 512,
    "color": "NRGBA",
    "bytes": 24890
  }
]
```

## 字段

| 字段 | 来源 |
|------|------|
| `format` | `DecodeConfig` 注册名（png / jpeg / gif） |
| `width` / `height` | `DecodeConfig().Width/Height` |
| `color` | 颜色模型名（NRGBA / RGBA / Gray / YCbCr / Paletted / CMYK…）；详细块对含 alpha 的标注「(含 alpha)」 |
| `bytes` | 文件 `Stat()` 大小；stdin 取读入字节数；文本里转人类可读（IEC：B/KiB/MiB） |

## flags

| flag | 默认 | 用途 |
|------|------|------|
| `--json` | false | JSON 数组输出 |

## 边界 / 错误处理

- **批量里某个文件坏 / 不支持**：打一行错误、**继续处理其余文件**，最后整体 exit 1（不让一个坏文件中断整批）。
- 文件不存在 / 非图片 / 截断 → 该文件报错，其余照常。
- stdin 模式 path 显示 `<stdin>`。
- `--json` 即使全部失败也输出合法空数组 `[]`。

## 内部架构

```
internal/imageinfo/
  imageinfo.go   Inspect(path, r, size) → Info（用 image.DecodeConfig 只读 header）
                 ColorModelName(color.Model) string
                 Info.HasAlpha()；HumanizeBytes(int64) string
                 import _ "image/png" / "image/jpeg" / "image/gif" 注册解码器
internal/cli/img.go
```

关键点：`Inspect` 用 `image.DecodeConfig`，**只读文件头**拿宽高/格式/颜色，不解整张图——对大图也是常数级开销。

## 测试

- `internal/imageinfo`：现场用 `image/png`、`image/jpeg`、`image/gif` 编码小图 → Inspect 报对 format/尺寸/颜色（PNG 含 alpha、JPEG 是 YCbCr 无 alpha、GIF 是 Paletted）；非图片 / 截断数据报错；ColorModelName 各模型映射；HasAlpha；HumanizeBytes 边界（B/KiB/MiB/GiB）
- `internal/cli`：单文件详细块含尺寸 / 多文件对齐表格 3 行 / `--json` 合法可 Unmarshal / stdin / 文件不存在报错且报路径 / 批量含坏文件（好文件仍处理 + 整体 exit 1）/ 全失败仍输出空数组

## 退出码

| 状况 | exit code |
|------|-----------|
| 全部成功 | 0 |
| 任一文件失败 / stdin 非图片 | 1 |

## 有意不做的事

| 候选 | 不做原因 |
|------|---------|
| WEBP / BMP / TIFF | 需 `x/image` 外部依赖；坚持 0 新依赖 |
| EXIF / 拍摄参数 | stdlib 不带 EXIF 解析；是另一个工具 |
| 解码整图 / 缩略图 / 格式转换 | 只做「读 header 报信息」，轻量 |
| 主色提取 | 需解整图 + 聚类，超范围 |

## TL;DR

1. `jdan img <file>...` —— 读 header 报尺寸/格式/颜色/大小
2. 支持 PNG / JPEG / GIF；只读文件头不解整图（快）
3. 单文件详细块、多文件对齐表格、`--json` 数组
4. 批量里坏文件不中断整批，最后整体 exit 1
5. **0 新依赖**，纯 stdlib `image` 包
