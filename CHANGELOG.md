# Changelog — lynx-start

两类版本:kernel(`kernel/vX.Y.Z`,semver)与 registry 模块(`modules/<name>/vX.Y.Z`,
源码分发,无稳定性承诺)。CLI / scaffold 变更随仓库 main 演进,不单独打 tag,
记在 Unreleased。推 tag 后 GitHub Actions 自动建 Release 并摘取本文对应小节。

## Unreleased

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

## cli v0.1.1 (2026-10-25)

- main 包移至 `cmd/lynx-start/`——`go install` 装出的二进制名随包目录,
  v0.1.0 会装成 `cli`;修复后 `go install
  github.com/lynx-go/lynx-start/cli/cmd/lynx-start@latest` 得到 `lynx-start`。

## cli v0.1.0 (2026-10-25)

- 首个独立版本 tag(此前随仓库 main 演进):init / add(含 name@version 钉版、
  安装基线)/ update(三方合并)/ remove(DAG 反向校验)/ doctor / list;
  registry 支持本仓库目录与 git URL(clone 缓存 + 增量刷新)。

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
