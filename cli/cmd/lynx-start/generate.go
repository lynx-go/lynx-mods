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
// 生成的静态 mise 脚本）：buf dep update && buf lint && buf generate。
// buf 由应用 mise 钉版，CLI 只编排不重新实现；生成物统一落应用根
// genproto/<m>/…，零后处理（生成物是应用侧产物，契约①：不预生成入库、
// 不进三方合并）。
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
	return nil
}
