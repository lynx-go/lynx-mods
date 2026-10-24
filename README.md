# lynx-start

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
代码，不是插件钩子），「演化」由 registry 承担（git tag 版本 + diff 更新）。
shadcn/ui 是参照系：Radix 是库、组件是源码、registry 是分发协议。

## 快速开始

```bash
# 构建 CLI
cd cli && go build -o /usr/local/bin/lyx .

# 生成应用骨架 + 安装模块
lyx init myapp --module github.com/acme/myapp --registry <本仓库目录>
cd myapp
lyx add demo --registry <本仓库目录>

# 跑起来（mise：go/buf 钉版）
mise install && mise trust
go mod init github.com/acme/myapp   # 或按提示
buf dep update && buf generate      # proto → genproto/
mise run db:up && mise run db:migrate
mise run dev                        # gRPC :9060 / HTTP :8080
```

## 仓库结构

```
kernel/     契约库（module.go 装配协议 / errcode / contexts / events /
            uow+buntx / crud / goosekit / testutil）
modules/    registry items：每模块是**真实 Go module**（自带 go.mod、
            genproto 与 buf 配置，在本仓内可编译、可测试、CI 覆盖）；
            lyx add 安装时改写导入路径为应用 module path（shadcn alias
            rewrite 的对应物），并排除 go.mod/genproto 等工程文件
cli/        lyx 安装器（init / add / list；shadcn 语义：默认不覆盖、幂等）
registry/   registry.json 模块索引
scaffold/   lyx init 生成的应用骨架模板
docs/adr/   决策记录（0001 bun / 0002 已废弃 / 0003 三层供给 / 0004 六条契约）
GLOSSARY.md 统一语言
```

> 模块必须是真实可编译代码：kernel 的任何破坏性变更都会在本仓
> `go build ./...` 立即暴露，而不是等到某个应用安装时才炸。这是
> shadcn（组件在自家 monorepo 里被构建与类型检查）的对应纪律。

## 模块六条契约（ADR-0004）

1. 分层归属：`internal/{domain,app,api,infra}/<module>` + `proto/<module>/v1`，生成物本地 buf 生成
2. 模块间仅 domain 级 DAG 依赖；跨域走 Kernel 端口 + app 适配器 + 事件字符串目录
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

- `lyx update`（diff 语义）/ `lyx doctor`（契约校验：import guard / 迁移状态 / DAG）
- 远端 registry（git tag 版本 pin）
- 首批真实业务模块（identity/access/audit…；SaaS 域设计素材已备，见早期讨论与 ADR-0002 存档）

---

# lynx-start (English)

A **shadcn-style module infrastructure for the lynx ecosystem**: a Kernel contract
library, a source-code module registry, and an installer CLI. Business domains are
installed as source code you own and edit freely. See the Chinese section above for
architecture, quick start, contracts, and roadmap; canonical docs live in
`docs/adr/` and `GLOSSARY.md`.
