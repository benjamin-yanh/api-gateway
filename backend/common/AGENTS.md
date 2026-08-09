# AGENTS.md — `common`

这里放全局基础设施和跨层工具，不放具体业务策略。

## 先看哪里

- `json.go`：统一 JSON 入口
- `database.go`、`redis.go`：基础设施初始化
- `body_storage.go`：请求体复用
- `rate-limit.go`、`limiter/`：限流底座
- `sys_log.go`、`pprof.go`、`pyro.go`：日志与观测

## 下一跳

- 改 JSON 行为：先看 `json.go`
- 改请求体缓存或可重读：看 `body_storage.go`
- 改限流机制：进入 `limiter/`
