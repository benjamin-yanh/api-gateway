# Telegram 模型请求异常告警

## 接入范围

- 渠道分配失败、没有可用渠道：例如 `model_not_found`、HTTP 503。
- `controller.Relay` 主转发链路最终错误：包括重试仍失败、上游异常、
  返回到该处理器的流式错误。重试最终成功时不告警。
- 不承诺覆盖进程崩溃、Nginx 错误、请求认证前拒绝，或没有经过主 Relay
  处理器的独立异步任务/媒体任务状态错误。这些应另接健康监控和任务告警。

消息仅包含 UTC 时间、模型、分组、HTTP 错误状态、错误码和 request id。
不发送原始错误消息、请求内容、响应内容、API Key 或用户身份。

## 配置 Telegram

1. 创建或使用已有 Bot，将 Bot 加入目标频道并授予发布消息权限。
2. 获取频道数值 `chat_id`（通常以 `-100` 开头）；公开频道也可使用 `@username`。
   私密 `t.me/+...` 邀请链接不是 chat_id，Bot API 不能以邀请链接发送。
3. 在中转端 root 专用的 `/etc/new-api/telegram-relay-alert.env` 中配置，
   通过 `new-api-relay.service.d/telegram-alert.conf` 的 EnvironmentFile 加载：

   ```text
   RELAY_ALERT_TELEGRAM_BOT_TOKEN=<Bot Token>
   RELAY_ALERT_TELEGRAM_CHAT_ID=<频道 chat_id>
   ```

   使用服务器凭据管理方式保存，不提交 Token 到 Git。不复用 Telegram OAuth
   的登录配置，避免意外发给另一 Bot 的目的地。只有合法格式的两个值都存在
   时启用通知；移除任意一个并重启服务即可关闭。
4. 部署包含本功能的中转端并重启，确保服务器可以通过 HTTPS 访问
   `api.telegram.org`。随后发起一次受控的无可用渠道请求，确认频道收到了
   与 API 返回相同 request id 的告警。

Bot API 官方说明：https://core.telegram.org/bots/api#sendmessage

## 发送及去重边界

- 单进程按分组、模型、状态码、错误码去重，固定 5 分钟；request id 不参与
  去重。进程重启会重置窗口，多节点可各发一条。
- 后台单 worker、64 条有界队列、最多 1024 个去重键，单次 HTTPS 请求超时
  10 秒，禁止跟随重定向。使用纯文本，不启用 Markdown 或 HTML 解析。
- 这是尽力发送的运维通知：队列满时丢弃，进程退出时不持久化待发通知；
  Telegram 发送失败写不含 Token 的固定错误日志，不自动重试。成功返回必须
  同时满足 HTTP 200 及 Telegram `ok: true`。
- 告警失败不改变 API 响应和账本，不影响模型请求的错误返回。

## 验证

`go test ./service ./middleware ./controller -run 'Test.*(RelayAlert|Distribut|Relay)'`
覆盖消息请求、Telegram 失败响应、重复故障抑制、窗口到期和队列满时的行为。

## 当前安装的凭据来源

本机 `TELEGRAM_GTONGXUE_WARM_BOT_TOKEN` 同步到远程同名变量，并映射为
`RELAY_ALERT_TELEGRAM_BOT_TOKEN`。`TELEGRAM_GTONGXUE_WARM_BOT_URL` 同步用于
标识机器人主页；机器人主页不是告警频道，不可直接用作 chat_id。
需要机器人加入目标频道且有发布权限，再设置 `RELAY_ALERT_TELEGRAM_CHAT_ID`。
缺少频道配置时通知保持关闭，API 请求照常处理。凭据文件权限为 root:root 0600。

若服务器不能直连 `api.telegram.org:443`，需在该专用 EnvironmentFile 配置
仅用于告警的 `RELAY_ALERT_PROXY_URL`，再重启
中转服务。不要配置仅本机可达的 `127.0.0.1` 代理，也不要关闭 TLS 校验。

## AWS SSH 告警出网

生产中转服务通过 `new-api-telegram-tunnel.service` 建立本机
`127.0.0.1:18081` SOCKS 代理，复用 AWS SSH 22 端口，无需开放额外公网端口。
`RELAY_ALERT_PROXY_URL=socks5://127.0.0.1:18081` 只配置告警 HTTP 客户端，
不设置全局 HTTPS_PROXY，不改变模型上游请求的出网。

AWS 使用独立无登录账号 `newapi-alert-egress`；专用公钥限制目的地
`api.telegram.org:443`。私钥在中转服务器 `/etc/new-api/telegram-tunnel-key`
生成并以 0600 保存，不复制本机管理员私钥，不将 Bot Token 传给 AWS。
可信主机公钥经已验证的管理 SSH 连接取得并单独存放，严格校验 SSH/TLS。
已有 AWS 项目的 443、配置、数据库和服务均不调整。

隧道由 systemd 开机启动并自动重连。排障先检查隧道 active 和本机监听，
再检查 Telegram 请求；不要把代理绑定到 0.0.0.0。回退时从告警环境文件移除
RELAY_ALERT_PROXY_URL 并按网络情况停用告警，再停用隧道、移除 AWS 专用账号
的授权密钥。已有业务服务不需要重启。
