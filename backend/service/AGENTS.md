# AGENTS.md — `service`

这里是业务规则层，不负责路由注册，也不直接承载数据库模型定义。

## 先看哪里

- `billing.go`、`quota.go`、`tiered_settle.go`
- `channel_select.go`、`channel_affinity.go`
- `token_estimator.go`、`token_counter.go`
- `task.go`、`task_polling.go`、`task_billing.go`
- `passkey/`、`openaicompat/`

## 下一跳

- 排查中继规则：先看计费、渠道选择、token 估算
- 排查异步任务：进入 `task*.go`
- 排查 OpenAI 兼容转换：进入 `openaicompat/`
