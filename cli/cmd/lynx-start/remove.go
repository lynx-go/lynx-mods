package main

import (
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lynx-go/lynx-start/cli/internal/manifest"
)

// cmdRemove: lynx-start remove <module> —— 卸载模块：回退 modules.go 注入
// 与 config 段、删除安装文件（共享文件保护）、清基线。迁移表 goose_<m>_db
// 一律保留（数据安全）；genproto/<m> 由 buf 管线生成，提示手动清理。
func cmdRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	registryDir := fs.String("registry", os.Getenv("LYNXSTART_REGISTRY"), "lynx-start repo directory")
	dir := fs.String("dir", ".", "app directory")
	dryRun := fs.Bool("dry-run", false, "preview without deleting")
	args = reorderArgs(map[string]bool{"registry": true, "dir": true}, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: lynx-start remove <module> [--registry <dir>] [--dry-run]")
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
	installed, ok := m.Get(name)
	if !ok {
		return fmt.Errorf("module %s is not installed (see lynx-start.json)", name)
	}
	if *registryDir == "" {
		*registryDir = m.Registry
	}
	idx, regDir, err := registryLoad(*registryDir)
	if err != nil {
		return err
	}
	*registryDir = regDir

	// 依赖方校验（DAG 反向）：任何其他已装模块声明依赖本模块即拒绝。
	var dependents []string
	for _, other := range m.Modules {
		if other.Name == name {
			continue
		}
		item, err := idx.Item(*registryDir, other.Name)
		if err != nil {
			return err
		}
		for _, dep := range item.ModuleDependencies {
			if dep.Name == name {
				dependents = append(dependents, other.Name)
			}
		}
	}
	if len(dependents) > 0 {
		return fmt.Errorf("module %s is depended on by %s: remove them first (reverse topological order)", name, strings.Join(dependents, ", "))
	}

	// 共享文件保护：vendored proto 等被多模块 Files 收录的文件不删。
	owned := map[string]bool{}
	for _, other := range m.Modules {
		if other.Name == name {
			continue
		}
		for _, f := range other.Files {
			owned[f] = true
		}
	}

	if *dryRun {
		fmt.Printf("(dry-run) remove %s@%s:\n", name, installed.Version)
	}
	removed, shared, missing := 0, 0, 0
	if !*dryRun {
		for _, rel := range installed.Files {
			if owned[rel] {
				shared++
				continue
			}
			p := filepath.Join(appDir, filepath.FromSlash(rel))
			if err := os.Remove(p); err != nil {
				if os.IsNotExist(err) {
					missing++
				} else {
					return err
				}
			} else {
				removed++
			}
		}
	} else {
		for _, rel := range installed.Files {
			switch {
			case owned[rel]:
				shared++
				fmt.Printf("  keep (shared with other modules) %s\n", rel)
			default:
				removed++
				fmt.Printf("  delete %s\n", rel)
			}
		}
	}

	if !*dryRun {
		// 注入回退（锚点保留，供将来 add 复用）。
		if err := revertModulesEntry(appDir, name, m.ModulePath); err != nil {
			return err
		}
		if err := removeEnvBlock(appDir, m, name); err != nil {
			fmt.Printf("warn: env cleanup skipped: %v\n", err)
		}
		if err := removeConfigFragment(appDir, name); err != nil {
			return err
		}
		// 基线/备份与清单条目。
		if err := os.RemoveAll(filepath.Join(appDir, ".lynx-start", "baseline", name)); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(appDir, ".lynx-start", "backup", name)); err != nil {
			return err
		}
		var kept []manifest.Module
		for _, mod := range m.Modules {
			if mod.Name != name {
				kept = append(kept, mod)
			}
		}
		m.Modules = kept
		if err := m.Save(appDir); err != nil {
			return err
		}
	}

	fmt.Printf("\n✓ module %s removed (%d files deleted, %d shared kept, %d already gone)\n", name, removed, shared, missing)
	fmt.Printf(`Not removed (deliberate):
  migrations table goose_%s_db    # data safety: drop manually if you really mean it
  genproto/%s                     # buf output: delete the dir, then mise run generate
Next:
  go mod tidy && go build ./...   # verify the app still compiles
`, name, name)
	return nil
}

// revertModulesEntry 从 modules.go 删除该模块的装配登记与 import（与
// injectModulesEntry 的插入格式对称；锚点保留）。未找到（用户手删过）
// 不算错误——登记缺失即目标状态。
func revertModulesEntry(appDir, name, modulePath string) error {
	path := filepath.Join(appDir, "internal", "modules", "modules.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read modules.go: %w", err)
	}
	src := string(raw)
	entry := fmt.Sprintf("{\n\t\t\tModule:   %s.Module(),\n\t\t\tAssemble: %s.Assemble,\n\t\t},\n\t\t", name, name)
	src = strings.ReplaceAll(src, entry, "")
	impRe := regexp.MustCompile(`(?m)^\t` + regexp.QuoteMeta(name) + ` "` + regexp.QuoteMeta(modulePath+"/internal/api/"+name) + `"\n`)
	src = impRe.ReplaceAllString(src, "")
	formatted, ferr := format.Source([]byte(src))
	if ferr != nil {
		return fmt.Errorf("gofmt modules.go after removal (hand-edited?): %w", ferr)
	}
	return os.WriteFile(path, formatted, 0o644)
}

// removeConfigFragment 删除 config.yaml 中该模块的配置段（marker 行到下一
// 个模块 marker 或文件尾；锚点行保留）。
func removeConfigFragment(appDir, name string) error {
	path := filepath.Join(appDir, "configs", "config.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config.yaml: %w", err)
	}
	lines := strings.Split(string(raw), "\n")
	marker := fmt.Sprintf("# --- module: %s ---", name)
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == marker {
			start = i
			break
		}
	}
	if start < 0 {
		return nil // 无该段（configFragment 为空的模块）
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "# --- module: ") {
			end = i
			break
		}
	}
	out := append(append([]string{}, lines[:start]...), lines[end:]...)
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

// removeEnvBlock 从 .env.example 删除该模块的 env 段（`# module <name>:` 注释
// 行与其后的变量行）；无段时幂等跳过。
func removeEnvBlock(appDir string, m *manifest.Manifest, name string) error {
	path := filepath.Join(appDir, ".env.example")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	marker := "# module " + name + ":"
	lines := strings.Split(string(raw), "\n")
	var out []string
	skipping := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, marker):
			skipping = true
		case skipping:
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				skipping = false
				out = append(out, line)
			}
			// 变量行（含空行前的赋值行）被丢弃
		default:
			out = append(out, line)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}
