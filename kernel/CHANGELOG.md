# Changelog — kernel

Kernel 对外做 semver 承诺:破坏性变更升主版本(0.x 阶段升 minor),向后兼容追加升 patch。
条目按发布时间倒序;根 CHANGELOG.md 含全仓(含模块)视角。

## v0.2.0 (2026-10-25)

- **事件目录契约(additive)**:`Module` 新增 `PublishedEvents` / `ConsumedEvents`;
  `ValidateModules` 启动期交叉校验——发布名合法且首段归属本模块、无重复发布、
  消费侧每个事件都有已装发布者(typo / 上游改名 / 漏装发布方 fail-closed)。
- **`password.DummyHash()`**:惰性生成与当前参数同参的 Argon2id 哑靶,供登录
  时序对齐使用——kernel 升参数后哑靶恒同参,修复模块内写死常量导致的
  时序对齐静默失效。
- demo/identity 模块随本版改用事件常量与 DummyHash(identity v0.1.1 起要求
  `kernel >= v0.2.0`)。

## v0.1.0 (2026-10-25)

初始发布——v0.1.0 候选包全量:

- `module` / 根包:装配协议(`Module` 静态描述 + `Assemble(Deps) (Assembly, error)`
  动态装配 + `AssembleAll` 聚合)、依赖 DAG(Kahn)与事件目录启动期校验、
  `ValidateName`;
- `errcode`:按模块登记的错误码区段与 ErrorInfo 形态;
- `contexts`:Principal 语境;
- `events`:事件信封(Envelope)、发布端口(Publisher)、名字约束(中划线段式);
- `uow` + `uow/buntx`:事务传播端口与 bun 适配(ctx 携带事务连接);
- `crud`:分页方言(HMAC token + offset/时间游标)、AIP-132 排序白名单;
- `goosekit`:per-module 迁移隔离(独立 goose 版本表 `goose_<module>_db`);
- `secretkey`:主密钥 purpose 派生;
- `clock`:时间源(Fake 注入);
- `idgen`:随机 ID / 前缀 ID;
- `password`:Argon2id(t=3 / m=64MiB / p=4);
- `jwt`:HS256 签发/验证(purpose 隔离);
- `bunx`:连接引导 + 泛型列表执行;
- `configkit`:配置段加载(`LoadSection`);
- `testutil`:隔离测试库基座(建库 → 迁移链 → FORCE DROP,DB 门控跳过)。
