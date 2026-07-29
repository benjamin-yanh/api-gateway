# AGENTS.md — `router`

这里是 HTTP 入口地图，只负责挂路由和划分访问面。

## 先看哪里

- `main.go`
- `api-router.go`
- `relay-router.go`
- `web-router.go`

## 下一跳

- 找接口挂载位置：先来这里
- 排查 `/api`：进 `api-router.go`
- 排查 `/v1`、`/mj`、`/suno`：进 `relay-router.go`
