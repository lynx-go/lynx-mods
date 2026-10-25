# lynx-start

[![CI](https://github.com/lynx-go/lynx-start/actions/workflows/ci.yml/badge.svg)](https://github.com/lynx-go/lynx-start/actions/workflows/ci.yml)

**lynx 生态的 shadcn 式模块基建**：Kernel 契约库 + 源码模块 registry + 安装 CLI。
为任何基于 lynx 的 Go 后端应用提供按需安装的业务域源码——安装后代码归你所有，
随便改。

[English intro below.]

## 定位：三层供给（ADR-0003）

| 层 | 形态 | 稳定性承诺 |
|---|---|---|
| 框架层 | lynx / grpcapi / bun / goose（本来就是库） | 随上游 |
| **Kernel**（`kernel/`，独立 Go module） | 装配协议、Principal 语境、错误码区段、crud 分页方言、uow 事务端口、事件信封、goosekit 迁移隔离 | **semver** |
| **源码模块**（`modules/`，registry items） | 一个业务域的四层实现（domain/app/api/infra + proto + 迁移 + 配置段） | 无承诺——源码即你的 |

「稳定」由 Kernel 承担（编译期类型契约），「可扩展」由源码模块承担（改的是自己的
代码，不是插件钩子），「演化」由 registry 承担（git tag 版本 + 安装基线三方合并更新，
安全敏感实现下沉 Kernel 经 semver 推送修复）。
shadcn/ui 是参照系：Radix 是库、组件是源码、registry 是分发协议。

## 快速开始

```bash
# 安装 CLI（Go 1.24+；或手工构建：cd cli && go build -o /usr/local/bin/lynx-start ./cmd/lynx-start）
go install github.com/lynx-go/lynx-start/cli/cmd/lynx-start@latest

# registry 两种形态：本仓库目录，或 git URL（自动 clone 到 ~/.cache 并增量刷新）
export LYNXSTART_REGISTRY=https://github.com/lynx-go/lynx-start.git
# 生成应用骨架 + 安装模块
lynx-start init myapp --module github.com/acme/myapp --registry <本仓库目录>
cd myapp
lynx-start add demo --registry <本仓库目录>      # 样例：验证装配管线
lynx-start init myapp-saas --preset saas --registry <本仓库目录>  # 或：一条命令装齐 SaaS 底座（identity+session+access+audit+organization+notification）
lynx-start add identity@0.1.1 --registry <本仓库目录>  # 可选：钉版本安装（git tag）
lynx-start add session --registry <本仓库目录>    # 会话：可吊销令牌对（依赖 identity，安装器校验）
lynx-start add access --registry <本仓库目录>     # 授权：RBAC + claims 注入（依赖 identity）
lynx-start add audit --registry <本仓库目录>      # 审计：RPC 拦截器自动记录 + 显式记录 + 查询（零依赖）
lynx-start add organization --registry <本仓库目录>  # 组织：组织与成员事实（创建者即 owner；依赖 identity，安装器校验）

# registry 模块出新版时（三方合并：本地 / 安装基线 / upstream）
git add -A && git commit -m wip    # update 写盘不可回滚，先 commit
lynx-start update identity --registry <本仓库目录>

# 体检（modules.go/config/基线/依赖一致性；⚠ 安全公告提示）与卸载
lynx-start doctor --registry <本仓库目录>
lynx-start audit --registry <本仓库目录>    # 安全公告门禁：命中已知公告即非零退出（CI 用）
lynx-start remove audit --registry <本仓库目录>   # DAG 反向校验；迁移表保留（数据安全）

# 跑起来（顺序重要：genproto 未生成时 go mod tidy 必失败；mise 钉版 go/buf）
mise install && mise trust
mise run generate               # buf dep update + lint + generate → genproto/
go mod tidy                     # kernel（semver）与模块依赖在此拉取
mise run db:up && mise run db:migrate
mise run dev                    # gRPC :9060 / HTTP :8080
```

写自己的业务域模块：见 [docs/module-authoring.md](docs/module-authoring.md)。

## 仓库结构

```
kernel/     契约库（独立 Go module，semver）——v0.1.0 候选包清单：
            module/assembly 装配协议与 DAG 校验 · errcode 错误码区段 ·
            contexts 主体语境 · events 事件信封与发布端口 ·
            uow+buntx 事务传播 · crud 分页方言（HMAC token + 时间游标）·
            goosekit per-module 迁移隔离 · secretkey 主密钥 purpose 派生 ·
            clock 时间源（Fake 注入）· idgen 随机 ID/前缀 ID ·
            password argon2id 哈希 · jwt HS256 签发/验证（purpose 隔离）·
            bunx 连接引导 + AIP-132 白名单排序 + 泛型列表执行 ·
            configkit 配置段加载 · testutil 隔离测试库基座
modules/    registry items：每模块是**真实 Go module**（自带 go.mod、
            genproto 与 buf 配置，在本仓内可编译、可测试、CI 覆盖）；
            lynx-start add 安装时重定位进应用 modules/<m>/ 专用区（尾段拍平，
            ADR-0006）并改写导入路径（shadcn alias rewrite 的对应物），
            排除 go.mod/genproto 等工程文件
cli/        lynx-start 安装器（init / add / update / remove / generate / doctor / audit / list / registry；shadcn 语义：默认不覆盖、幂等；
            add 支持 name@version（git tag）、基线缓存写 ~/.lynx-start（按模块@commit 区分、可重建，
            应用只提交 lock+manifest），update 做三方合并）
registry/   registry.json 模块索引
scaffold/   lynx-start init 生成的应用骨架模板
docs/adr/   决策记录（0001 bun / 0002 已废弃 / 0003 三层供给 / 0004 六条契约）
docs/module-authoring.md   模块编写指南（在应用里新增业务域 / 发布到 registry）
GLOSSARY.md 统一语言
```

## 可安装模块

| 模块 | 版本 | 说明 |
|---|---|---|
| `demo` | 0.1.0 | 单实体 CRUD 样例，验证装配管线（不承载业务语义） |
| `identity` | 0.1.1 | 注册/登录（Argon2id 口令散列 + HS256 JWT，purpose 域隔离）/自助改密；向应用贡献 Bearer 认证器——安装后未声明 `anonymous` 的方法自动要求认证。事件目录：`identity.user.created` / `identity.user.password-changed`。安全边界：令牌无吊销（TTL 默认 30m，停用即时生效需每请求回读）；生产部署必须更换 `identity.secret`（Assemble 对开发默认密钥打警告） |
| `session` | 0.1.0 | **依赖 identity**（首个跨模块依赖，安装器校验 DAG）。把 identity 的无状态令牌升级为可吊销会话：CreateSession（principal 驱动）→ 访问(15m)+刷新(30d)令牌对；Refresh 轮换刷新令牌（旧 jti 重放即吊销整个会话）；Logout/LogoutAll；订阅 `identity.user.password-changed` 自动全局吊销。访问令牌与 identity 令牌在认证链共存 |
| `access` | 0.1.0 | **依赖 identity**。RBAC：角色目录在配置段声明（`access.roles`，可审计），分配事实落库（幂等 Grant/Revoke）；**Enricher 在认证后注入 roles/permissions claims**——任何模块的方法可在 proto 里声明 `permissions:[...]`（MATCH_ALL）或 `roles:[...]`（MATCH_ANY），与令牌来源无关；bootstrap 管理员邮箱启动期收敛。管理 API 需 `access.assignment.write` 权限 |
| `audit` | 0.1.0 | 审计：RPC 拦截器自动记录 + 显式记录（同事务）+ 分页查询（append-only，零模块依赖） |
| `organization` | 0.1.1 | **依赖 identity**。组织与成员事实（首个账号域之外的业务模块）：组织 = 结构与成员，不自带 RBAC——成员带 `is_owner` 结构标记（创建者即 owner、owner-only 管理、最后一个 owner 不可离开），角色语义留给 access 表达；slug 全局唯一且创建后不可变；AddMember 经 identity 只读端口校验用户存在（幂等）。事件目录：`organization.organization.created` / `organization.member.added` / `organization.member.removed`（0.1.1 起 member.* 载荷含 `organization_name`） |
| `notification` | 0.1.0 | **依赖 identity+organization**（纯事件目录消费，不 import 其 Go 代码）。站内通知：订阅 `identity.user.password-changed` 与 `organization.member.added` / `organization.member.removed` 自动生成通知——内容在发生时刻定格为快照（title/body 落库，payload 保留原始载荷供前端跳转）；自助 List（分页+未读过滤+未读计数）/ MarkRead / MarkAllRead（通知只可见于收件人本人）。写入仅经事件订阅，无发送 RPC（跨模块调用面另立决策） |
| `task` | 0.1.0 | **依赖 organization**（成员资格经其 domain 只读端口校验，ADR-0007 通道②）。组织内任务——**第一个组织挂靠业务资源（ADR-0008 姿势首秀）**：`organization_id` 是租户键（Get/List/Delete 端口签名携带组织，ID 泄露也跨组织取不到）；成员即可创建/查看，完成与删除限创建者（`creator_id` 是 v1 资源所有权裁决依据）；事件目录：`task.task.created` / `task.task.completed` |

> 模块必须是真实可编译代码：kernel 的任何破坏性变更都会在本仓
> `go build ./...` 立即暴露，而不是等到某个应用安装时才炸。这是
> shadcn（组件在自家 monorepo 里被构建与类型检查）的对应纪律。

## 模块六条契约（ADR-0004）

1. 分层归属：`domain/<module>`（对外契约面）+ `internal/{app,api,infra}/<module>` + `proto/<module>/v1`（模块仓侧；装入应用后重定位进 `modules/<m>/` 并拍平尾段，见 ADR-0006），生成物本地 buf 生成
2. 模块间仅 domain 级 DAG 依赖（编译器强制——domain 在 internal 之外，app/infra/api 互引直接编不过）；跨域走 Kernel 端口 + app 适配器 + 事件字符串目录
3. 配置自治：模块自带 Config struct（`UnmarshalKey` 自有段），无全局 config.proto
4. 迁移隔离：每模块独立 goose 版本表（`goose_<module>_db`），禁跨模块外键
5. 错误码区段：`errcode.MustRegister("<module>")`，区段内自定义
6. Kernel 无租户预设：多租户是未来模块的横切实现

## 装配协议

模块暴露 `Module()`（静态：策略描述符/迁移/依赖声明）与 `Assemble(kernel.Deps)
(kernel.Assembly, error)`（动态：Registrations/Services/Tasks/Authenticators/
Stages/ExtraRoutes）。应用侧 `modules/modules.go` 的 `Installed` 列表是
唯一登记点，`main.go` 聚合后交给 lynx + grpcapi，启动期 fail-closed 断言防漂移。

## 非目标（Non-goals）

以下是**承诺不做**（不是还没做）——用于 5 分钟内自我筛选：

- **单进程 modular monolith**：跨进程事件投递、outbox、分布式集成不在覆盖面（进程内事件总线 + 启动期目录校验是当前边界）；
- **Postgres / bun only**：无第二存储方言（Deps.DB 是 *bun.DB）；
- **HTTP 面圈定在 proto 注解 + ExtraRoute**：流式、WebSocket、文件上传不在 registry 模块覆盖面（应用侧自担）;
- **不抄 shadcn HTTP registry 协议**：registry 走 git 仓库 + tag，对私有部署更友好；
- **多租户不进 kernel**（契约⑥）：是未来横切模块的事。

## Roadmap

- 下一批业务模块（SaaS 域设计素材已备，见早期讨论与 ADR-0002 存档）

---

# lynx-start (English)

**shadcn-style module infrastructure for the lynx ecosystem**: a Kernel contract
library + a source-code module registry + an installer CLI. Business domains are
installed as source code into any lynx-based Go backend — the code is yours
once installed. Edit it freely.

## Positioning: three supply layers (ADR-0003)

| Layer | Form | Stability promise |
|---|---|---|
| Framework | lynx / grpcapi / bun / goose (libraries) | follows upstream |
| **Kernel** (`kernel/`, standalone Go module) | assembly protocol, principal context, error-code spaces, crud pagination dialect, uow transaction port, event envelope, goosekit migration isolation | **semver** |
| **Source modules** (`modules/`, registry items) | a four-layer implementation of one business domain (domain/app/api/infra + proto + migrations + config section) | none — the source is yours |

"Stable" is carried by the Kernel (compile-time typed contracts), "extensible"
by source modules (you edit your own code, not plugin hooks), and "evolvable"
by the registry (git-tagged versions + three-way-merge updates; security-sensitive
implementations sink into the Kernel and ship fixes via semver).
shadcn/ui is the reference: Radix is a library, components are source,
the registry is the distribution protocol.

## Quick start

```bash
# Install the CLI (Go 1.24+; or build manually: cd cli && go build -o /usr/local/bin/lynx-start ./cmd/lynx-start)
go install github.com/lynx-go/lynx-start/cli/cmd/lynx-start@latest

# Registry comes in two forms: a local checkout, or a git URL (auto-cloned to ~/.cache with incremental refresh)
export LYNXSTART_REGISTRY=https://github.com/lynx-go/lynx-start.git
# Generate the app skeleton and install modules
lynx-start init myapp --module github.com/acme/myapp --registry <this-repo>
cd myapp
lynx-start add demo --registry <this-repo>      # sample: verifies the assembly pipeline
lynx-start init myapp-saas --preset saas --registry <this-repo>  # or: one command installs the SaaS base (identity+session+access+audit+organization+notification)
lynx-start add identity@0.1.1 --registry <this-repo>  # optional: pin a version (git tag)
lynx-start add session --registry <this-repo>    # revocable session token pair (depends on identity, validated by the installer)
lynx-start add access --registry <this-repo>     # authorization: RBAC + claims injection (depends on identity)
lynx-start add audit --registry <this-repo>      # audit: RPC interceptor auto-recording + explicit recording + queries (zero deps)
lynx-start add organization --registry <this-repo>  # organization: org & membership facts (creator is owner; depends on identity)

# When a registry module ships a new version (three-way merge: local / install baseline / upstream)
git add -A && git commit -m wip    # update writes are not rollback-safe — commit first
lynx-start update identity --registry <this-repo>

# Health checks (modules.go/config/baseline/dependency consistency; security advisories as warnings) and uninstall
lynx-start doctor --registry <this-repo>
lynx-start audit --registry <this-repo>     # advisory gate: non-zero exit on known-advisory hits (for CI)
lynx-start remove audit --registry <this-repo>   # reverse-DAG validation; migration tables are kept (data safety)

# Run it (order matters: go mod tidy fails before genproto exists; mise pins go/buf)
mise install && mise trust
mise run generate               # buf dep update + lint + generate → genproto/
go mod tidy                     # kernel (semver) and module deps are pulled here
mise run db:up && mise run db:migrate
mise run dev                    # gRPC :9060 / HTTP :8080
```

To write your own business-domain module see
[docs/module-authoring.en.md](docs/module-authoring.en.md)
(中文版: [docs/module-authoring.md](docs/module-authoring.md)).

## Repository layout

```
kernel/     contract library (standalone Go module, semver) — v0.2.0 package set:
            module/assembly protocol & DAG validation · errcode spaces ·
            contexts principal context · events envelope & publisher port ·
            uow+buntx transaction propagation · crud pagination dialect
            (HMAC token + cursor) · goosekit per-module migration isolation ·
            secretkey master-key purpose derivation · clock (fake-injectable) ·
            idgen random/prefixed IDs · password argon2id hashing ·
            jwt HS256 issue/verify (purpose isolation) ·
            bunx connection bootstrap + AIP-132 whitelisted ordering + generic list ·
            configkit config-section loading · testutil isolated test-DB harness
modules/    registry items: each module is a REAL Go module (own go.mod,
            genproto and buf config — compiled and tested in this repo, CI-covered);
            lynx-start add installs into the app's modules/<m>/ zone (tail segment
            flattened, ADR-0006) and rewrites import paths (the counterpart of
            shadcn's alias rewrite), excluding engineering files
cli/        lynx-start installer (init / add / update / remove / generate / doctor / list;
            shadcn semantics: never overwrite by default, idempotent;
            add supports name@version (git tag) and writes an install baseline,
            update performs a three-way merge)
registry/   registry.json module index
scaffold/   the app skeleton template generated by lynx-start init
docs/adr/   decision records (0001 bun / 0002 archived / 0003 three-layer supply / 0004 six contracts)
docs/module-authoring.en.md   module authoring guide (add a business domain to your app / publish to the registry)
GLOSSARY.md  ubiquitous language
```

## Installable modules

| Module | Version | Notes |
|---|---|---|
| `demo` | 0.1.0 | Single-entity CRUD sample that verifies the assembly pipeline (no business semantics) |
| `identity` | 0.1.1 | Register/login (Argon2id password hashing + HS256 JWT, purpose isolation) / self-service password change; contributes a Bearer authenticator to the app — after installation, methods not declared `anonymous` require authentication. Event catalog: `identity.user.created` / `identity.user.password-changed`. Security boundary: tokens are not revocable (default TTL 30m; per-request DB re-read required for instant disable); production deployments MUST change `identity.secret` (Assemble warns about the dev default key) |
| `session` | 0.1.0 | **Depends on identity** (first cross-module dependency, DAG-validated by the installer). Upgrades identity's stateless tokens to revocable sessions: CreateSession (principal-driven) → access(15m)+refresh(30d) token pair; Refresh rotates the refresh token (replaying an old jti revokes the whole session); Logout/LogoutAll; subscribes to `identity.user.password-changed` for automatic global revocation. Access tokens and identity tokens coexist in the auth chain |
| `access` | 0.1.0 | **Depends on identity**. RBAC: the role catalog is declared in the config section (`access.roles`, auditable), assignment facts live in the DB (idempotent Grant/Revoke); **an Enricher injects roles/permissions claims after authentication** — any module's method may declare `permissions:[...]` (MATCH_ALL) or `roles:[...]` (MATCH_ANY) in proto, regardless of token origin; bootstrap admin emails converge at startup. Managing the API requires the `access.assignment.write` permission |
| `audit` | 0.1.0 | Audit: RPC interceptor auto-recording + explicit recording (same transaction) + paginated query (append-only, zero module dependencies) |
| `organization` | 0.1.1 | **Depends on identity**. Organizations and membership facts (the first module beyond the account domain): structure and membership, not RBAC — members carry an `is_owner` structural flag (creator is owner, owner-only management, the last owner cannot leave); role semantics are left to access. Slug is globally unique and immutable after creation; AddMember verifies user existence via identity's read-only port (idempotent). Event catalog: `organization.organization.created` / `organization.member.added` / `organization.member.removed` (since 0.1.1 the member.* payload carries `organization_name`) |
| `notification` | 0.1.0 | **Depends on identity+organization** (pure event-catalog consumption — never imports their Go code). In-app notifications: subscribes to `identity.user.password-changed` and `organization.member.added` / `organization.member.removed` and turns them into notifications — content is snapshotted at occurrence (title/body stored, payload kept for frontend navigation); self-service List (pagination + unread filter + unread count) / MarkRead / MarkAllRead (notifications visible only to their recipient). Writes happen exclusively via event subscription — no send RPC (cross-module call surface is a separate decision) |
| `task` | 0.1.0 | **Depends on organization** (membership checked via its domain read-only port, ADR-0007 channel 2). Organization-scoped tasks — **the first organization-scoped business resource (ADR-0008 in practice)**: `organization_id` is the tenant key (Get/List/Delete carry the organization in the port signature — a leaked ID still cannot cross organizations); any member may create/list, while completing and deleting are creator-only (`creator_id` is the v1 resource-ownership rule). Event catalog: `task.task.created` / `task.task.completed` |

> Modules must be real, compilable code: any kernel breaking change surfaces in
> this repo's `go build ./...` immediately — not when some app installs the
> module. This is the counterpart of shadcn's discipline (components are built
> and type-checked inside their own monorepo).

## The six module contracts (ADR-0004)

1. Layer ownership: `domain/<module>` (the outward contract surface) + `internal/{app,api,infra}/<module>` + `proto/<module>/v1` (module-repo side; installed into the app under `modules/<m>/` with the tail segment flattened — see ADR-0006), generated code built locally with buf
2. Cross-module dependencies are domain-level DAG only (compiler-enforced — domain lives outside internal, so app/infra/api cross-imports simply do not compile); cross-domain integration goes through Kernel ports + app adapters + the string event catalog
3. Config autonomy: each module carries its own Config struct (`UnmarshalKey` on its own section), no global config.proto
4. Migration isolation: per-module goose version table (`goose_<module>_db`), no cross-module foreign keys
5. Error-code spaces: `errcode.MustRegister("<module>")`, custom codes within the space
6. No tenancy preset in the Kernel: multi-tenancy is a future cross-cutting module

## Assembly protocol

A module exposes `Module()` (static: policy descriptors / migrations / dependency
declarations) and `Assemble(kernel.Deps) (kernel.Assembly, error)` (dynamic:
Registrations/Services/Tasks/Authenticators/Enrichers/Stages/ExtraRoutes). The
`Installed` list in the app's `modules/modules.go` is the single
registration point; `main.go` aggregates and hands everything to lynx + grpcapi,
with fail-closed startup assertions against drift.

## Non-goals

These are commitments, not a backlog — use them to self-filter in five minutes:

- **Single-process modular monolith**: no cross-process event delivery, no outbox, no distributed integration (the in-process event bus + startup catalog validation is the current boundary);
- **Postgres / bun only**: no second storage dialect (`Deps.DB` is `*bun.DB`);
- **The HTTP surface is confined to proto annotations + ExtraRoute**: streaming, WebSocket and file upload are not covered by registry modules (bring your own);
- **No shadcn HTTP registry protocol**: the registry is a git repo + tags — friendlier to private deployments;
- **No tenancy in the Kernel** (contract 6): it belongs to a future cross-cutting module.

## Roadmap

- Next batch of business modules (SaaS-domain design material is ready; see early discussions and the ADR-0002 archive)
