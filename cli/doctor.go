package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lynx-go/lynx-start/cli/internal/manifest"
)

// cmdDoctor: lynx-start doctor —— 应用与安装清单的一致性体检（只读）：
// modules.go 双向对账、config 段存在性、基线/文件完整性、模块 DAG。
// 任何 ✗ 项退出码非零（CI/脚本可用）。
func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	registryDir := fs.String("registry", os.Getenv("LYNXSTART_REGISTRY"), "lynx-start repo directory")
	dir := fs.String("dir", ".", "app directory")
	args = reorderArgs(map[string]bool{"registry": true, "dir": true}, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	appDir, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	m, err := manifest.Load(appDir)
	if err != nil {
		return err
	}
	if *registryDir == "" {
		*registryDir = m.Registry
	}
	idx, err := registryLoad(*registryDir)
	if err != nil {
		return err
	}
	problems := 0
	check := func(ok bool, format string, args ...any) {
		if ok {
			fmt.Printf("  ✓ ")
		} else {
			problems++
			fmt.Printf("  ✗ ")
		}
		fmt.Printf(format+"\n", args...)
	}

	fmt.Printf("lynx-start doctor: %s (%d modules)\n", appDir, len(m.Modules))

	// 1) modules.go 双向对账：登记的模块都在清单，清单的模块都已登记。
	modulesSrc := ""
	if raw, err := os.ReadFile(filepath.Join(appDir, "internal", "modules", "modules.go")); err == nil {
		modulesSrc = string(raw)
	} else {
		check(false, "read internal/modules/modules.go: %v", err)
	}
	if modulesSrc != "" {
		for _, mod := range m.Modules {
			check(strings.Contains(modulesSrc, mod.Name+".Module()"),
				"modules.go registers %s", mod.Name)
		}
		for _, name := range registeredModuleNames(modulesSrc) {
			_, ok := m.Get(name)
			check(ok, "manifest lists %s (registered in modules.go but missing from lynx-start.json)", name)
		}
	}

	// 2) 每个已装模块：依赖已装、config 段在、Files 在应用、基线完整。
	for _, mod := range m.Modules {
		item, err := idx.Item(*registryDir, mod.Name)
		if err != nil {
			check(false, "registry knows %s: %v", mod.Name, err)
			continue
		}
		for _, dep := range item.ModuleDependencies {
			check(m.Has(dep), "%s dependency %s installed", mod.Name, dep)
		}
		if item.ConfigFragment != "" {
			raw, err := os.ReadFile(filepath.Join(appDir, "configs", "config.yaml"))
			has := err == nil && strings.Contains(string(raw), fmt.Sprintf("# --- module: %s ---", mod.Name))
			check(has, "%s config fragment present", mod.Name)
		}
		gone := 0
		for _, rel := range mod.Files {
			if _, err := os.Stat(filepath.Join(appDir, filepath.FromSlash(rel))); err != nil {
				gone++
			}
		}
		check(gone == 0, "%s files intact (%d of %d missing)", mod.Name, gone, len(mod.Files))
		if len(mod.Files) == 0 {
			check(false, "%s has no update baseline (add %s -o to rebuild)", mod.Name, mod.Name)
		} else {
			baseDir := baselineDir(appDir, mod.Name)
			baseGone := 0
			for _, rel := range mod.Files {
				if _, err := os.Stat(filepath.Join(baseDir, filepath.FromSlash(rel))); err != nil {
					baseGone++
				}
			}
			check(baseGone == 0, "%s baseline intact (%d of %d missing)", mod.Name, baseGone, len(mod.Files))
		}
	}

	if problems == 0 {
		fmt.Println("✓ all checks passed")
		return nil
	}
	return fmt.Errorf("doctor found %d problem(s)", problems)
}

// registeredModuleNames 从 modules.go 源码提取 <name>.Module() 形态的模块名
// （手工追加登记的模块没有 manifest 条目时用于反向对账）。
func registeredModuleNames(src string) []string {
	re := regexp.MustCompile(`([A-Za-z][A-Za-z0-9_]*)\.Module\(\)`)
	var names []string
	seen := map[string]bool{}
	for _, match := range re.FindAllStringSubmatch(src, -1) {
		name := match[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
