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

## 讨论 2026-07-29：最后 1M Token 余额保护

### 人格：学习者作者

- 关注点：读者容易把“最后 1M”理解成单请求输出上限。
- 发现：阈值、预计输出和实际写入上游的 max 是三个不同概念。
- 代码证据：`relay/helper/price.go:95-98`、`types/price_data.go:48-49`。
- 争议：是否所有请求进入保护区后都按最贵模型计算。
- 最终决议：最贵模型只决定进入保护区；本次请求按实际模型/分组价格反算 effective max。
- 是否影响文档：是，加入术语表、算例和常见误区。

### 人格：普通开发者读者

- 关注点：何时修改 max、余额不足返回什么、为什么可能截断。
- 发现：仅修改解析 DTO 会被 PassThrough 或渠道 ParamOverride 绕过。
- 代码证据：`relay/compatible_handler.go`、`relay/responses_handler.go`、`relay/claude_handler.go`、`relay/gemini_handler.go`。
- 争议：严格模式是否继续支持无法 patch 的透传。
- 最终决议：定价前写 DTO，ParamOverride 后做最终 outbound clamp；无法安全 patch 时 fail closed。
- 是否影响文档：是，新增“双重 Clamp”章节。

### 人格：资深开发者

- 关注点：并发原子性、取消尾部、崩溃恢复和跨数据库。
- 发现：钱包/Token 当前先读后无条件扣减；BillingSession 内存标志不足以恢复；取消后的本地 Usage 可能漏掉隐藏推理和未送达 Token。
- 代码证据：`service/billing_session.go`、`model/user.go:1257`、`model/token.go:412`、`relay/helper/stream_scanner.go:298`。
- 争议：只做条件 UPDATE 是否足够。
- 最终决议：条件 UPDATE 解决余额竞争，同时新增 request_id 唯一的持久 Reservation，状态迁移与额度变更同事务提交。
- 是否影响文档：是，新增状态机、跨库策略和崩溃注入测试。

### 人格：项目参与者维护者

- 关注点：现状与提案边界、RelayKit 独立构建、协议覆盖。
- 发现：Responses Background 当前没有完整 Cancel/查询入口；Realtime 是 response.done 后扣费；Claude/Cohere 有局部默认 max，但不是余额驱动统一上限。
- 代码证据：`relaykit/dto/openai_request.go`、`relay/channel/openai/relay_realtime.go`、`relay/claude_handler.go`。
- 争议：是否一期覆盖 Background 和 Realtime。
- 最终决议：一期 HTTP 文本闭环；保护区内 Background 暂拒绝；Realtime 和异步协议分阶段实现。
- 是否影响文档：是，明确协议矩阵和分阶段上线。

### 人格：博客编辑与观察者

- 关注点：避免把计费现状、外部调研和实施方案堆在一个页面。
- 发现：14 已讲风险，15 已讲外部机制，新方案适合独立成第 16 章。
- 最终决议：新增 `16-balance-protection-max-token.html`，三张独立 SVG，并从动态导航和调研页接入。
- 是否影响文档：是，形成“现状 → 调研 → 方案”的阅读路径。

### 验收记录

- HTML5 解析：通过，`parse5` 未报告结构错误。
- 相对链接与图片：全站 18 个 HTML 页面引用均可解析，余额保护页 3 张图均有 `alt`。
- SVG：XML 解析通过，均包含 `role="img"` 和 `aria-label`；正文提供图题和读图说明。
- 响应式检查：页面复用 `gateway.css`，窄屏卡片改为单列，宽图位于可横向滚动的 `.diagram` 容器，宽表格由章节容器承接横向滚动。
- 可视化验收：通过本地只读 HTTP 服务在应用内浏览器完成 1280 px 桌面与 390 px 窄屏检查；无缺图、无控制台错误，窄屏页面本身无横向溢出。

## 2026-07-29：最后 1M Token 保护第一阶段实现复审

### 人格：普通开发者读者

- 关注点：配置如何开启、限制发生在哪里、为什么返回 403。
- 发现：需要把“最后 1M 是阈值”与“本次 effective max 是限制”分开，并明确默认关闭。
- 代码证据：`setting/operation_setting/quota_setting.go`、`controller/relay.go`、`service/balance_protection.go`。
- 争议：客户端主动传入低于 64 的 max 是否应被拒绝。
- 最终决议：最低 64 只约束余额反算能力；客户端主动选择更小响应不应被误拒。
- 是否影响文档：是，更新配置表、请求公式和错误语义。

### 人格：资深开发者准确性审阅

- 关注点：并发、参数覆盖、透传、CompletionRatio 和崩溃一致性。
- 发现：只改解析 DTO 会被 `param_override` 或 BodyStorage 透传绕过；普通倍率旧预扣没有给最大输出应用 CompletionRatio；单表先查后减存在 TOCTOU。
- 代码证据：`relay/common/balance_protection.go`、四类协议 Handler、`relay/helper/price.go`、`model/user.go`、`model/token.go`。
- 争议：两次条件更新是否可以称作完整原子 Reservation。
- 最终决议：第一阶段只宣称“单表条件扣减不为负”；Token 与资金源之间仍由进程内补偿，持久 Reservation/事务 Saga 保留为第二阶段。
- 是否影响文档：是，状态机明确标成后续目标，不能描述成当前数据库状态。

### 人格：项目参与者维护者

- 关注点：GlobalConfig 热更新、配置 data race、数据库兼容和协议扩展。
- 发现：tiered ceiling 是热路径并发读对象，原生 map 不安全；保护模式与延迟批量扣减不兼容。
- 代码证据：`types/rw_map.go`、`model/option.go:validateOptionValue`、`service/balance_protection.go`。
- 争议：是否为保护价格加缓存。
- 最终决议：第一阶段不缓存模型/资金快照，以低余额区安全优先；ceiling 使用 `RWMap`；发现 `BatchUpdateEnabled=true` 时 fail closed。
- 是否影响文档：是，登记六个真实 Option Key 和启用前置条件。

### 实现验收

- 配置：总开关默认 false；阈值配置以万 Token 为单位，默认 100（换算后为 1,000,000 Token）；安全系数 1.10；最低输出 64；站点硬上限 1,000,000；tiered ceiling 默认空 Map。
- 管理界面：计费设置页提供余额保护开关和“余额保护阈值”整数输入；输入值直接表示多少万 Token，七种前端语言均已补齐单位和默认值说明。
- 协议：OpenAI Chat/Completions、Responses、Claude、Gemini 已覆盖 DTO 写入和出站二次 Clamp；Realtime 明确未覆盖。
- 并发：钱包和有限 Token 使用带余额条件的 GORM UPDATE 并检查 `RowsAffected`；SQLite 并发回归测试验证不会扣成负数。
- 计费：普通倍率预扣已拆为输入费用与带 CompletionRatio 的最大输出费用。
- 严格区资金：预扣完整有限可用额度，以覆盖工具、缓存和本地无法可靠预测的附加费用；结算后退还差额。
- 上游风险：一旦 dispatch，不再切换第二个渠道；错误或 Usage 未知时不退款，管理员审计写 `usage_unknown=true`。
- 审计：保护命中信息写入消费日志 `other.admin_info.balance_protection`，普通用户日志不会暴露内部价格边界。
- 测试：后端根模块各业务包显式全量测试通过；新增用例覆盖万 Token 默认换算、最小/最大边界、零值/负值/小数/非数字、乘法溢出、热加载和实际阈值 quota。前端新增表单 Schema 用例，覆盖 100 万默认值及非法输入；类型检查、生产构建、目标文件 lint、七语言 i18n 同步以及 HTML/SVG 结构和链接检查均通过。全仓 lint 仍报告项目既有规则违例，本次修改文件的定向 lint 无报错。
- 页面验收：应用内浏览器以 1280 px 和 390 px 视口完成实测；桌面无资源错误，移动端正文宽度保持 390 px，表格和流程图仅在各自容器内横向滚动。
