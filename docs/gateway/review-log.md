# 网关文档多人格审阅记录

> 观察者职责：记录“谁提出问题、代码证据是什么、编辑如何裁决”。只记录可验证结论，不把个人猜测写成项目行为。

## 角色

- **学习者作者**：先用场景、curl 和术语建立心智模型。
- **普通开发者读者**：检查章节是否能沿请求时间线读懂，并能定位 401/429/503。
- **资深开发者读者**：检查并发、重试、BodyStorage、计费不变量和适配器契约。
- **项目参与者维护者**：检查配置来源、默认值、热加载、扩展和数据库/缓存影响。
- **博客编辑**：控制每章一个问题、先结论后证据、避免把所有内容塞进总览。
- **观察者**：维护本记录和交付前检查项。

## 讨论 2026-07-15

### 议题：普通 Token 是否固定渠道

- 普通开发者：入口图写“Token→渠道”容易让人误解成永久绑定。
- 资深开发者：`Distribute` 顺序是指定渠道、亲和缓存、满足条件的优先级/权重选择；普通 Token 没有固定 Channel ID。
- 项目参与者：亲和性只在规则 Key、模型、路径、分组和 TTL 条件满足时复用。
- 编辑裁决：正文使用“Token 提供身份和约束；Distribute 每次选择渠道”，指定渠道、亲和性、普通调度分成三个小节。
- 证据：`middleware/distributor.go:32`、`service/channel_select.go:84`、`service/channel_affinity.go`。

### 议题：指定渠道是否会重试到别的渠道

- 普通开发者：错误状态码默认会重试，想知道管理员指定是否例外。
- 资深开发者：`shouldRetry` 先检查 affinity skip，再检查 channel error/skip retry，随后在 `specific_channel_id` 存在时阻断普通状态码重试。
- 项目参与者：指定渠道分支只校验渠道存在和启用状态，不应宣称已完成模型能力筛选。
- 编辑裁决：协议和排障页明确“管理员指定是本次请求强制渠道；普通重试被阻断；适配器仍可能因能力不匹配失败”。
- 证据：`middleware/auth.go:SetupContextForToken`、`middleware/distributor.go:35`、`controller/relay.go:325`。

### 议题：预扣是不是最终扣费

- 学习者：用“先扣钱”解释会误导免费模型和失败退款。
- 资深开发者：先估算 Token/价格并 `PreConsumeBilling`，响应后按 Usage 结算；错误路径退款或违规扣费。
- 项目参与者：动态计费还要检查 max token、图片 n、任务时长等上限和 quota 饱和审计。
- 编辑裁决：计费页使用“预扣—结算—退款闭环”，并在配置和排障页重复安全边界。
- 证据：`controller/relay.go:145-179`、`service/billing_session.go`、`common/quota_math.go`。

### 议题：普通读者和资深读者如何共用一套文档

- 普通开发者：每页先给一句话、流程图和代码路径。
- 资深开发者：需要 Context keys、错误边界、默认值和扩展契约。
- 编辑裁决：总览提供三条阅读路径；每专题采用“场景→流程→代码事实→维护者提示”；配置和协议独立成查表页。

## 交付前观察清单

- [x] Router、TokenAuth、Distribute、Relay、Adaptor、Billing、Log 均有入口。
- [x] 指定渠道、亲和性、普通调度、auto 跨组和 retry 边界已区分。
- [x] 配置表包含来源、默认、生效阶段、热加载、影响、安全风险、代码证据。
- [x] 协议矩阵覆盖 Chat、Completions、Responses、Claude、Gemini、Rerank、Realtime、Midjourney、Suno，并标注 `RelayNotImplemented`。
- [x] 排障表使用症状→检查点→日志/代码结构，避免泄露密钥建议。
- [x] SVG 图与 HTML 分离；总览提供普通开发者、资深开发者和运维阅读路径。

