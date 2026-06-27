# jdan ascii-art

把图片渲染成 **ASCII 字符画**（像 jp2a）。复用已接好的 stdlib 图片解码,**0 新依赖**。是 `img`（只读尺寸/格式）的「画出来」补充。

## 它能干什么

```bash
$ jdan ascii-art logo.png            # 按终端宽度自动缩放
$ jdan ascii-art photo.jpg -w 60     # 指定列宽
$ jdan ascii-art logo.png --color    # 24-bit 真彩（仅 TTY）
$ jdan ascii-art logo.png --invert   # 反明暗（浅底终端）
$ cat x.png | jdan ascii-art         # stdin
```

## flags

| flag | 用途 |
|------|------|
| `-w, --width` | 输出列宽（默认按终端宽度，拿不到则 80；会被原图宽度上限钳制） |
| `--ramp` | 字符 ramp：`standard`（默认 10 级）/ `detailed`（70 级）/ `blocks`（`░▒▓█`）/ 自定义串 |
| `--invert` | 反明暗（浅底终端 / 打印） |
| `--color` | 每字符 24-bit 真彩（仅 TTY；管道自动剥离） |
| `--char-aspect` | 字符高/宽比（默认 0.5） |

## 算法

1. 解码图片 → `image.Image`（stdlib PNG/JPEG/GIF）。
2. 按列宽切网格,每格**块平均**采样（比最近邻清晰）。
3. 算每格 Rec.601 亮度 → 映射到字符 ramp 索引（`idx = lum × (n-1) / 255`,`--invert` 取反）。
4. 可选:每字符按源像素 `\x1b[38;2;r;g;bm` truecolor 染色。

**亮度方向**:默认「亮像素 → 密字符（`@`）」,贴合**深底终端**（亮字符=多点亮）。`--invert` 反过来,给浅底终端 / 打印到白纸。

**长宽比校正**:终端字符高 ≈ 宽的 2 倍,所以纵向采样乘 0.5,否则字符画纵向拉伸变胖。`--char-aspect` 可按字体微调。

## CJK 宽度注意

`--ramp blocks`（`░▒▓█`）更密,但这些是 **East-Asian-ambiguous** 字符,在中文 locale 终端会按 2 列宽渲染 → 字符画横向拉伸。命中时会打 stderr 警告。默认 `standard` ramp 全是 width-1 的 ASCII,任何终端不变形（这是先前 `disk`/`cal` 踩过的同一类宽度坑）。

## 格式

PNG / JPEG / GIF（GIF 取第一帧）—— stdlib 解码器（`imageinfo`/`img`/`qr` 已 blank-import,全局注册;本命令也显式 blank-import 一份）。**WebP/HEIC 不支持**:要 `golang.org/x/image` 等新依赖,违 0 依赖,遇到报清晰错。透明 PNG 的 alpha 是预乘的,透明区域趋近暗（注意）。

## 内部架构 & 可测性

```
internal/asciiart/asciiart.go
  Render(img image.Image, opts Options) string   —— 采样 + ramp 映射 + 可选染色，纯函数
  Options{ Width, Ramp, Invert, Color, CharAspect }
  luminance / avgColor / ResolveRamp
internal/cli/ascii_art.go                         —— image.Decode（复用解码器）+ 终端宽（复用 termWidth）
                                                     + TTY 染色（复用 isTTY）+ flag
```
`Render` 收已解码的 `image.Image` → 纯函数,用合成小图（`image.NewRGBA` 设已知像素）测出确定结果;CLI 层做解码 + 终端宽 + TTY 检测,可注入 stdin/out。

## 测试

- `internal/asciiart`:luminance（黑/白/红）;Render（全黑→全空格、全白→`@`、`--invert` 反向、`--color` 含/不含 ANSI、长宽比行数=40、列宽钳到原图宽）;ResolveRamp（关键字/自定义）
- `internal/cli`:stdin PNG 渲染、文件参数、非图片报错、文件不存在报错、**管道（非 TTY）即便 `--color` 也无 ANSI**

## 退出码

| 状况 | code |
|------|------|
| 成功 | 0 |
| 解码失败 / 不支持格式 / 文件打不开 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| WebP / HEIC | 要新依赖;PNG/JPEG/GIF 覆盖主场景 |
| GIF 动画逐帧播放 | 取第一帧;终端动画是另一个大功能 |
| braille（`⠿`）高清模式 / 边缘检测 | 第一版块平均够用,后续可加 |
| `--json` | 字符画是给人看的,json 无意义 |

## TL;DR

1. `jdan ascii-art logo.png` 按终端宽渲染;`-w` 定宽、`--color` 真彩、`--invert` 反明暗
2. 块平均采样 + 亮度→ramp;默认单色 ASCII（可粘贴/管道）,`--color` 真彩仅 TTY
3. 长宽比默认 0.5 校正,避免纵向拉伸;blocks ramp 在 CJK 终端会拉伸（警告）
4. 复用 stdlib PNG/JPEG/GIF 解码,**0 新依赖**;`Render` 纯函数全测
