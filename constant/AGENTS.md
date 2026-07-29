# AGENTS.md — `constant`

这里放跨模块共享常量：渠道类型、API 类型、上下文键、任务类型。

## 先看哪里

- `api_type.go`
- `channel.go`
- `context_key.go`
- `task.go`

## 下一跳

- 新增渠道或任务平台：先改这里，再去 `relay/`
- 排查上下文值：看 `context_key.go`
