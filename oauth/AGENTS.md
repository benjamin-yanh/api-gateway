# AGENTS.md — `oauth`

这里实现标准 OAuth Provider 及其注册机制。

## 先看哪里

- `provider.go`、`types.go`
- `registry.go`
- `github.go`、`discord.go`、`oidc.go`、`linuxdo.go`

## 下一跳

- 新增标准 Provider：先看接口和注册中心
- 排查回调：结合 `controller/oauth.go`
