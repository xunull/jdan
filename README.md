# jdan

Go 编写的常用小工具集合（单二进制）。

## 构建

```bash
# 若上级目录存在 go.work 导致构建报错，可临时：
# Linux/macOS: GOWORK=off go build -o jdan ./cmd/jdan
# Windows PowerShell:
$env:GOWORK="off"; go build -o jdan.exe ./cmd/jdan
```

## 命令

### `jdan file bak`

将**普通文件**复制到**同目录**下的备份文件，命名规则：

- 无 `--desc`（或 trim 后为空）：`{原完整文件名}.bak.{YYYYMMDD-HHMMSS}`
- 有 `--desc`：`{原完整文件名}.bak.{YYYYMMDD-HHMMSS}-{描述}`  
  描述：仅允许 **英文字母、ASCII 数字、汉字、ASCII 空格**；空格会变为 `_`。其它字符（标点、制表符等）会 **拒绝执行** 并打日志说明。
- 若目标备份路径已存在（同一时间戳）：**不复制**，报错提示「已存在相同时间戳的备份」。

```bash
jdan file bak ./report.pdf
jdan file bak ./report.pdf --desc "before edit"
```

### `jdan http timing`

测量 HTTP 请求各阶段耗时：DNS 查询、TCP 连接、TLS 握手、服务端处理、内容传输、总耗时，以及 HTTP 状态码。

```bash
jdan http timing https://github.com
jdan http timing https://github.com -n 3        # 请求 3 次，输出每次结果与平均值
jdan http timing https://github.com --json       # JSON 格式输出
jdan http timing https://github.com -n 3 --json  # 3 次 + JSON
jdan http timing https://example.com -k          # 跳过 TLS 证书验证
```

| 参数 | 说明 |
|------|------|
| `-n` | 请求次数（默认 1；大于 1 时追加平均值） |
| `--json` | 以 JSON 格式输出（Duration 以毫秒浮点数表示） |
| `-k` / `--insecure` | 跳过 TLS 证书验证 |

### 全局

- `--config`：可选配置文件路径；配置与环境变量前缀为 `JDAN`（见 Viper 惯例）。命令行标志优先于环境变量与配置文件。

## 开发

```bash
go test ./...
```
