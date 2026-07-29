# AGENTS.md — `relay`

这里是统一 AI 中继层：协议识别、请求转换、上游适配、流式处理、任务型接入。

## 先看哪里

- `relay_adaptor.go`
- `common/`
- `helper/`
- `channel/`
- `*_handler.go`

## 下一跳

- 找某个渠道实现：从 `relay_adaptor.go` 跳到 `channel/<provider>/`
- 排查共享中继行为：看 `common/`
- 新增任务型平台：进入 `channel/task/`
