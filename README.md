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
# 构建 CLI
cd cli && go build -o /usr/local/bin/lynx-start .

# 生成应用骨架 + 安装模块
lynx-start init myapp --module github.com/acme/myapp --registry <本仓库目录>
cd myapp
lynx-start add demo --registry <本仓库目录>      # 样例：验证装配管线
lynx-start add identity@0.1.1 --registry <本仓库目录>  # 可选：钉版本安装（git tag）
lynx-start add session --registry <本仓库目录>    # 会话：可吊销令牌对（依赖 identity，安装器校验）
lynx-start add access --registry <本仓库目录>     # 授权：RBAC + claims 注入（依赖 identity）
lynx-start add audit --registry <本仓库目录>      # 审计：RPC 拦截器自动记录 + 显式记录 + 查询（零依赖）

# registry 模块出新版时（三方合并：本地 / 安装基线 / upstream）
git add -A && git commit -m wip    # update 写盘不可回滚，先 commit
lynx-start update identity --registry <本仓库目录>

# 体检（modules.go/config/基线/依赖一致性）与卸载
lynx-start doctor --registry <本仓库目录>
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
            lynx-start add 安装时改写导入路径为应用 module path（shadcn alias
            rewrite 的对应物），并排除 go.mod/genproto 等工程文件
cli/        lynx-start 安装器（init / add / update / list；shadcn 语义：默认不覆盖、幂等；
            add 支持 name@version（git tag）、写安装基线，update 做三方合并）
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
| `identity` | 0.1.0 | 注册/登录（Argon2id 口令散列 + HS256 JWT，purpose 域隔离）/自助改密；向应用贡献 Bearer 认证器——安装后未声明 `anonymous` 的方法自动要求认证。事件目录：`identity.user.created` / `identity.user.password-changed`。安全边界：令牌无吊销（TTL 默认 30m，停用即时生效需每请求回读）；生产部署必须更换 `identity.secret`（Assemble 对开发默认密钥打警告） |
| `session` | 0.1.0 | **依赖 identity**（首个跨模块依赖，安装器校验 DAG）。把 identity 的无状态令牌升级为可吊销会话：CreateSession（principal 驱动）→ 访问(15m)+刷新(30d)令牌对；Refresh 轮换刷新令牌（旧 jti 重放即吊销整个会话）；Logout/LogoutAll；订阅 `identity.user.password-changed` 自动全局吊销。访问令牌与 identity 令牌在认证链共存 |
| `access` | 0.1.0 | **依赖 identity**。RBAC：角色目录在配置段声明（`access.roles`，可审计），分配事实落库（幂等 Grant/Revoke）；**Enricher 在认证后注入 roles/permissions claims**——任何模块的方法可在 proto 里声明 `permissions:[...]`（MATCH_ALL）或 `roles:[...]`（MATCH_ANY），与令牌来源无关；bootstrap 管理员邮箱启动期收敛。管理 API 需 `access.assignment.write` 权限 |

> 模块必须是真实可编译代码：kernel 的任何破坏性变更都会在本仓
> `go build ./...` 立即暴露，而不是等到某个应用安装时才炸。这是
> shadcn（组件在自家 monorepo 里被构建与类型检查）的对应纪律。

## 模块六条契约（ADR-0004）

1. 分层归属：`domain/<module>`（对外契约面）+ `internal/{app,api,infra}/<module>` + `proto/<module>/v1`，生成物本地 buf 生成
2. 模块间仅 domain 级 DAG 依赖（编译器强制——domain 在 internal 之外，app/infra/api 互引直接编不过）；跨域走 Kernel 端口 + app 适配器 + 事件字符串目录
3. 配置自治：模块自带 Config struct（`UnmarshalKey` 自有段），无全局 config.proto
4. 迁移隔离：每模块独立 goose 版本表（`goose_<module>_db`），禁跨模块外键
5. 错误码区段：`errcode.MustRegister("<module>")`，区段内自定义
6. Kernel 无租户预设：多租户是未来模块的横切实现

## 装配协议

模块暴露 `Module()`（静态：策略描述符/迁移/依赖声明）与 `Assemble(kernel.Deps)
(kernel.Assembly, error)`（动态：Registrations/Services/Tasks/Authenticators/
Stages/ExtraRoutes）。应用侧 `internal/modules/modules.go` 的 `Installed` 列表是
唯一登记点，`main.go` 聚合后交给 lynx + grpcapi，启动期 fail-closed 断言防漂移。

## Roadmap

- 远端 registry（本地 git tag 版本 pin 已支持：`add <m>@<ver>` / `update`；远端仓库解析后续接入）
- 下一批业务模块（SaaS 域设计素材已备，见早期讨论与 ADR-0002 存档）

---

# lynx-start (English)

A **shadcn-style module infrastructure for the lynx ecosystem**: a Kernel contract
library, a source-code module registry, and an installer CLI. Business domains are
installed as source code you own and edit freely. See the Chinese section above for
architecture, quick start, contracts, and roadmap; canonical docs live in
`docs/adr/` and `GLOSSARY.md`.
