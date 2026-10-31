// lynx-mods 是模块安装器 CLI（shadcn 语义）：
//
//	lynx-mods init <name>              从 scaffold 生成应用骨架
//	lynx-mods add <module>[@<ver>]     安装模块源码到当前应用（版本走 git tag）
//	lynx-mods update <module>[@<ver>]  三方合并更新（本地 / 基线 / upstream）
//	lynx-mods remove <module>          卸载（DAG 反向校验 + 共享文件保护）
//	lynx-mods generate                 buf 生成管线 + genproto 归位 modules/<m>/
//	lynx-mods doctor                   应用与安装清单一致性体检
//	lynx-mods list                     列出 registry 中的模块
//
// 安装默认不覆盖已存在文件（-o 强制；--dry-run 预览）；add 建立
// .lynx-mods/baseline/ 安装基线，update 依此做三方合并（冲突打 git
// 标记，upstream 删除不自动执行）。基线随应用提交，跨机器可更新。
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
	case "update":
		err = cmdUpdate(os.Args[2:])
	case "remove":
		err = cmdRemove(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "generate":
		err = cmdGenerate(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "registry":
		err = cmdRegistry(os.Args[2:])
	case "audit":
		err = cmdAudit(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "lynx-mods: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lynx-mods:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage:
  lynx-mods init <name> [--module <path>] [--registry <dir>] [--force]
  lynx-mods add <module>[@<version>] [--registry <dir>] [-o|--overwrite] [--dry-run]
  lynx-mods update <module>[@<version>] [--registry <dir>] [--force]
  lynx-mods remove <module> [--registry <dir>] [--dry-run]
  lynx-mods generate [--dir <app>]         # buf pipeline + genproto relocation into modules/<m>/
  lynx-mods doctor [--registry <dir>] [--format text|github]
  lynx-mods audit [--registry <dir>] [--format text|github]   # security advisory gate (non-zero on hits)
  lynx-mods list [--registry <dir>]
  lynx-mods registry <add|check> [--registry <repo-root>] [<module-dir>]
      contributor tooling: register a module (item.json/registry.json/go.work) and check consistency

Versions resolve to git tags modules/<name>/v<x.y.z> in the registry repo;
add/update without @<version> take the registry worktree (latest).
`)
}
