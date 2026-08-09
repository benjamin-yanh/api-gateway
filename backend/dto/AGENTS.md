# AGENTS.md — `dto`

这里放协议层 DTO，尤其是会被客户端解析后再次转发到上游的请求结构。

## 先看哪里

- `openai_request.go`、`openai_response.go`
- `claude.go`、`gemini.go`、`realtime.go`
- `audio.go`、`video.go`、`openai_image.go`
- `openai_request_zero_value_test.go`

## 下一跳

- 改协议字段：找对应协议文件
- 需要保留显式零值：先检查是否应使用指针字段
- 改规则而不是结构：去 `service/` 或 `relay/`
