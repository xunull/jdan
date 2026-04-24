---
date: 2026-04-09
topic: listening-ports
---

# 查看本地监听端口

## Problem Frame

开发者和运维人员在排查网络问题时，经常需要快速确认本机哪些端口正在监听、是被哪个进程占用。现有方案（如 `lsof -i -P -n`、`netstat -tuln`）输出格式不一，部分命令需要 sudo 才能看到进程名，且输出不易于脚本二次处理。

## Requirements

**命令与入口**
- R1. 新增子命令 `jdan ports`，展示本机所有处于 LISTEN 状态的端口。
- R2. 默认输出人类可读的表格；支持 `--json`（`-j`）flag 输出 JSON 格式，便于脚本集成。
- R3. 表格输出按协议分块展示（先 TCP 块，后 UDP 块），JSON 输出为单一数组。

**端口信息**
- R4. 每条记录包含：协议（TCP/UDP）、监听地址（如 `127.0.0.1:8080`、`*:443`、`[::1]:22`）、进程名。
- R5. 进程名通过 `lsof` 命令获取（`-P -n` 抑制端口/地址解析，`-sTCP:LISTEN` 过滤）；若无法获取进程名（如权限不足），进程名字段留空或显示 `-`。
- R6. 按端口号从小到大排序。

**协议过滤**
- R7. 支持 `--tcp`（`-t`）仅显示 TCP 端口，支持 `--udp`（`-u`）仅显示 UDP 端口；省略两个 flag 时默认显示 TCP + UDP。

**Docker 端口说明**
- R8. Docker 映射到宿主机的端口（如 `docker run -p 8080:80`）会被正常检测并显示，因为 Docker daemon 会在宿主机上创建真实的监听 socket。

## Success Criteria

- `jdan ports` 输出包含所有 LISTEN 状态的 TCP/UDP 端口，端口号正确，进程名准确。
- `jdan ports --json` 输出合法 JSON，字段完整。
- `--tcp` / `--udp` 过滤正确。
- 权限不足时仍能显示端口和地址，进程名显示为 `-`。
- 整体输出速度 < 1 秒。

## Scope Boundaries

- 不支持 Windows（`jdan` 目前仅有 macOS/Linux 的跨平台考虑，本命令专注 macOS）。
- 不显示 LISTEN 之外的状态（如 ESTABLISHED、TIME_WAIT）。
- 不支持进程 ID（PID）列，进程名已足够定位。
- 不做 Docker 容器内部的端口扫描（Docker EXPOSE 元数据无法从宿主机查询，除非已通过 `-p` 映射）。

## Key Decisions

- **命令名选 `ports`**：简短、直白，对应 "ports = listening network ports" 这一核心概念。
- **表格分块按协议划分**：用户查看 TCP 和 UDP 端口的使用场景不同，分开展示更清晰。
- **通过 `lsof` 获取进程名**：macOS 内置工具，结果可靠，无需引入额外系统调用库；`-sTCP:LISTEN` 是 macOS `lsof` 支持的语法，用于过滤 LISTEN 状态的 TCP 连接。
- **权限降级处理**：即使没有 sudo 也能显示端口和地址，只是进程名显示为 `-`；避免因权限问题完全无法使用。

## Dependencies / Assumptions

- 目标平台为 macOS（Linux 可作为后续扩展，不在本次范围）。
- `lsof` 在 macOS 上无需 sudo 也能获取大部分进程名，但部分系统进程可能受限。

## Outstanding Questions

### Resolve Before Planning

（无阻塞性问题，可直接进入规划）

## Next Steps

→ `/ce:plan` 进行结构化实现规划
