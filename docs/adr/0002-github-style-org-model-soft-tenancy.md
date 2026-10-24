---
status: superseded by ADR-0004
---

# GitHub 式组织/成员模型，软租户隔离

> **已废弃（2026-10-24）**：项目重定位为通用模块基建后，本决策不再是模板核心。GitHub 式组织模型与软租户设计整体转入「未来多租户模块」的设计素材，竞品调研结论（个人组织、四角色、席位等）随行保留。

多租户采用 GitHub 式 Organization / Membership 模型：共享表 + `org_id` 外键 + 复合唯一约束，应用层强制注入租户语境；不采用 torchwood 的 schema-per-project + RLS 物理隔离。理由：模板面向一般 SaaS，schema-per-tenant 是 BaaS 特殊需求的产物，其 DDL 编排、运维与迁移成本对模板用户过高；GitHub 式组织/成员是 fork 用户最熟悉的心智模型。RLS 强化留作文档中的进阶选项，不在模板默认路径上。

## Consequences

- 数据完整性依赖「含 org_id 的复合唯一约束」+「租户上下文缺失即 fail-closed」，模板必须内置拦截器与 repo 注入护栏。
- 每个业务表的查询都必须显式携带 org 语境，repo 层需要统一的防漏注入机制（见 ADR-0001 的 bun 集成模式）。
