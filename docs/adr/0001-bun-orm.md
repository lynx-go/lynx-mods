# ORM 选用 bun

数据访问层采用 uptrace/bun（pgdialect + pgdriver）。决定性理由是资产继承：参考实现 torchwood 与 lynx-clean-template 均基于 bun，其 bunrepo/mapper 分层、基于 `bun.IDB` 的 unit-of-work 模式、UPDATE 显式列白名单护栏（torchwood 2026-09-08 零值列事故的产物）都是踩坑后的现成答案。GORM 链式语义魔法多、AutoMigrate 与 SQL 迁移哲学冲突；ent 的 codegen 心智负担高；sqlc 在动态列表过滤（AIP-160 风格）上是硬伤。

## Considered Options

- **bun（选定）**：与两个参考项目一致，`IDB` 接口最适合事务管理器模式，raw SQL 一等公民。
- **GORM**：社区与语料最大，但与参考栈零交集，零值更新坑更隐蔽。
- **ent**：privacy 层对多租户有独特价值，可作 fork 进阶方向，不作模板默认。
- **sqlc**：可直接读 goose 迁移做编译期校验，适合未来复杂报表的混合补充，不做主 ORM。
