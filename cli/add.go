package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"

	"github.com/lynx-go/lynx-start/cli/internal/manifest"
	"github.com/lynx-go/lynx-start/cli/internal/registry"
	"github.com/lynx-go/lynx-start/cli/internal/render"
)

// 注入锚点（与 scaffold/internal/modules/modules.go、configs/config.yaml 模板一致）。
const (
	importAnchor = "// lynx-start:module-import（lynx-start add 注入锚点，勿删）"
	entryAnchor  = "// lynx-start:module-entry（lynx-start add 注入锚点，勿删）"
	configAnchor = "# lynx-start:config-fragment（lynx-start add 追加模块配置段的锚点，勿删）"
)

// cmdAdd: lynx-start add <module> —— 安装模块源码到当前应用。
func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	registryDir := fs.String("registry", os.Getenv("LYXSTART_REGISTRY"), "lynx-start 仓库目录")
	overwrite := fs.Bool("o", false, "覆盖已存在文件")
	overwriteLong := fs.Bool("overwrite", false, "覆盖已存在文件")
	dryRun := fs.Bool("dry-run", false, "预览不落盘")
	dir := fs.String("dir", ".", "应用目录")
	args = reorderArgs(map[string]bool{"registry": true, "dir": true}, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: lynx-start add <module> [-o] [--dry-run] [--registry <dir>]")
	}
	name := fs.Arg(0)
	appDir, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	m, err := manifest.Load(appDir)
	if err != nil {
		return err
	}
	// go.mod 必须先在（init 生成；手建应用需先 go mod init）——否则后面
	// go mod tidy 报「cannot find main module」，与 genproto 缺失的报错
	// 一样难以归因，这里提前给出可行动的错误。
	if _, statErr := os.Stat(filepath.Join(appDir, "go.mod")); statErr != nil {
		return fmt.Errorf("应用缺少 go.mod：lynx-start init 生成的应用已自带；手动创建的应用请先 go mod init %s", m.ModulePath)
	}
	if *registryDir == "" {
		*registryDir = m.Registry // 清单里记的仓库目录兜底
	}
	idx, err := registryLoad(*registryDir)
	if err != nil {
		return err
	}
	item, err := idx.Item(*registryDir, name)
	if err != nil {
		return err
	}

	// 模块依赖（domain 级 DAG）必须已装。
	for _, dep := range item.ModuleDependencies {
		if !m.Has(dep) {
			return fmt.Errorf("模块 %s 依赖 %s：请先 lynx-start add %s", name, dep, dep)
		}
	}
	already := m.Has(name)

	data := render.NewData(m.Name, m.ModulePath)
	overwriteAny := *overwrite || *overwriteLong

	// 1) 渲染模块源码树：模块在本仓是真实 Go module，安装时排除工程文件
	//    （go.mod/item.json/genproto/buf 配置），.go/.proto 中的规范 module
	//    path 改写为应用 module path（shadcn alias rewrite 的对应物；安装
	//    时模块名目录拍平：…/modules/<name>/X → <app>/X）。
	//    本模块与全部声明依赖都要改写——模块可能 import 依赖模块的 domain
	//    包（如 session → identity），依赖方必已安装（上面已校验）。
	prefixes := [][]byte{[]byte(item.CanonicalModulePath() + "/")}
	for _, dep := range item.ModuleDependencies {
		depItem, err := idx.Item(*registryDir, dep)
		if err != nil {
			return err
		}
		prefixes = append(prefixes, []byte(depItem.CanonicalModulePath()+"/"))
	}
	appPath := []byte(m.ModulePath)
	res, err := render.Tree(item.Dir, appDir, data, render.Options{
		Overwrite: overwriteAny,
		DryRun:    *dryRun,
		Skip: func(rel string) bool {
			return rel == "item.json" || rel == "go.mod" ||
				strings.HasPrefix(rel, "genproto/") ||
				rel == "buf.yaml" || rel == "buf.gen.yaml" || rel == "buf.lock"
		},
		Rewrite: func(rel string, content []byte) []byte {
			if !strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, ".proto") {
				return content
			}
			for _, p := range prefixes {
				content = bytes.ReplaceAll(content, p, append(appPath, '/'))
			}
			return content
		},
		Report: printf,
	})
	if err != nil {
		return err
	}
	if *dryRun {
		fmt.Printf("\n(dry-run) 将写入 %d 个文件、跳过 %d 个\n", len(res.Created), len(res.Skipped))
		return nil
	}

	// 2) modules.go 注入（幂等）。
	if !already {
		if err := injectModulesEntry(appDir, name, m.ModulePath); err != nil {
			return err
		}
		// 3) config.yaml 追加模块配置段（幂等）。
		if err := appendConfigFragment(appDir, name, item.ConfigFragment); err != nil {
			return err
		}
		m.Modules = append(m.Modules, manifest.Module{Name: item.Name, Version: item.Version})
		if err := m.Save(appDir); err != nil {
			return err
		}
	}

	fmt.Printf("\n✓ 模块 %s@%s 安装完成（写入 %d、跳过 %d）\n", item.Name, item.Version, len(res.Created), len(res.Skipped))
	if len(res.Skipped) > 0 && !overwriteAny {
		fmt.Println("  注意：存在已跳过的同名文件（本地修改保护）；要覆盖用 -o")
	}
	fmt.Printf(`下一步（顺序重要：genproto 未生成时 go mod tidy 必失败）：
  mise install && mise trust   # 工具链（go / buf；已初始化的应用可跳过）
  mise run generate            # buf 生成 genproto/
  go mod tidy%s
  mise run db:up && mise run db:migrate
  mise run dev                 # gRPC :9060 / HTTP :8080
`, goGetHint(item))
	return nil
}

func goGetHint(item *registry.Item) string {
	if len(item.GoDependencies) == 0 {
		return ""
	}
	return "\n  go get " + strings.Join(item.GoDependencies, " ")
}

// injectModulesEntry 在 internal/modules/modules.go 的锚点处注入 import 与
// 装配登记（存在即跳过，幂等）。
func injectModulesEntry(appDir, name, modulePath string) error {
	path := filepath.Join(appDir, "internal", "modules", "modules.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 modules.go（lynx-start init 生成物）: %w", err)
	}
	src := string(raw)
	if strings.Contains(src, name+".Module()") {
		return nil // 已登记
	}
	entry := fmt.Sprintf("{\n\t\t\tModule:   %s.Module(),\n\t\t\tAssemble: %s.Assemble,\n\t\t},\n\t\t", name, name)
	if !strings.Contains(src, entryAnchor) {
		return fmt.Errorf("modules.go 缺少入口锚点 %q（误删？重新比对 scaffold 模板）", entryAnchor)
	}
	src = strings.Replace(src, entryAnchor, entry+entryAnchor, 1)

	imp := fmt.Sprintf("%s %q\n", name, modulePath+"/internal/api/"+name)
	if !strings.Contains(src, importAnchor) {
		return fmt.Errorf("modules.go 缺少 import 锚点 %q", importAnchor)
	}
	if !strings.Contains(src, fmt.Sprintf("%q", modulePath+"/internal/api/"+name)) {
		src = strings.Replace(src, importAnchor, imp+importAnchor, 1)
	}
	formatted, ferr := format.Source([]byte(src))
	if ferr != nil {
		return fmt.Errorf("gofmt modules.go after injection: %w", ferr)
	}
	return os.WriteFile(path, formatted, 0o644)
}

// appendConfigFragment 在 configs/config.yaml 锚点后追加模块配置段（幂等）。
func appendConfigFragment(appDir, name, fragment string) error {
	path := filepath.Join(appDir, "configs", "config.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 config.yaml: %w", err)
	}
	src := string(raw)
	marker := fmt.Sprintf("# --- module: %s ---", name)
	if strings.Contains(src, marker) {
		return nil
	}
	block := "\n" + marker + "\n"
	if fragment != "" {
		block += fragment
		if !strings.HasSuffix(fragment, "\n") {
			block += "\n"
		}
	}
	if !strings.Contains(src, configAnchor) {
		return fmt.Errorf("config.yaml 缺少配置锚点 %q", configAnchor)
	}
	src = strings.Replace(src, configAnchor, configAnchor+"\n"+strings.TrimPrefix(block, "\n"), 1)
	return os.WriteFile(path, []byte(src), 0o644)
}

// registryLoad 是 registry.Load 的薄封装（init/list 复用）。
func registryLoad(dir string) (*registry.Index, error) {
	return registry.Load(dir)
}
