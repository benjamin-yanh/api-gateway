# AGENTS.md — `model`

这里是持久化层：GORM 模型、迁移、缓存装载、跨数据库兼容。

## 先看哪里

- `main.go`
- `user.go`、`token.go`、`channel.go`
- `option.go`
- `subscription.go`、`pricing.go`、`topup.go`

## 下一跳

- 改迁移或数据库初始化：先看 `main.go`
- 改某类业务数据：找同名实体文件
- 写原生 SQL 前：先确认三数据库兼容
