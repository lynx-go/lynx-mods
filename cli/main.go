// lyx 是 lynx-start 的模块安装器（shadcn 语义）：
//
//	lyx init <name>            从 scaffold 生成应用骨架
//	lyx add <module>           安装模块源码到当前应用
//	lyx list                   列出 registry 中的模块
//
// 安装默认不覆盖已存在文件（-o 强制；--dry-run 预览）；更新=源码迁移，
// 靠 git 流程管理（shadcn 同款取舍）。
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "add":
		err = cmdAdd(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "lyx: 未知命令 %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lyx:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `用法:
  lyx init <name> [--module <path>] [--registry <dir>] [--force]
  lyx add <module> [--registry <dir>] [-o|--overwrite] [--dry-run]
  lyx list [--registry <dir>]
`)
}
