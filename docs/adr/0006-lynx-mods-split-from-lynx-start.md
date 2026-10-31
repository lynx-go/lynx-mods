# ADR-0006: lynx-mods 成立——承接 lynx-start 的模块分发

日期：2026-10-31
状态：已接受

## 背景

lynx-start 定位调整为「lynx 框架的 web 开发基础库」（契约 + 电池 +
contrib 适配），其类 shadcn/ui 的模块分发功能（registry + 安装 CLI +
scaffold）整体迁入本仓。完整决策账本见
lynx-start/docs/adr/0009（D1-D15）；本 ADR 记录承接侧的关键决定。

## 决策

1. **CLI 整体迁移**（含 init/generate/scaffold）：`init --preset` 与安装器
   在代码级交织（原 lynx-start cli/init.go:98-110），不拆分。二进制与
   cmd 目录更名 **lynx-mods**（`go install
   github.com/lynx-go/lynx-mods/cli/cmd/lynx-mods@latest`）。
2. **module path 换系**：8 个模块 `github.com/lynx-go/lynx-mods/modules/<m>`
   → `github.com/lynx-go/lynx-mods/modules/<m>`（互 import、go.mod、
   item.json.modulePath、`.proto` go_package、CLI canonicalPrefix 同步）。
   对契约库的依赖改为版本化 require：
   `github.com/lynx-go/lynx-start v0.5.0` +
   `github.com/lynx-go/lynx-start/contrib/bunx v0.1.0`（不再 replace
   本地 kernel）。
3. **契约调用面换新**（lynx-start v0.5.0 纯契约化）：
   `compose.ValidateModules/AssembleAll/PolicyFiles/SortModules/Descriptors`
   → `compose.` 同名；`chain.DefaultChain` → `chain.DefaultChain`；
   `Deps.DB` 改 `any`，模块装配处 `NewRepository(bunx.MustDB(deps))` →
   `NewRepository(bunx.MustDB(deps))`。
4. **历史 tag 原样搬运**：10 个 `modules/*/v0.1.x` tag 经 `git fetch`
   从 lynx-start 原样引入（对象真实、载荷差异保留——e2e 的
   organization@0.1.0 场景依赖旧载荷真实缺少 organization_name）。
5. **安装器双前缀兼容**：canonicalModulesPrefix 由单一常量改为集合
   （新 `lynx-mods/modules/` 主 + 旧 `lynx-start/modules/` 兼容），
   同时覆盖 `.go` import 与 `.proto` go_package 两条改写路径及旧 tag
   item.json.modulePath 对账——安装 lynx-start 时代的历史版本照常改写。
6. **版本策略**：8 模块 item.json version 0.1.x→**0.2.0**（module path
   变更诚实升版）；kernelConstraint→`>=0.5.0`；grpcapi 统一 v0.4.0
   （含模块 buf.yaml 的 BSR dep）；bun 钉 v1.3.0；CLI 发 `cli/v0.3.0`
   （二进制更名+registry 换仓）。advisories 保留历史 ID
   `LYNXSTART-0001`（不可变标识，指向搬来的历史 tag 仍成立），新公告
   用 `LYNXMODS-` 前缀。
7. **应用侧工件更名**：`lynx-mods.json`→`lynx-mods.json`、
   `.lynx-mods/`→`.lynx-mods/`、`~/.lynx-mods/`→`~/.lynx-mods/`、
   `~/.cache/lynx-mods/`→`~/.cache/lynx-mods/`；环境变量
   `LYNXMODS_REGISTRY`→`LYNXMODS_REGISTRY`、`LYNXMODS_E2E_MODULES`→
   `LYNXMODS_E2E_MODULES`；**`LYNXSTART_TEST_PG_DSN` 不改名**——它属
   lynx-start 的 contrib/bunx/testutil（库侧契约），本仓 CI 必须设此名。
8. **默认 registry 指向**：文档与示例一律指向本仓
   （`https://github.com/lynx-go/lynx-mods.git`）；`--registry` 仍接受
   任意 git URL（私有 registry 是开放性接缝）。

## 后果

- 正面：范式基座（lynx-start）与分发业务（本仓）独立演化、独立发版；
  历史 tag 真实可溯；lynx-start 时代安装的应用可平滑升级。
- 代价：module path 换系是一次性的（迁移 commit + 0.2.0 诚实升版）；
  e2e 双前缀金标准（`add identity@0.1.0` 旧 tag → generate → build）
  必须持续守护兼容路径。

## 关联

- lynx-start ADR-0009（D1-D15 全账本）、UPGRADING.md（应用迁移指引）
- 本仓 ADR-0002（模块模型六契约，Origin lynx-start 0004）等重编号系列
