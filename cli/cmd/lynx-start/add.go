package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
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
	idx, regDir, err := registryLoad(*registryDir)
	if err != nil {
		return err
	}
	*registryDir = regDir
	item, cleanup, err := idx.ItemAt(*registryDir, name, version)
	if err != nil {
		return err
	}
	defer cleanup()

	// 模块依赖（domain 级 DAG）必须已装，且满足声明的最低版本
	//（item.json 的 [{"identity": ">=0.1.1"}] 形态）——把启动期 fail-closed
	// 前移到安装期。
	for _, dep := range item.ModuleDependencies {
		if !m.Has(dep.Name) {
			return fmt.Errorf("module %s depends on %s: run lynx-start add %s first", name, dep.Name, dep.Name)
		}
		if installed, ok := m.Get(dep.Name); ok && !dep.Meets(installed.Version) {
			return fmt.Errorf("module %s requires %s >=%s (installed %s): run lynx-start update %s first", name, dep.Name, dep.Min, installed.Version, dep.Name)
		}
	}
	// kernel 地板：模块声明的 kernelConstraint 对照应用 go.mod 的 require。
	if item.KernelConstraint != "" {
		min, err := registry.ParseConstraint(item.KernelConstraint)
		if err != nil {
			return fmt.Errorf("%s item.json kernelConstraint: %w", name, err)
		}
		have, err := appKernelVersion(appDir)
		if err != nil {
			return err
		}
		if registry.CompareVersions(have, min) < 0 {
			return fmt.Errorf("module %s requires kernel >=%s (app go.mod has %s): run go get github.com/lynx-go/lynx-start/kernel@v%s", name, min, have, min)
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
		depItem, err := idx.Item(*registryDir, dep.Name)
		if err != nil {
			return err
		}
		prefixes = append(prefixes, []byte(depItem.CanonicalModulePath()+"/"))
	}
	appPath := []byte(m.ModulePath)
	res, err := render.Tree(item.Dir, appDir, data, render.Options{
		Overwrite: overwriteAny,
		DryRun:    *dryRun,
		Skip:      skipModuleFile,
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
		if err := appendEnvExample(appDir, m, item); err != nil {
			fmt.Printf("warn: env example skipped: %v\n", err)
		}
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

// skipModuleFile 排除模块目录里的工程/生成文件（add 与 update 共用）：
// go.mod/go.sum 是模块自身模块管理的产物（应用的由 go mod tidy 生成——
// 尤其 go.sum，三方合并它只会产生垃圾冲突）；genproto 与 buf 配置由
// 应用的 buf 管线生成；item.json 是 registry 元数据。
func skipModuleFile(rel string) bool {
	return rel == "item.json" || rel == "go.mod" || rel == "go.sum" ||
		strings.HasPrefix(rel, "genproto/") ||
		rel == "buf.yaml" || rel == "buf.gen.yaml" || rel == "buf.lock"
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

// registryLoad 解析 registry 参数（git URL → 缓存 clone，本地路径原样）并
// 加载索引；返回已解析的本地目录（init/add/update/remove/doctor/list 复用，
// 后续 Item/ItemAt/scaffold 路径一律用它）。
func registryLoad(dir string) (*registry.Index, string, error) {
	resolved, err := registry.Resolve(dir)
	if err != nil {
		return nil, "", err
	}
	idx, err := registry.Load(resolved)
	return idx, resolved, err
}

// appKernelVersion 读应用 go.mod 里 kernel 的 require 版本（缺失返回
// "0.0.0"——约束校验会给出可行动报错）。
func appKernelVersion(appDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(appDir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read app go.mod: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "github.com/lynx-go/lynx-start/kernel "); ok {
			v := strings.TrimSpace(strings.Split(rest, " ")[0])
			return strings.TrimPrefix(v, "v"), nil
		}
	}
	return "0.0.0", nil
}

// appendEnvExample 把模块声明的 env 变量追加进 .env.example（幂等；Secret
// 变量生成随机占位——example 文件不入真值，生产由应用自行更换）。变量名
// 带应用前缀（与 config 环境覆盖约定一致：identity.secret → <P>_IDENTITY_SECRET）。
func appendEnvExample(appDir string, m *manifest.Manifest, item *registry.Item) error {
	if len(item.Env) == 0 {
		return nil
	}
	prefix := strings.ToUpper(strings.ReplaceAll(m.Name, "-", "_"))
	path := filepath.Join(appDir, ".env.example")
	src := ""
	if raw, err := os.ReadFile(path); err == nil {
		src = string(raw)
	}
	var block strings.Builder
	for _, e := range item.Env {
		name := prefix + "_" + e.Key
		if strings.Contains(src, name+"=") {
			continue
		}
		value := ""
		if e.Secret {
			buf := make([]byte, 16)
			if _, err := rand.Read(buf); err != nil {
				return fmt.Errorf("generate %s: %w", name, err)
			}
			value = hex.EncodeToString(buf)
		}
		desc := e.Description
		if e.Secret {
			desc += " (secret — rotate for production)"
		}
		fmt.Fprintf(&block, "\n# module %s: %s\n%s=%s\n", item.Name, desc, name, value)
	}
	if block.Len() == 0 {
		return nil
	}
	if src != "" && !strings.HasSuffix(src, "\n") {
		src += "\n"
	}
	return os.WriteFile(path, []byte(src+block.String()), 0o644)
}
