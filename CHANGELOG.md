# Changelog — lynx-start

两类版本:kernel(`kernel/vX.Y.Z`,semver)与 registry 模块(`modules/<name>/vX.Y.Z`,
源码分发,无稳定性承诺)。CLI / scaffold 变更随仓库 main 演进,不单独打 tag,
记在 Unreleased。推 tag 后 GitHub Actions 自动建 Release 并摘取本文对应小节。

## Unreleased

- **breaking：安装布局镜像挂载 + 基线缓存分离（ADR-0006，v0.x 阶段直接 breaking）**：
  - 模块装入应用 `modules/<m>/` 专用区（`modules/` = registry 镜像区，
    `internal/` = 应用自有代码），`{domain,internal/{app,api,infra}}` 下尾段
    模块名目录拍平；模块仓内源码结构零改动。Go internal 可见性下应用经
    CLI 生成的桥接包 `modules/<m>/module.go` 取得 Module/Assemble；
    modules.go 登记点移至 `modules/modules.go`。proto 契约源仍集中应用根。
  - 新命令 `lynx-start generate`：buf 管线（dep update/lint/generate），
    布局规则单一事实源收敛 CLI（不靠安装拷贝也不靠 mise shell 后处理）；
    scaffold 的 mise generate 任务变为一行转发。
  - 基线缓存迁至 `~/.lynx-start/baselines/<appModulePath>/<m>@<commit>/`
    （多版本并存、可重建、不进仓库）：应用只提交 lock.json+manifest；
    update/doctor 在缓存缺失时按 lock 记录的 commit 从 registry 自动重建
    （ensureBaseline，跨设备 rename 回退复制）；update 冲突备份移至
    `~/.lynx-start/backups/<appModulePath>/<m>/`；remove 整树清理
    `modules/<m>/`（含桥接与 genproto）与用户缓存。
  - 导入改写五条结构化规则（genproto/domain/三层 internal，逐依赖映射）+
    改写后残留扫描 fail-closed；domain guard_test 的相对路径深度随拍平
    修正（安装期确定性改写）。doctor 增旧布局检测（可行动重装指引）。
  - e2e 全量组合在新布局下全绿（含 update 三方合并/remove/doctor）；
    旧布局应用（如 lynx-start-demo）需按指引重装迁移。
- **registry 第 6 个模块 `organization` v0.1.0(组织与成员事实,首个账号域之外的业务模块;命名纪律:除局部短变量外一律全称,模块名/目录/文件名同为全称)**:
  - 定位:组织 = 结构与成员事实,不自带 RBAC——成员带 `IsOwner` 结构标记
    (创建者即 owner,owner-only 管理;最后一个 owner 不可离开,owner 不可被
    他人移除),角色/权限语义留给 access 的角色目录表达;slug 全局唯一
    (唯一索引,冲突映射 `ErrSlugTaken`),创建后不可变。
  - API 面(全部 authenticated,v1 不做组织级 RBAC——成员资格/owner 校验在
    app 层):CreateOrganization/GetOrganization(仅成员可见)/
    ListMyOrganizations/RenameOrganization/AddMember(幂等,经 identity
    只读端口校验用户存在)/RemoveMember(幂等,不可移 owner)/ListMembers/
    Leave(最后一个 owner 拒绝)。
  - 事件:`organization.organization.created` / `organization.member.added` /
    `organization.member.removed`;DependsOn identity(domain 级只读消费,
    access 同款 userReader 模式);v0.1 零配置段;迁移
    `organization_organizations` / `organization_members`(独立版本表,
    user_id 只存 ID 不设外键)。
  - 工程:item.json 声明 `moduleDependencies [{"identity": ">=0.1.1"}]` +
    `kernelConstraint ">=0.4.0"` + `goDependencies kernel@v0.4.0`;guard 两件
    (domain 纯度 + AssertDepsAtAPILayer)、policy_test(全方法非匿名)、
    DB 门控测试真库验证;CI e2e matrix 增 `identity organization` 边与
    全量组合;saas 预设纳入 organization(底座从纯账号域扩展到组织结构)。
- **M3 完结(2026-10-25 第四批)**:
  - I-14 供应链锁 `.lynx-start/lock.json`:每模块记解析版本、来源
    (tag/worktree)与 commit、基线文件 sha256。add/update 写入;update
    合并前复核基线哈希——被篡改的基线会静默污染三方合并,现在 fail-closed
    并给出重建指引;doctor 复核条目在、哈希符、钉版 tag 未移动;remove
    清理。实测:篡改基线后 update 被拒,doctor 全绿态零误报。
  - I-17 preset:`registry/presets.json` + `lynx-start init --preset saas`
    一次装齐组合(组合顺序即安装顺序,依赖由安装器校验);`list` 展示
    presets;首个预设 saas(identity+session+access+audit)四模块全链路实测。
  - 修复:doctor 改按已装版本解析 item(tag 冻结形态——工作区 item 声明的
    新版 env/约束拿来校验旧安装会误报);tag 漂移检查只对 tag 安装条目
    (worktree 记 HEAD,与旧 tag 比对无意义)。
- **M3 及格线(2026-10-25 第三批)**:
  - I-13 安全公告通道:`registry/advisories.json`(affectedVersions 精确
    匹配,不做范围解析)+ 新命令 `lynx-start audit`(命中即非零退出,
    CI 门禁;--format github 出注解);doctor 集成为 ⚠ 提示(不计数不过载
    体检语义);已回填首条真实公告 LYNXSTART-0001(identity 0.1.0 时序哑靶)。
  - I-16 版本范围约束:item.json `moduleDependencies` 支持
    `[{"identity": ">=0.1.1"}]` 对象形态(旧字符串形态兼容),add 安装期
    校验已装版本(启动期 fail-closed 前移到安装期);新增 `kernelConstraint`
    对照应用 go.mod 的 kernel require(add 校验 + doctor 复核);session
    回填真实约束 identity>=0.1.1。
  - I-15 env 声明:item.json `env` 字段([{key,description,secret}]);
    add/update 自动写 `.env.example`(secret 生成 32 hex 随机占位,变量带
    应用前缀),doctor 校验声明在位,remove 清理对应段;identity/session
    已声明主密钥变量。
  - I-12 贡献路径去手工化:新命令 `lynx-start registry add <dir>`(item.json
    骨架 + registry.json 登记 + go.work use,一处输入三处一致,幂等)+
    `lynx-start registry check`(一致性体检,可入 CI);CONTRIBUTING.md。
- **M2 断点修复(2026-10-25 第二批)**:
  - I-07 OpenAPI 接线:scaffold 打开 `grpcapiserver.Config.OpenAPI`,应用
    免费获得 `GET /openapi.json`(能力在 grpcapi v0.4.0 的 openapi 包);
  - I-08 scaffold 换用 `kernel.DefaultChain`(v0.3.0)——复刻默认链的
    易错逻辑从生成物移入 semver 库;
  - I-09 `update --dry-run`:合并计划(自动合并/冲突/upstream 删除)完整
    呈现,零写盘;
  - I-10 CI 组合矩阵:单模块 / 依赖边 / 全量五组合,e2e 经
    `LYNX_E2E_MODULES` 参数化;
  - I-11 `doctor --format github`:失败项输出 workflow command 注解,
    通过项静默,退出码稳定(0/1);
  - init go.mod 写入依赖地板(kernel v0.3.0 / grpcapi v0.4.0)。
- **英文文档(I-01)**:README 完整英文段、module-authoring.en.md、scaffold
  README 双语、GLOSSARY 英文对照;
- **README:go install 与 Non-goals(I-02/I-03)**:快速开始首选
  `go install github.com/lynx-go/lynx-start/cli@latest`;新增「非目标」章节;
- **scaffold:启动期 HTTP 路由表(I-04)**:服务启动打印 方法/路径/RPC/认证
  要求,免翻 proto 找路径;
- **scaffold:数据库连接失败排障提示(I-05)**:端口被既有 Postgres 占用等常见
  形态,报错附可执行动作(bootstrap.HintDB,migrate 与 server 共用);
- **发布自动化(I-06)**:本 CHANGELOG + `.github/workflows/release.yml`
  (tag push 自动建 Release,notes 从本文摘取)。

## cli v0.2.1 (2026-10-25)

ADR-0006 修订：**genproto 回归应用根** `genproto/<m>/v1/`（v0.2.0 曾归位
`modules/<m>/genproto/`）。复盘：genproto 是改写约定里最稳定的一条特例（目录名死约定），
生成物无人手工 diff、镜像完整性价值近乎零；generate 后搬移是长期维护税（首版即出过
目标路径 bug）且偏离 buf 生态惯例。generate 现在是纯 buf 管线、零后处理；remove 顺手
清理 `genproto/<m>/`。三目录各司其职：`proto/` 契约源、`genproto/` 生成物、`modules/`
模块源码。**v0.2.0 装的应用需重装迁移**（add -o 重写 genproto import），lynx-start-demo 已同步。

## cli v0.2.0 (2026-10-25)

安装布局重大变更（ADR-0006，**breaking**——旧布局应用按 doctor 指引重装迁移）：

- 模块装入应用 `modules/<m>/` 专用区（`modules/` = registry 镜像区，`internal/` = 应用自有代码），`{domain,internal/{app,api,infra}}` 尾段模块名目录拍平，模块仓源码结构零改动；应用经 CLI 生成的桥接包 `modules/<m>/module.go` 取得 Module/Assemble；modules.go 登记点移至 `modules/modules.go`。
- 新命令 `lynx-start generate`：buf 管线（dep update / lint / generate）+ genproto 按模块归位 `modules/<m>/genproto/<m>/v1/`——布局规则单一事实源收敛 CLI；scaffold 的 mise generate 任务变为一行转发。
- 基线缓存迁 `~/.lynx-start/baselines/<appModulePath>/<m>@<commit>/`（多版本并存、可重建、不进应用仓库；应用只提交 lock.json + lynx-start.json）；update/doctor 在缓存缺失时按 lock 记录的 commit 从 registry 自动重建；update 冲突备份移 `~/.lynx-start/backups/…`；remove 整树清理 `modules/<m>/` 与用户缓存。
- 导入改写五条结构化规则 + 残留扫描 fail-closed；guard_test 相对路径随拍平在安装期修正；doctor 增旧布局检测。

## cli v0.1.1 (2026-10-25)

- main 包移至 `cmd/lynx-start/`——`go install` 装出的二进制名随包目录,
  v0.1.0 会装成 `cli`;修复后 `go install
  github.com/lynx-go/lynx-start/cli/cmd/lynx-start@latest` 得到 `lynx-start`。

## cli v0.1.0 (2026-10-25)

- 首个独立版本 tag(此前随仓库 main 演进):init / add(含 name@version 钉版、
  安装基线)/ update(三方合并)/ remove(DAG 反向校验)/ doctor / list;
  registry 支持本仓库目录与 git URL(clone 缓存 + 增量刷新)。

## kernel v0.4.0 (2026-10-25)

- testutil 共享 fake(CapturePublisher/InlineRunner)+ AssertDepsAtAPILayer
  依赖护栏(ADR-0005 路线 A:装配保持手写,wire 借思想不借工具);
  详见 [kernel/CHANGELOG.md](kernel/CHANGELOG.md)。

## kernel v0.3.0 (2026-10-25)

- `DefaultChain(logger, set, agg, authnOpts)`(additive):模块贡献 Stage 时
  必须复刻的 server 默认链(recovery/requestid/clientinfo/authn/authz/
  validate)收进 kernel——修复可达性,详见
  [kernel/CHANGELOG.md](kernel/CHANGELOG.md)。

## kernel v0.2.0 (2026-10-25)

- 事件目录契约:`Module.PublishedEvents` / `ConsumedEvents` + 启动期交叉校验
  (消费的每个事件必须有已装发布方);
- `password.DummyHash()`:登录时序对齐哑靶(参数恒同参);
- 详见 [kernel/CHANGELOG.md](kernel/CHANGELOG.md)。

## kernel v0.1.0 (2026-10-25)

- 初始发布:装配协议 / errcode / contexts / events / uow+buntx / crud /
  goosekit / secretkey / clock / idgen / password / jwt / bunx / configkit /
  testutil;
- 详见 [kernel/CHANGELOG.md](kernel/CHANGELOG.md)。

## identity v0.1.1 (2026-10-25)

- 安全修复:登录时序对齐哑靶改调 `kernel/password.DummyHash()`(删模块内写死
  常量;kernel 升参数后旧哑靶失配、时序对齐静默失效);
- goDependencies 提升至 `kernel >= v0.2.0`。

## identity v0.1.0 (2026-10-25)

- 注册 / 登录(Argon2id + HS256 JWT,purpose 隔离)/ 自助改密;
- 向应用贡献 Bearer 认证器:未声明 `anonymous` 的方法自动要求认证(fail-closed);
- 事件:`identity.user.created` / `identity.user.password-changed`;
- 安全边界:令牌无吊销(TTL 默认 30m);生产必须更换 `identity.secret`。

## session v0.1.0 (2026-10-25)

- 首个跨模块依赖(identity);可吊销会话:访问(15m)+刷新(30d)令牌对,
  Refresh 轮换刷新令牌(旧 jti 重放即吊销全会话),Logout/LogoutAll;
- 订阅 `identity.user.password-changed` 自动全局吊销。

## access v0.1.0 (2026-10-25)

- RBAC:角色目录配置声明(`access.roles`)+ 分配落库(幂等 Grant/Revoke);
- Enricher 认证后注入 roles/permissions claims——任何模块可在 proto 声明
  `permissions` / `roles`,与令牌来源无关;
- bootstrap 管理员邮箱启动期收敛。

## audit v0.1.0 (2026-10-25)

- 审计三路径:RPC 拦截器自动记录(Order=50 槽位)+ 显式记录(同事务)+
  分页查询(append-only,零模块依赖)。

## demo v0.1.0 (2026-10-25)

- 单实体 CRUD 样例,验证装配管线(不承载业务语义)。
