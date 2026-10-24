// Package kernel 是 lynx-start 的稳定契约库（ADR-0003 三层供给的中间层）：
// 装配协议、Principal 语境、错误码区段、列表抽象、事务端口、事件信封与
// per-module 迁移辅助。Kernel 对模块做 semver 承诺；模块源码（registry item）
// 编译期依赖本库，但本库永不 import 任何模块代码（import guard 由应用侧
// 护栏测试强制）。
//
// 契约要点见 docs/adr/0004-module-model-contracts.md：
//   - 模块间仅 domain 级 DAG 依赖；
//   - 配置自治（UnmarshalKey 自有段）；
//   - 迁移链隔离（独立 goose 版本表）；
//   - 错误码区段制；
//   - Kernel 无租户预设。
package kernel

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/lynx-go/grpcapi/server"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Module 是模块的静态装配贡献面（装配协议的静态半边；动态半边见
// InstalledModule/Assembly）。
//
// 模块源码暴露两样东西：`Module()`（本结构：静态描述）与
// `Assemble(kernel.Deps) (kernel.Assembly, error)`（动态装配）。应用侧
// `internal/modules/modules.go` 的 Installed 列表登记两者，main.go 经
// AssembleAll 聚合。
type Module struct {
	// Name 是模块唯一名：小写字母/数字/中划线，同时用作配置段名、迁移目录名
	// 与错误码区段名。
	Name string

	// Description 一句话用途说明（registry 展示用）。
	Description string

	// DependsOn 声明依赖的其他模块名（domain 级 DAG）。安装器与 doctor 校验
	// 无环；被依赖模块的 domain 包是唯一允许 import 的跨模块包。
	DependsOn []string

	// Policies 是模块 proto 的 FileDescriptor（生成物自带），grpcapi
	// policy.Build(policy.FromFiles(...)) 的输入。
	Policies []protoreflect.FileDescriptor

	// Migrations 是模块迁移目录（embed.FS，root 指向该模块的迁移文件），
	// goosekit 以独立版本表执行（契约④：迁移链隔离）。
	Migrations fs.FS

	// ExtraRoutes 是模块的静态手写 HTTP 路由贡献（依赖注入构造的经 Assembly）。
	ExtraRoutes []server.ExtraRoute

	// Config 声明模块的配置段。
	Config ConfigSpec
}

// ConfigSpec 声明模块配置段：模块在自己的 Init 里以
// `UnmarshalKey(spec.Key, &cfg, lynx.WithErrorUnused())` 解码自有 struct
// （ADR-0004 契约③：配置自治，无全局 config.proto）。
type ConfigSpec struct {
	// Key 是配置段名；空则默认为模块名。
	Key string
	// Defaults 是段内默认值（安装器写入 config.yaml.template 片段时参考）。
	Defaults map[string]any
}

// KeyOrName 返回生效的配置段名。
func (s ConfigSpec) KeyOrName(name string) string {
	if s.Key != "" {
		return s.Key
	}
	return name
}

// ValidateName 校验模块名合法性：非空、仅小写字母/数字/中划线。
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("module name is empty")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return fmt.Errorf("module name %q: only lowercase letters, digits and '-' are allowed", name)
		}
	}
	return nil
}

// ValidateModules 对已装模块集合做启动前校验：名字合法且唯一、DependsOn
// 指向已装模块、依赖图无环（domain 级 DAG 契约）。校验失败应在组合根即告警，
// doctor 命令复用同一入口。
func ValidateModules(mods []Module) error {
	seen := make(map[string]bool, len(mods))
	for _, m := range mods {
		if err := ValidateName(m.Name); err != nil {
			return err
		}
		if seen[m.Name] {
			return fmt.Errorf("duplicate module name %q", m.Name)
		}
		seen[m.Name] = true
	}
	// Kahn 拓扑校验：所有声明的依赖必须已装，且不存在环。
	deps := make(map[string][]string, len(mods))
	indeg := make(map[string]int, len(mods))
	for _, m := range mods {
		for _, d := range m.DependsOn {
			if !seen[d] {
				return fmt.Errorf("module %q depends on %q which is not installed", m.Name, d)
			}
			deps[d] = append(deps[d], m.Name)
			indeg[m.Name]++
		}
	}
	var queue []string
	for _, m := range mods {
		if indeg[m.Name] == 0 {
			queue = append(queue, m.Name)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range deps[cur] {
			indeg[next]--
			if indeg[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	for name, d := range indeg {
		if d > 0 {
			return fmt.Errorf("module dependency cycle involving %q", name)
		}
	}
	return nil
}

// PolicyFiles 汇总全部模块的策略描述符（policy.FromFiles 的输入）。
func PolicyFiles(mods []Module) []protoreflect.FileDescriptor {
	var out []protoreflect.FileDescriptor
	for _, m := range mods {
		out = append(out, m.Policies...)
	}
	return out
}

// SortModules 按名字稳定排序（装配与展示的一致性）。
func SortModules(mods []Module) {
	sort.Slice(mods, func(i, j int) bool { return mods[i].Name < mods[j].Name })
}

// Descriptors 抽取已装模块的静态描述（PolicyFiles 等静态聚合的输入）。
func Descriptors(mods []InstalledModule) []Module {
	out := make([]Module, 0, len(mods))
	for _, m := range mods {
		out = append(out, m.Module)
	}
	return out
}
