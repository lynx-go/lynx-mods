package kernel

import (
	"fmt"
	"log/slog"

	"github.com/lynx-go/grpcapi/authn"
	"github.com/lynx-go/grpcapi/interceptor"
	"github.com/lynx-go/grpcapi/server"
	"github.com/lynx-go/lynx"
	"github.com/lynx-go/lynx-start/kernel/events"
	"github.com/lynx-go/lynx-start/kernel/uow"
	"github.com/lynx-go/lynx/contrib/schedule"
	"github.com/lynx-go/lynx/eventbus"
	"github.com/uptrace/bun"
)

// Deps 是应用基础层提供给每个模块的共享依赖：数据库连接、工作单元、
// 事件发布端口、事件总线（订阅侧）、配置与日志。模块源码（安装后归应用
// 所有）在自己的 Assemble 里消费 Deps 构造仓储与用例——依赖显式、
// 代码可直读直改，不引入生成式 DI 的重装步骤。
type Deps struct {
	DB     *bun.DB
	Tx     uow.Runner
	Events events.Publisher
	// Bus 是事件总线的订阅侧：发布走 Events 端口（信封校验与编码归一），
	// 订阅跨模块事件目录（如 identity.user.password-changed）时用 Bus。
	Bus    eventbus.Bus
	Config lynx.Config
	Logger *slog.Logger
}

// Assembly 是模块的动态装配产物：需要构造依赖图才能产生的贡献面。
// 静态贡献（策略描述符、迁移、依赖声明）在 Module 上。
type Assembly struct {
	// Registrations 是模块的 gRPC/HTTP 服务注册（grpcapi）。
	Registrations []server.Registration
	// Services 是模块贡献的 lynx.Service（事件订阅、后台作业等）。
	Services []lynx.Service
	// Tasks 是模块贡献的定时任务，应用聚合进唯一 Scheduler。
	Tasks []schedule.Task
	// Authenticators 是模块贡献的认证器（如 session/apikey 模块）。
	Authenticators []authn.Authenticator
	// Enrichers 是模块贡献的认证后增强器（如 access 模块往 Principal
	// 注入 roles/permissions claims——proto 策略的 permissions/roles
	// 声明由此生效）。
	Enrichers []authn.Enricher
	// Stages 是模块贡献的拦截器槽位（audit=50/ratelimit=40/usage=60）。
	Stages []interceptor.Stage
	// ExtraRoutes 是模块的手写 HTTP 路由。
	ExtraRoutes []server.ExtraRoute
}

// InstalledModule 是应用已装模块的完整形态：静态描述 + 动态装配入口。
// 应用侧 internal/modules/modules.go 的 Installed 列表登记全部已装模块，
// lyx add 负责追加登记。
type InstalledModule struct {
	Module Module
	// Assemble 构造模块的动态装配产物；每个应用进程调用一次。
	Assemble func(Deps) (Assembly, error)
}

// Aggregated 是全部已装模块装配产物的聚合，main.go 直接消费。
type Aggregated struct {
	Registrations  []server.Registration
	Services       []lynx.Service
	Tasks          []schedule.Task
	Authenticators []authn.Authenticator
	Enrichers      []authn.Enricher
	Stages         []interceptor.Stage
	ExtraRoutes    []server.ExtraRoute
}

// AssembleAll 校验已装模块集合（唯一性、依赖 DAG）并逐个装配、聚合。
// 任何模块装配失败即整体失败（fail-fast：模块是应用的一部分，不允许
// 半可用状态）。
func AssembleAll(deps Deps, mods []InstalledModule) (*Aggregated, error) {
	descriptors := make([]Module, 0, len(mods))
	for _, m := range mods {
		if m.Assemble == nil {
			return nil, fmt.Errorf("module %q: nil Assemble", m.Module.Name)
		}
		descriptors = append(descriptors, m.Module)
	}
	if err := ValidateModules(descriptors); err != nil {
		return nil, err
	}
	agg := &Aggregated{}
	for _, m := range mods {
		a, err := m.Assemble(deps)
		if err != nil {
			return nil, fmt.Errorf("assemble module %q: %w", m.Module.Name, err)
		}
		agg.Registrations = append(agg.Registrations, a.Registrations...)
		agg.Services = append(agg.Services, a.Services...)
		agg.Tasks = append(agg.Tasks, a.Tasks...)
		agg.Authenticators = append(agg.Authenticators, a.Authenticators...)
		agg.Enrichers = append(agg.Enrichers, a.Enrichers...)
		agg.Stages = append(agg.Stages, a.Stages...)
		agg.ExtraRoutes = append(agg.ExtraRoutes, a.ExtraRoutes...)
	}
	return agg, nil
}
