# {{.AppName}}

基于 [lynx-start](https://github.com/lynx-go/lynx-start) 模块基建的 Go 应用：
lynx 运行时 + grpcapi 双面（gRPC + HTTP gateway）+ bun/Postgres + goose 迁移。

## 已装模块

见 `lynx-start.json`（安装清单）与 `internal/modules/modules.go`（装配登记）。

## 快速开始

```bash
mise install && mise trust  # 工具链（go / buf）
mise run generate           # proto → genproto/（必须先于 go mod tidy）
go mod tidy
mise run db:up              # 本地 Postgres（可选：cp .env.example .env 覆盖端口/DSN）
mise run db:migrate         # 各模块迁移链（独立版本表）
mise run dev                # gRPC :9060 / HTTP :8080
```

## 添加模块

```bash
lynx-start list             # 查看模块目录
lynx-start add <module>
```

## 写自己的模块

已装模块是源码，可直接改；要新增业务域，照 `demo` 模块的四层结构手写并登记到
`internal/modules/modules.go`。完整步骤见
[module-authoring 指南](https://github.com/lynx-go/lynx-start/blob/main/docs/module-authoring.md)。

## 配置

`configs/config.yaml` + 环境变量覆盖（前缀 `{{.EnvPrefix}}_`）：
如 `data.database.source` → `{{.EnvPrefix}}_DATA_DATABASE_SOURCE`。

## 迁移

```bash
mise run db:migrate                              # 全部模块 up（DSN 缺省同 config.yaml）
go run ./cmd/migrate -dsn <dsn> -module demo     # 单模块
go run ./cmd/migrate -dsn <dsn> -down 1 -module demo
```
