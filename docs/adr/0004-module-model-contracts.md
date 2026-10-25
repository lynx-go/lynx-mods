# 模块模型六条契约

模块是源码分发单元，自由修改的前提是契约清晰。六条契约由安装器校验、护栏测试强制、ADR-0003 的三层供给结构承载：

1. **分层归属**：模块代码落 `domain/<module>` 与 `internal/{app,api,infra}/<module>` 与 `proto/<module>/v1`（模块仓侧形态；装入应用后经安装映射重定位进 `modules/<m>/` 并拍平尾段，见 ADR-0006），生成物在应用侧用钉版 buf 本地生成，不预生成入库。domain 位于 internal 之外：它是模块的对外契约面（聚合/端口/哨兵错误），"跨模块只能依赖 domain"由 Go 的 internal 可见性规则**在编译器层面强制**（2026-10-24 session 模块引入跨模块依赖时发现 internal 布局使契约②不可编译，遂修订）。
2. **模块间依赖**：仅允许 domain 级依赖（声明于 item 的 moduleDependencies），整体构成 DAG，安装器校验无环；app/infra/api 层禁止互引（编译器强制 + 护栏测试双保险）。跨域协作走 Kernel 端口 + app 层适配器，或事件目录（字符串事件名，禁止 import 他模块 domain 拿事件名——torchwood 事件目录反环规则的移植）。
3. **配置自治**：模块自带 Config struct，在自己的 Init 里 `UnmarshalKey("<module>", &cfg, WithErrorUnused())`（lynx v1.20 contrib 已验证的模式）；不设全局 config.proto 协调点。
4. **迁移隔离**：每模块独立迁移目录与独立 goose 版本表（`goose_<module>_db`），链间永不重排号；禁止跨模块外键，跨模块引用只存 ID。
5. **错误码区段**：Kernel 按模块分配错误码区间，模块在区间内自定义；app 层直接使用 grpc/status（torchwood P3-18 取舍沿用）。
6. **无租户预设**：Kernel 的 Principal/语境不带租户假设；多租户将来作为一个模块经 claim + 拦截器实现横切扩展，不进地基。

## Consequences

- `lyx add` 的安装清单必须与 Item 声明一致，`lyx doctor` 校验 import guard、策略断言（grpcapi 启动 fail-closed 已内置）、迁移链状态。
- 模块源码模板即文档：样例模块 demo 专职验证管线，不承载业务语义。
