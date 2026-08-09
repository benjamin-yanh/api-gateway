# AGENTS.md — `middleware`

这里是请求链路控制面：鉴权、限流、分发、日志、统计。

## 先看哪里

- `auth.go`
- `distributor.go`
- `rate-limit.go`、`model-rate-limit.go`
- `stats.go`、`logger.go`、`request-id.go`

## 下一跳

- 请求为何命中某个渠道：看 `distributor.go`
- 权限失败：看 `auth.go`
- 请求被拒绝或变慢：结合限流和统计一起看
