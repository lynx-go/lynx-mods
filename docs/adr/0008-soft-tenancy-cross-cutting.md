# 软多租户横切约定——organization_id 即唯一租户键

2026-10-26 决策。路线图挂了三批的「tenancy 越晚决定 retrofit 越痛」终于
兑现：organization 模块已落地（组织与成员事实），第一个**业务资源模块**
出现之前是定横切约定的最佳（也是最后便宜的）时机——目前尚无任何业务
资源表需要 retrofit，此后每多一个都是改表+改查询+改约束。

继承关系：ADR-0002（已 superseded，素材保留）定的方向不变——GitHub 式
组织模型、共享表 + `org_id` 键、否决 schema-per-tenant 与 RLS 默认路径；
契约⑥（ADR-0004）不变——kernel 无租户预设。本 ADR 定的是**模块层的
落地约定**。

## 背景

- 「租户」在本体系里的语义：**组织（organization 模块的 Organization
  聚合）**。用户全局（identity），组织是资源与授权的挂靠单位——GitHub
  模型本义。
- 现状：organization v0.1 是扁平成员 + `IsOwner` 结构标记；access 的
  `Assignment{UserID, Role}` 无组织维度（全局角色）；identity/session/
  notification/audit 全部无租户概念。
- 待定的横切问题：业务模块的资源怎么挂组织；查询防漏的姿势与自动化
  程度；组织级 RBAC 的演进路径；软多租户到硬隔离的升级保险。

## Considered Options

- **A. 软多租户 + 模块纪律 + 租户键不变量（选定）**：organization_id
  作为唯一租户键写进约定，业务资源模块第一天按标准姿势挂键；查询
  防漏靠 app 层成员资格校验 + repo 层显式过滤的纪律姿势；自动化
  横切（强制过滤 hook）等真实压力再评估。
- **B. 硬多租户默认开**（行级强制过滤：bun query hook 或 RLS）：对
  「公司内部平台/一般 SaaS」过重；RLS 运维与调试成本 ADR-0002 已否；
  且强制过滤层一旦有 bug 是静默丢数据级别的故障模式——纪律姿势的
  bug 是显式漏过滤（e2e 可逮）。RLS 仍是文档级进阶选项（应用侧自接）。
- **C. schema-per-tenant / 库级隔离**：BaaS 特殊需求的产物（ADR-0002
  理由继承），不在 registry 模块覆盖面。
- **D. GitHub 完整形态的 owner 多态**（资源属于个人或组织，两级归属）：
  是 GitHub 的产品选择而非通用默认。两级归属让每个资源查询多一个
  分支、唯一约束与权限判定复杂化。v1 收窄为「资源必挂组织」；真需要
  时 additive 演进（加 owner_type 列），不破坏既有键。

## Decision

1. **租户键不变量（本 ADR 最重要的一条）**：租户键 = `organization_id`
   （text，只存 ID 不设外键——契约④同款），**永不引入第二个租户概念**
   （不出现 tenant_id/workspace_id 等平行列——双租户概念并存是 retrofit
   最痛源，见竞品教训）。软→硬升级（行级过滤/物理分库/RLS）永远是
   **在既有键上加层**，数据模型零改动。
2. **业务资源模块的标准姿势**（module-authoring 已同步成文）：
   - 表：`organization_id text NOT NULL`；**复合唯一约束必含
     organization_id**（跨组织重名合法——ADR-0002 教训继承）；列表
     索引以 organization_id 打头；
   - app 层：资源读写前经通道②（ADR-0007：organization domain 只读
     端口）校验 actor 成员资格；repo 查询显式携带 organization_id；
   - proto：资源路由挂组织层级
     `/v1/organizations/{organization_id}/<resources>`（REST 归属表达）；
   - `DependsOn: organization`（安装器 DAG 自动带）。
3. **查询防漏：纪律姿势，不建横切机制**。v1 无强制过滤 hook——机制
   下沉需要真实压力（与 M4 暂缓、wire 弃用同一哲学）；第一个多组织
   业务资源模块落地后，若 e2e/review 仍出现漏过滤，再评估 kernel bun
   层的租户过滤横切（届时是有用例支撑的机制改进）。护栏先行项：
   组织资源场景进 e2e 组合矩阵。
4. **组织级 RBAC 演进路径（预留，不实现）**：access 的 Assignment 未来
   加可空 `organization_id` 维度——空 = 全局角色（现状语义不变），非空 =
   组织内角色；角色**目录**保持全局（access.roles 不分层），授予分层。
   organization 的 `IsOwner` 维持结构标记定位（不是角色）。team（组织内
   分组）是 organization 的结构扩展，team 级授权届时随 Assignment 第三
   维度一并定。顺序确认：access 组织维度 → team → 组织级角色表达。
5. **账号域模块不 retrofit**：identity/session 的用户与会话全局是
  GitHub 模型本义；notification 挂 user、audit 是全局审计视角（读权限
  控制）——都保持现状，未来需要租户维度时 additive 加列（如审计按
  组织过滤），不属破坏性。

## Consequences

- 第一个业务资源模块（todo/project 类）落地时有成文姿势可抄，无需
  再辩论表结构；组织资源 e2e 组合进入 CI matrix 的扩展位。
- `IsOwner`/access/team 三者的演进边界清晰：结构标记（组织管理授权）→
  全局角色 → 组织角色（Assignment 加维度）→ team（第三维度）。
- 硬隔离升级路径有保险：键已在、概念唯一，加过滤层/分库路由不改模型。
- 防漏横切被明确「暂缓而非否决」，重启条件成文（漏过滤真实出现）——
  与 M4 重启扳机同款纪律。
- kernel 完全不动：本 ADR 全部落在模块约定与文档层，契约⑥原样。
