# 安装布局镜像挂载与基线缓存分离

2026-10-25 用户发起的重大变更：模块装入应用后统一进 `modules/<m>/` 专用区
（不合并进应用顶层 domain/、internal/{app,api,infra}/），安装基线从应用内
`.lynx-start/` 移到用户目录 `~/.lynx-start/` 并按 `<module>@<commit>` 区分
多版本。本文记录四个决策与其依据。

## 背景

旧布局把模块四层**合并**进应用顶层目录（`<app>/domain/<m>/`、
`<app>/internal/{app,api,infra}/<m>/`）——多模块共享目录靠尾段模块名隔离。
痛点：外部模块源码与应用自有代码（internal/bootstrap、internal/config）
混居无分区；基线目录 `.lynx-start/baseline/` 体积大且建议随应用提交，
污染仓库与 diff。

## 决策

### 1. 挂载点：顶层 `modules/<m>/`（非 internal/modules）

`modules/` = registry 镜像区（外部源码），`internal/` = 应用自有代码——
物理分区表达来源边界；与 registry 仓 `modules/` 同名同构，diff/更新对比
直觉化；路径少一层嵌套。模块内 `internal/**` 的 Go 可见性语义原样保留
（只有 `modules/<m>/` 树内可 import），**应用代码经 CLI 生成的桥接包**
`modules/<m>/module.go` 取得 Module/Assemble——这是应用访问模块装配
入口唯一通道。

### 2. 尾段拍平：`modules/<m>/{domain,internal/{app,api,infra}}`

`{domain,internal/{app,api,infra}}` 下的尾段模块名目录在安装映射时拍平
（`domain/identity/` → `modules/identity/domain/`）。专属父目录已物理隔离
模块，尾段的命名空间职责消失。代价：包目录名与包名不一致
（`modules/identity/domain` 包名 identity）——Go 允许（包名以源文件声明
为准），CLI 注入的 import 一律带显式别名消除歧义；guard_test 的
AssertDepsAtAPILayer 相对路径深度随之变化，安装时做一条确定性文本改写
（`../../internal/app` → `../internal/app`）。仓库内模块源码结构**不动**
——拍平只发生在安装映射。

### 3. proto 与 genproto 均集中应用根、generate 收敛 CLI

- proto 契约源集中应用根 `proto/<m>/v1/`（buf 单根、vendored policy.proto
  共享保护、proto-drift CI 不动）。
- genproto 生成物集中应用根 `genproto/<m>/v1/`（buf 直接输出、零后处理）。
  **修订记录（v0.2.1）**：v0.2.0 曾把 genproto 归位 `modules/<m>/genproto/`
  （generate 后搬移）——为「改写零特例 + 模块树 100% 镜像」；复盘推翻：
  genproto 特例是全部改写约定里最稳定的一条（目录名死约定），而生成物
  无人手工 diff、镜像完整性价值近乎零，搬移后处理反而是长期维护税
  （首版即出过目标路径 bug）且偏离 buf 生态惯例（根 genproto/ 是默认
  形态）。三目录各司其职：`proto/` 源、`genproto/` 生成物、`modules/`
  模块源码。
- **新命令 `lynx-start generate`**（exec buf 管线：dep update / lint /
  generate）：布局规则单一事实源在 CLI，规则演进靠升级 CLI（与 P1-⑥
  DefaultChain 下沉 kernel 同款哲学）。不靠安装时拷贝——契约①「生成物
  应用侧钉版 buf 本地生成」保持，且生成物不进三方合并。
- 导入改写为结构化规则族（genproto 特例 + domain/三层 internal 逐依赖），
  改写后残留扫描 fail-closed（规则漏配即报错，不静默产坏代码）。

### 4. 基线是用户目录缓存：`~/.lynx-start/baselines/<appModulePath>/<m>@<commit>`

- 应用仓库只提交 `.lynx-start/lock.json`（供应链锁）与 `lynx-start.json`
  （manifest）——「这个应用装了什么」的事实，跨机器可审计。
- 基线含改写后的导入路径（per-app），按应用 module path 隔离；同一模块
  不同 commit 并存（多版本区分）；备份（update 冲突覆盖前副本）同住用户
  目录。
- **基线可确定性重建**：按 lock 记录的 commit `git archive` 提取该版本
  源码，经 add 同款映射+改写管线复现（依赖前缀集 = manifest 已装模块，
  与安装时等价——改写只对源码中实际出现的前缀生效）。add/update/doctor
  共用 ensureBaseline：缓存缺失自动重建并提示；换机器不再依赖提交基线。
  重建失败的兜底是 `add -o` 重装（registry 历史被改写等极端情形）。

## Consequences

- 破坏性变更（v0.x 阶段直接 breaking）：旧布局应用 doctor 检出并给出
  重装指引；lynx-start-demo 手动迁移。
- modules.go 登记点从 `internal/modules/modules.go` 移到
  `modules/modules.go`（scaffold 模板与 ADR-0003 收敛点表述同步）。
- ADR-0004 契约①的分层表述语义不变——「模块代码落 domain/<m> 与
  internal/{app,api,infra}/<m>」描述的是模块仓侧；装入应用后的形态由
  本 ADR 的安装映射定义。
- 移除模块时 `modules/<m>/` 整树删除（桥接与 genproto 含内，均可再生成）；
  goose 迁移表仍保留（数据安全）。
