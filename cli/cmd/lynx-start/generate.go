package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cmdGenerate: lynx-start generate —— 应用侧代码生成管线（ADR-0006 把
// 布局规则收到 CLI：单一事实源，规则演进靠升级 CLI，不靠改 scaffold
// 生成的静态 mise 脚本）：
//
//  1. buf dep update && buf lint && buf generate（buf 由应用 mise 钉版，
//     CLI 只编排不重新实现；产出应用根 genproto/<m>/…）
//  2. 归位：genproto/<m>/ → modules/<m>/genproto/<m>/（已装/自建模块目录
//     存在才归位；不认识的留在应用根）。生成物是应用侧产物（契约①：
//     不预生成入库、不进三方合并），只在模块目录里落位。
//
// 幂等：开头清应用根 genproto（buf 不回收孤儿文件），归位前清目标目录。
func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	dir := fs.String("dir", ".", "app directory")
	args = reorderArgs(map[string]bool{"dir": true}, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	appDir, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(appDir, "buf.gen.yaml")); err != nil {
		return fmt.Errorf("no buf.gen.yaml in %s (run inside a lynx-start app: lynx-start init generates it)", appDir)
	}
	if _, err := exec.LookPath("buf"); err != nil {
		return fmt.Errorf("buf not found in PATH: run `mise install` in the app (scaffold pins the buf toolchain)")
	}

	steps := [][]string{
		{"buf", "dep", "update"},
		{"buf", "lint"},
		{"buf", "generate"},
	}
	for _, step := range steps {
		cmd := exec.Command(step[0], step[1:]...)
		cmd.Dir = appDir
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		fmt.Printf("== %s\n", strings.Join(step, " "))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s failed (run it directly for details)", strings.Join(step, " "))
		}
	}
	return relocateGenproto(appDir)
}

// relocateGenproto 把应用根 genproto/<m>/ 归位到 modules/<m>/genproto/<m>/。
// 目标模块目录不存在（未装同名模块）的留在应用根——可能是应用自身的
// proto 或用户手工放置。
func relocateGenproto(appDir string) error {
	genRoot := filepath.Join(appDir, "genproto")
	entries, err := os.ReadDir(genRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 没有生成物（应用尚无 proto）
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		moduleDir := filepath.Join(appDir, "modules", name)
		if info, err := os.Stat(moduleDir); err != nil || !info.IsDir() {
			continue
		}
		// 目标是 modules/<m>/genproto/<m>/：buf 输出 genproto/<m>/v1，
		// 归位后保持 <m>/v1 层级（与 go_package 改写目标一致）。
		genprotoDir := filepath.Join(moduleDir, "genproto")
		dst := filepath.Join(genprotoDir, name)
		if err := os.RemoveAll(genprotoDir); err != nil {
			return err
		}
		if err := os.MkdirAll(genprotoDir, 0o755); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(genRoot, name), dst); err != nil {
			return err
		}
		fmt.Printf("genproto %s → modules/%s/genproto/%s\n", name, name, name)
	}
	return nil
}
