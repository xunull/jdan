# jdan size — 目录体积排行

扫描目录树，按占盘大小排行，带占比条形图。

省掉这串管道：

```bash
du -sh * 2>/dev/null | sort -hr | head -20
```

`sort -hr` 在 BSD（macOS 自带）和 GNU coreutils 上行为还不一致，脚本没法直接抄。

```
$ jdan size ~/.claude
/Users/quincy/.claude  784.7Mi  （11,039 个文件）

  projects         577.7Mi  █████████████░░░░  73.6%
  plugins           79.3Mi  ██░░░░░░░░░░░░░░░  10.1%
  transcripts       74.8Mi  ██░░░░░░░░░░░░░░░   9.5%
  file-history      49.6Mi  █░░░░░░░░░░░░░░░░   6.3%
  shell-snapshots    1.2Mi  ░░░░░░░░░░░░░░░░░   0.1%
  其他 9 项          1.4Mi  ░░░░░░░░░░░░░░░░░   0.2%

用时 36ms
```

## 最重要的一件事：它量的是「实际占盘」，不是 Finder 显示的大小

一个文件有两个「大小」：

- **逻辑大小**（`st_size`）：文件内容有多长。Finder 的「显示简介」用这个
- **实际占盘**（`st_blocks × 512`）：它在硬盘上真正占了多少块。`du` 用这个

`jdan size` 默认用后者，因为你问的是「删掉能腾出多少空间」。

两者差得比直觉大，而且**方向经常被搞反**：

| 场景 | 逻辑大小 | 实际占盘 | 倍数 |
|---|---|---|---|
| 500 个 1 字节文件 | 500 B | 2.0 MB | **4000×** |
| `/etc/hosts` | 366 B | 4096 B | 11× |
| 1 GiB 稀疏文件 | 1 GiB | 0 B | 反向 |

对 `node_modules`、`.git`、`Caches` 这类**大量小文件**的目录，4 KiB 块向上取整是主导项，实际占盘远大于逻辑大小。压缩和稀疏文件才是反方向的少数情况。

要看 Finder 那个数字用 `--apparent`：

```bash
jdan size ~/Library --apparent
```

## 与 du 的关系

**根总量与 `du -sh` 完全一致**（逐字节，不是「误差 1% 以内」）。语义全部对齐：

- 硬链接只计一次（`du` 默认也去重，`du -l` 才不去重）
- 默认不跨文件系统（对齐 `du -x`）
- 不跟随符号链接，链接自身按其占盘计
- 目录自身的块计入总量（ext4 上每个目录通常 4096 B，APFS 上是 0）

**一处刻意不同：单个子目录的数字可能与 `du` 不一样。**

一个硬链接文件被多棵子树同时看到时，只能算在其中一棵下面。`du` 算给「先遇到的」那条路径 —— 这在并发扫描下取决于线程调度，同一棵树跑两次各子树的数字会互换，占比图会跳。

`jdan size` 算给**字典序最小的路径**。与遍历顺序无关，因此：

- 同一棵树连跑 N 次，`--json` 输出逐字节相同
- `--jobs 16` 与 `--jobs 1` 结果逐字节相同

代价就是子目录数字可能和 `du` 对不上。根总量一致。

页脚会告诉你去重了多少：

```
用时 2.3s / 2,640 个硬链接已去重
```

## 常用法

```bash
jdan size                       # 当前目录，前 10 名
jdan size ~/Library             # 指定目录
jdan size --depth 3             # 展开三层
jdan size --top 20              # 每层显示 20 项
jdan size --files               # 把文件也列出来（找单个大文件）
jdan size --all                 # 含隐藏文件（du 默认就含，jdan 默认不含）
jdan size --one-file-system=false   # 跨越挂载点
jdan size --json | jq           # 全树 JSON
```

### 找「哪一个文件」吃了空间

默认只列目录，看不到单个文件。`--files` 才能：

```
$ jdan size ~/.claude/projects/xxx --files --top 5
/Users/quincy/.claude/projects/xxx  58.8Mi  （22 个文件）

  05acc370-….jsonl   54.4Mi  ████████████████░  92.5%
  21578d18-….jsonl    3.1Mi  █░░░░░░░░░░░░░░░░   5.3%
  05acc370-…         772.0Ki  ░░░░░░░░░░░░░░░░░   1.3%
```

一个 54 MB 的 jsonl 占了整个目录的 92.5%。

`--files` 只改变呈现粒度，不改变统计：加不加它，总量和文件计数完全相同。

## 全部选项

| 选项 | 默认 | 说明 |
|---|---|---|
| `--depth N` | 1 | 展开层数。1 = 根 + 直接子项 |
| `--top N` | 10 | 每层最多显示几项，其余合并为「其他 N 项」。0 = 不限 |
| `--apparent` | 关 | 用逻辑大小而非实际占盘 |
| `-x, --one-file-system` | 开 | 不跨越文件系统边界。跨盘用 `--one-file-system=false` |
| `-a, --all` | 关 | 含隐藏文件和目录 |
| `--files` | 关 | 把文件也建成节点列出 |
| `--jobs N` | 自动 | 并发度。0 = min(CPU 数, 16) |
| `--no-color` | 关 | 关闭染色（也尊重 `NO_COLOR` 环境变量） |
| `--verbose` | 关 | 列出每条无权访问的路径 |
| `--json` | 关 | 输出全树 JSON |

## 权限错误不会中断

碰到 `/private/var/db` 这类无权访问的目录时，收集但不中断，扫完在页脚汇总：

```
用时 1.2s / 3 个目录无权访问，结果可能偏小（--verbose 看详情）
```

退出码仍是 0 —— 不 `sudo` 也要能给出有用的部分结果。`--verbose` 看具体是哪些路径。

只有这几种情况退出码为 1：路径不存在、参数是文件而不是目录、起始目录本身读不了。

## JSON 输出

```bash
jdan size ~/Library --json | jq '.children[0]'
```

```json
{
  "path": "/Users/quincy/Library",
  "type": "dir",
  "bytes": 92650000000,
  "files": 412883,
  "children": [
    { "path": "…/Caches", "type": "dir", "bytes": 33500000000, "files": 128441, "children": [] }
  ],
  "apparent": false,
  "supported": true,
  "deduped": 2640,
  "errors": [{ "path": "…", "error": "permission denied" }]
}
```

三条约定：

- **`--top` 和 `--depth` 不作用于 JSON**，永远输出全树。那两个是展示层的裁剪，让它们影响 JSON 会让下游拿不到完整数据
- **`type` 字段**区分 `dir` / `file`。默认模式下文件体积折叠进目录、只有 `--files` 才有 file 节点，没这个字段消费者无从判断
- **`apparent` / `supported` / `deduped` / `errors` 只在根对象**，子节点不重复

`errors` 按路径升序，`children` 按 (bytes 降序, path 升序) —— 都是为了让连跑多次的输出逐字节相同。

## 性能

并发遍历。目录遍历是 IO bound 不是 CPU bound，所以并发度按存储介质定而不是 CPU 核数：SSD 上 8-16 最好，机械盘建议 `--jobs 2` 到 `--jobs 4`（并发反而因寻道变慢）。

实测 `~/.claude`（11793 文件，**热缓存**）：

| `--jobs` | 耗时 | 加速 |
|---|---|---|
| 1 | 194ms | 1× |
| 4 | 35ms | 5.5× |
| 8 | 31ms | 6.3× |
| 16 | 33ms | 5.9× |

冷缓存下瓶颈在元数据 IO，倍数会明显低于这个。

扫大目录时 stderr 会显示进度（`已扫描 123,456 个条目…`），管道和 `--json` 下自动静默。

## 平台

darwin / linux 是一等公民。其他平台（Windows、FreeBSD 等）拿不到 `st_blocks`，会降级为逻辑大小，并在输出头部明确提示：

```
注：本平台无法测量实际占盘，以下为逻辑大小（Size()）。
```

降级模式下硬链接去重和跨文件系统检测也会关闭（都依赖 inode 信息）。

## 相关命令

- `jdan disk` — 挂载点级别的容量（`df` 式）。先用它看哪个分区满了，再用 `size` 看是谁占的
- `jdan tree2` — 目录结构速览，不算体积
