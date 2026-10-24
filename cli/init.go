package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lynx-go/lynx-start/cli/internal/manifest"
	"github.com/lynx-go/lynx-start/cli/internal/render"
)

// cmdInit: lynx-start init <name> —— 从 scaffold 生成应用骨架。
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	module := fs.String("module", "", "Go module path（默认 example.com/<name>）")
	registryDir := fs.String("registry", os.Getenv("LYXSTART_REGISTRY"), "lynx-start 仓库目录（含 scaffold/ 与 registry/）")
	force := fs.Bool("force", false, "目标目录已存在内容时继续")
	args = reorderArgs(map[string]bool{"module": true, "registry": true}, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("用法: lynx-start init <name> [--module <path>] [--registry <dir>]")
	}
	appName := fs.Arg(0)
	if *registryDir == "" {
		*registryDir = os.Getenv("LYNXSTART_REGISTRY")
	}
	if *registryDir == "" {
		return fmt.Errorf("--registry 必填（或设 LYNXSTART_REGISTRY）：指向 lynx-start 仓库目录")
	}

	target := appName
	if _, err := os.Stat(target); err == nil && !*force {
		if entries, _ := os.ReadDir(target); len(entries) > 0 {
			return fmt.Errorf("目录 %s 非空（--force 继续）", target)
		}
	}
	modulePath := *module
	if modulePath == "" {
		modulePath = "example.com/" + appName
	}
	scaffoldDir := filepath.Join(*registryDir, "scaffold")
	res, err := render.Tree(scaffoldDir, target, render.NewData(appName, modulePath),
		render.Options{Overwrite: *force, Template: true, Report: printf})
	if err != nil {
		return err
	}

	m := &manifest.Manifest{
		Schema:     1,
		Name:       appName,
		ModulePath: modulePath,
		Registry:   *registryDir,
	}
	if err := m.Save(target); err != nil {
		return err
	}

	fmt.Printf("\n✓ 应用骨架就绪：%s（%d 个文件，跳过 %d）\n", target, len(res.Created), len(res.Skipped))
	fmt.Printf(`下一步：
  cd %s
  git init && git add -A && git commit -m "chore: lynx-start init"
  go mod tidy && go get github.com/lynx-go/lynx-start/kernel@v0.1.0
  lynx-start add <module> --registry %s
`, target, *registryDir)
	return nil
}

// cmdList: lynx-start list —— 列出 registry 模块。
func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	registryDir := fs.String("registry", os.Getenv("LYXSTART_REGISTRY"), "lynx-start 仓库目录")
	if err := fs.Parse(args); err != nil {
		return err
	}
	idx, err := registryLoad(*registryDir)
	if err != nil {
		return err
	}
	fmt.Printf("%s registry（%d 个模块）:\n", idx.Name, len(idx.Items))
	for _, it := range idx.Items {
		item, err := idx.Item(*registryDir, it.Name)
		if err != nil {
			return err
		}
		deps := ""
		if len(item.ModuleDependencies) > 0 {
			deps = "  依赖: " + strings.Join(item.ModuleDependencies, ", ")
		}
		fmt.Printf("  %-16s %-8s %s%s\n", item.Name, item.Version, item.Description, deps)
	}
	return nil
}

func printf(format string, args ...any) { fmt.Printf(format+"\n", args...) }
