# AGENTS.md — `controller`

这里是接口编排层：收请求、调服务、写响应。

## 先看哪里

- `relay.go`：统一中继主链路
- `user.go`、`token.go`、`channel.go`：核心管理接口
- `subscription.go`、`topup_*.go`：支付与订阅
- `oauth.go`、`passkey.go`、`twofa.go`：认证安全

## 下一跳

- 排查 Relay 请求：先看 `relay.go`
- 排查后台资源接口：按资源名找同名文件
- 遇到复杂规则：继续去 `service/`
