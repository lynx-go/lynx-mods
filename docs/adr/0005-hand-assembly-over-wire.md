# 装配保持手写(wire 评估结论)

2026-10-25 评估「充分利用 wire DI 特性」后的决策:**维持手写 `Assemble(Deps)` 装配协议,不引入生成式 DI;借 wire 的思想,不借工具**。

## 背景

- 装配协议现状(ADR-0003/0004):模块暴露 `Module()`(静态)+ `Assemble(kernel.Deps) (kernel.Assembly, error)`(动态),依赖显式构造,「不引入生成式 DI 的重装步骤」自始写进 kernel 文档。
- wire 的映射其实很齐:Assemble 就是手写 injector;provider set ≈ 模块构造函数组;`wire.Bind` ≈ domain 端口绑定;类型化 Config provider ≈ `configkit.LoadSection`;**应用侧 set 覆盖模块 set** 是对 shadcn 模型最有价值的能力——不改模块源码、编译期类型安全地替换模块内部实现(如换仓储)。
- **决定性事实:google/wire 于 2025-08-25 归档**(官方"feature-complete 非废弃"——不再修 bug、不跟进新 Go 版本)。生成的 wire_gen.go 是普通 Go 可长期存活,但 wire 二进制需在未来的 Go 版本上解析源码,这不是冻结产出能兜住的(钉死工具链可缓解,等于永久背一个冻结依赖)。Go 社区编译期 DI 品类在 wire 之后无维护中的继任者,手写构造重新成为社区共识。

## Considered Options

- **A. 维持手写,借思想(选定)**:零维护风险;wire 最值钱的「声明式覆盖」与「源码归你、直接改」本质是同一件事的两种表达,后者不需要框架。
- **B. kernel 级 opt-in 接入 wire**:工程上可行(item.json 声明 assembly:wire、skipModuleFile 排 wire_gen.go、generate 步骤扩展),但为归档工具引入永久冻结依赖,且当前每模块装配仅 3-5 行构造,ROI 不成立。若未来出现真实痛点(单模块装配 >10 构造、用户大量定制仓储),再评估——且应先评估维护中的替代而非 wire。
- **C. 换运行时 DI(fx/samber-do)**:反射装配与「编译期显式」哲学冲突更大,弃。

## Decision(路线 A 的三件落地)

1. **testutil 共享 fake 构造集**(wire「测试 fake set」思想的去框架版):`kernel/testutil` 提供 `CapturePublisher`(与 BusPublisher 同语义——信封不合法即拒绝,曾经的手抄 fake 漏 Validate 是真实踩坑)与 `InlineRunner`(uow.Runner 直通),模块测试不再各自手抄;
2. **Deps 只在 api 装配层的 guard**(「Deps 子集化」思想的去框架版):kernel 根包(Deps 所在)只允许 `internal/api/<m>` 装配层 import,domain/app/infra 一律走子包端口(uow/events/crud/…)——静态护栏测试强制,替代框架的依赖图约束;
3. **定制指南**:module-authoring 增加「替换模块内部实现的标准姿势」——直接改是正道(源码归你),但改哪里、怎么避开 update 冲突热点,需要成文。

## Consequences

- kernel 装配协议与文档措辞不变;生成器中立——用 wire 或手写的应用都能长在 kernel 契约上(torchwood 自身用 wire,不受影响)。
- wire_gen.go 不进 skipModuleFile 白名单(无人采用 opt-in 路线);若未来重启 B,须连带处理生成物合并语义与工具链钉版成本。
- 升级 M4 时 codemod 通道同样受益于「装配层薄」:手写装配的修改点集中,AST 重写面小。
