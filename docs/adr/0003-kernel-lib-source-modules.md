# Kernel 库 + 源码模块（shadcn 式供给）

lynx-start 的供给形态为三层：框架层（lynx/grpcapi/bun/goose/wire，本来就是库）、**Kernel 契约库**（独立 Go module `lynx-start/kernel`，semver 承诺）、**源码模块**（registry item，安装后源码归应用所有）。「稳定」由 Kernel 承担（编译期类型契约 + semver），「可扩展」由源码模块承担（改的是自己的代码，不是插件钩子），「演化」由 registry 承担（git tag 版本 + diff 更新）。应用侧组装收敛到 `modules/modules.go（原 internal/modules/modules.go，ADR-0006 移位）` 的 `Installed` 列表，模块对其余部分的全部贡献经装配协议（`kernel.Module`）声明。

本决策取代同日早先的「fork 优先 + 库内核」方案——那次讨论的对立（fork vs 库）在三层的供给结构里各归其位。shadcn/ui 是本形态的参照系（Radix 是库、组件是源码、registry 是分发协议）。

## Considered Options

- **三层供给（选定）**：Kernel=库、模块=源码、registry=演化。
- 全源码（Kernel 也安装为源码）：所有权更彻底，但失去编译期稳定契约，升级面更碎。
- 全库（模块做成库）：回到插件/回调地狱，无法预判产品的真实变化点，已被讨论否决。
- 纯 fork 模板：横切改进无法回流，且「按需安装」无从谈起。
