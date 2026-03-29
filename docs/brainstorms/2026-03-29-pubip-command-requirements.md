---
date: 2026-03-29
topic: pubip-command
---

# pubip 子命令

## Problem Frame
用户需要快速查询当前出口的公网 IP 地址，分 IPv4 和 IPv6 两个独立子命令。

## Requirements

**`jdan pubip4`**
- R1. 输出本机当前公网 IPv4 地址，纯文本，仅地址本身，无其他修饰
- R2. 若所有重试均失败，输出一行提示信息（例如 `无法获取 IPv4 地址`）并以非零退出码退出

**`jdan pubip6`**
- R3. 输出本机当前公网 IPv6 地址，纯文本，仅地址本身，无其他修饰
- R4. 若所有重试均失败，输出一行提示信息（例如 `无法获取 IPv6 地址`）并以非零退出码退出

**重试机制**
- R5. 内部重试至多 3 次
- R6. 每次失败时重新发起请求，不等待重试

**查询方式**
- R7. 通过 HTTPS 请求公网 IP 查询服务（如 ipify、icanhazip 等）获取地址

## Success Criteria
- `jdan pubip4` 在有网络且服务可用时输出纯 IPv4 地址
- `jdan pubip6` 在有网络且服务可用时输出纯 IPv6 地址
- 无网络或服务不可用时重试 3 次后输出提示信息并报错退出
- 两次子命令相互独立，无共享状态

## Scope Boundaries
- 无 JSON 输出选项
- 不做 IP 格式校验（直接透传查询服务返回结果）
- 不做历史记录或缓存

## Dependencies / Assumptions
- 依赖公网 IP 查询服务（需选择稳定可靠的服务）

## Outstanding Questions

### Resolve Before Planning
- [R7] 使用 ipify：
  - IPv4 查询：https://api.ipify.org
  - IPv6 查询：https://api6.ipify.org

## Next Steps
→ `/ce:plan` for structured implementation planning
