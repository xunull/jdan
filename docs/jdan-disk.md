# jdan disk

像 `df`：列各挂载点的容量/已用/可用/使用率，带使用率条和高占用染色。给路径则只看该路径所在的文件系统。**0 新依赖**（纯 `syscall` + `os`）。仅 darwin / linux。

## 它能干什么

```bash
$ jdan disk
文件系统         容量   已用   可用  使用率          挂载点
/dev/disk3s1s1  1.8Ti  1.6Ti  269Gi  86% ████████░   /
/dev/disk9s2    1.8Ti  1.7Ti   95Gi  95% █████████   /Volumes/m1max-tm
```

## flags

| 形式 | 含义 |
|------|------|
| `jdan disk` | 列所有真实挂载点 |
| `jdan disk <path>` | 只看该路径所在的文件系统 |
| `-a, --all` | 显示伪文件系统（devfs/tmpfs/map…）和 0 容量项 |
| `-i, --inodes` | 显示 inode 用量而非字节 |
| `--bytes` | 原始字节而非人类可读 |
| `--no-color` | 关闭高占用染色 |
| `--json` | 结构化输出 |

## 使用率算法（对齐 df）

```
已用 = (总块 - 空闲块) × 块大小
可用 = 非 root 可用块 × 块大小          // bavail，< 空闲块（有 root 保留）
使用率 = 已用 / (已用 + 可用)，向上取整   // 跟 GNU df 一致
```

因为有 root 保留块，`已用 + 可用 < 总容量`，所以使用率不是 `已用/总量`。这点跟 `df` 一致，已写测试钉死（100 块、空闲 20、可用 10 → 89%）。

## 染色（仅 TTY）

- 使用率 ≥90% → 红，≥75% → 黄，其它不染。
- 管道/重定向（非 TTY）→ 纯文本，**不插任何 ANSI**，保持可解析。
- `--no-color` 强制关。

## 平台与 0 依赖

| 平台 | 采集方式 |
|------|----------|
| darwin | `syscall.Getfsstat` 一次拿全部挂载（含设备名/挂载点/类型/块数） |
| linux | 解析 `/proc/self/mounts` 枚举 + 对每个挂载点 `syscall.Statfs` |
| 其它（Windows…） | build-tag stub，返回「暂不支持」 |

全程 stdlib，连 `golang.org/x/sys` 都不用。darwin 的 `Statfs_t` 字符数组是 `[N]int8`，转 string 时截到 NUL；linux 的 `/proc/mounts` 把空格转义成 `\040` 等八进制，会还原。

## 内部架构 & 可测性

```
internal/diskx/diskx.go         —— Mount + Total/Used/UsePercent/InodePercent /
                                   HumanBytes / Filter / Render / bar / colorize，纯函数全测
internal/diskx/diskx_darwin.go  —— Mounts()/StatPath() 用 syscall.Getfsstat（build: darwin）
internal/diskx/diskx_linux.go   —— 解析 /proc/self/mounts + syscall.Statfs（build: linux）
internal/diskx/diskx_other.go   —— 不支持平台的 stub（!darwin && !linux）
internal/cli/disk.go            —— flag 分发；mounts/statPath 回调可注入便于测试
```

syscall 层不好单测 → 平台文件只负责采集 `[]Mount`，**过滤/算百分比/格式化/渲染/染色全是纯函数，用注入的 fake mounts 全测**；采集层加宽松 smoke（真机至少 1 个非零挂载、根总量>0，不支持则 skip）。CLI 把 `Mounts`/`StatPath` 做成可注入回调，测试塞假数据。

## 测试

- `internal/diskx`：HumanBytes（边界/Ti）、UsePercent（对齐 df + root 保留 + 不除零）、InodePercent、Filter（隐藏伪 FS/0 容量，`-a` 全留）、bar（0/50/100%）、colorize（红/黄/无/无 ANSI）、Render（表头/行/inode/bytes/无 ANSI）、visWidth 忽略 ANSI；采集 smoke
- `internal/cli`：默认列表 + 隐藏 devfs / `-a` / 单路径走 statPath / 路径错 / `--json` / 管道无 ANSI / 采集失败报错

## 退出码

| 状况 | exit code |
|------|-----------|
| 成功 | 0 |
| 路径不存在 / statfs 失败 / 平台不支持 | 1 |

## 有意不做

| 候选 | 原因 |
|------|------|
| 磁盘 I/O 速率 / SMART 健康 | 要持续采样/特权；`disk` 只管容量 |
| 目录占用排行（`du`） | 那是另一个命令，别混 |
| Windows | v1 stub；darwin/linux 覆盖主场景 |

## TL;DR

1. `jdan disk` 列挂载点；`disk /path` 看单个；`-i` inode、`--bytes`、`--json`
2. 使用率对齐 `df`（含 root 保留，不是简单 已用/总量）
3. 高占用 TTY 染色（≥90 红 / ≥75 黄）；管道纯文本可解析
4. darwin（Getfsstat）/ linux（/proc/mounts + Statfs），**0 新依赖**
5. 采集层平台分文件，渲染/算法是纯函数全测；CLI 回调可注入
