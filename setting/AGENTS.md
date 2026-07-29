# AGENTS.md — `setting`

这里是动态配置目录，负责把数据库中的选项映射成业务域配置。

## 先看哪里

- `config/config.go`
- `operation_setting/`
- `model_setting/`
- `ratio_setting/`
- `system_setting/`

## 下一跳

- 找配置所属域：先看子目录名
- 改后台设置：通常要同步这里和 `web/src/components/settings/`
- 改定价或比例：进入 `ratio_setting/` 或 `billing_setting/`
