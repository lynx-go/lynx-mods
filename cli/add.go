package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
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
	registryDir := fs.String("registry", os.Getenv("LYNXSTART_REGISTRY"), "lynx-start repo directory")
	overwrite := fs.Bool("o", false, "overwrite existing files")
	overwriteLong := fs.Bool("overwrite", false, "overwrite existing files")
	dryRun := fs.Bool("dry-run", false, "preview without writing")
	dir := fs.String("dir", ".", "app directory")
	args = reorderArgs(map[string]bool{"registry": true, "dir": true}, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: lynx-start add <module>[@<version>] [-o] [--dry-run] [--registry <dir>]")
	}
	// name@version：无版本取 registry 工作区最新；有版本从 git tag
	// modules/<name>/v<version> 提取（可复现安装）。
	name := fs.Arg(0)
	version := ""
	if base, ver, found := strings.Cut(name, "@"); found {
		name, version = base, ver
	}
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
		return fmt.Errorf("app is missing go.mod: lynx-start init generates it; for a hand-created app run go mod init %s first", m.ModulePath)
	}
	if *registryDir == "" {
		*registryDir = m.Registry // 清单里记的仓库目录兜底
	}
	idx, err := registryLoad(*registryDir)
	if err != nil {
		return err
	}
	item, cleanup, err := idx.ItemAt(*registryDir, name, version)
	if err != nil {
		return err
	}
	defer cleanup()

	// 模块依赖（domain 级 DAG）必须已装。
	for _, dep := range item.ModuleDependencies {
		if !m.Has(dep) {
			return fmt.Errorf("module %s depends on %s: run lynx-start add %s first", name, dep, dep)
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
		fmt.Printf("\n(dry-run) would write %d files, skip %d\n", len(res.Created), len(res.Skipped))
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
	}

	// 4) 基线快照 + 版本 pin：无论首装还是重装（-o），凡本次被 upstream
	//    形态写入的文件都进基线——update 三方合并的 base 由此而来。
	files := append([]string{}, res.Created...)
	files = append(files, res.Skipped...)
	sort.Strings(files)
	if err := syncBaseline(appDir, item.Name, res); err != nil {
		return fmt.Errorf("snapshot install baseline: %w", err)
	}
	m.Set(manifest.Module{Name: item.Name, Version: item.Version, Files: files})
	if err := m.Save(appDir); err != nil {
		return err
	}

	fmt.Printf("\n✓ module %s@%s installed (%d written, %d skipped)\n", item.Name, item.Version, len(res.Created), len(res.Skipped))
	if len(res.Skipped) > 0 && !overwriteAny {
		fmt.Println("  note: existing files were skipped (local-modification guard); use -o to overwrite")
	}
	fmt.Printf(`Next steps (order matters: go mod tidy fails before genproto exists):
  mise install && mise trust   # toolchain (go / buf; skip if already set up)
  mise run generate            # buf generates genproto/
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
		return fmt.Errorf("read modules.go (generated by lynx-start init): %w", err)
	}
	src := string(raw)
	if strings.Contains(src, name+".Module()") {
		return nil // 已登记
	}
	entry := fmt.Sprintf("{\n\t\t\tModule:   %s.Module(),\n\t\t\tAssemble: %s.Assemble,\n\t\t},\n\t\t", name, name)
	if !strings.Contains(src, entryAnchor) {
		return fmt.Errorf("modules.go is missing entry anchor %q (deleted? diff against the scaffold template)", entryAnchor)
	}
	src = strings.Replace(src, entryAnchor, entry+entryAnchor, 1)

	imp := fmt.Sprintf("%s %q\n", name, modulePath+"/internal/api/"+name)
	if !strings.Contains(src, importAnchor) {
		return fmt.Errorf("modules.go is missing import anchor %q", importAnchor)
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
		return fmt.Errorf("read config.yaml: %w", err)
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
		return fmt.Errorf("config.yaml is missing config anchor %q", configAnchor)
	}
	src = strings.Replace(src, configAnchor, configAnchor+"\n"+strings.TrimPrefix(block, "\n"), 1)
	return os.WriteFile(path, []byte(src), 0o644)
}

// registryLoad 是 registry.Load 的薄封装（init/list 复用）。
func registryLoad(dir string) (*registry.Index, error) {
	return registry.Load(dir)
}
