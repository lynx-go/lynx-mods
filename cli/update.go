package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lynx-go/lynx-start/cli/internal/manifest"
	"github.com/lynx-go/lynx-start/cli/internal/merge"
	"github.com/lynx-go/lynx-start/cli/internal/registry"
	"github.com/lynx-go/lynx-start/cli/internal/render"
)

// cmdUpdate: lynx-start update <module>[@<version>] —— 把已装模块更新到
// registry 新版，与本地改动做三方合并（local / 安装基线 / upstream）。
//
// 语义：基线 ≡ 上次从 registry 同步的形态（add 时建立，update 后刷新）。
// 本地与 upstream 改动不重叠则自动合并；同区改动打 git 冲突标记，冲突文件
// 覆盖前在 .lynx-start/backup/ 留副本。upstream 删除的文件不自动删（提示）。
// 建议先 commit 应用再 update——合并写盘不可自动回滚。
func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	registryDir := fs.String("registry", os.Getenv("LYNXSTART_REGISTRY"), "lynx-start repo directory")
	dir := fs.String("dir", ".", "app directory")
	force := fs.Bool("force", false, "re-sync even when the version is unchanged")
	args = reorderArgs(map[string]bool{"registry": true, "dir": true}, args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: lynx-start update <module>[@<version>] [--registry <dir>]")
	}
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
	installed, ok := m.Get(name)
	if !ok {
		return fmt.Errorf("module %s is not installed (see lynx-start add)", name)
	}
	if *registryDir == "" {
		*registryDir = m.Registry
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

	if item.Version == installed.Version && !*force {
		fmt.Printf("✓ %s already at %s (use --force to re-sync the baseline from the registry worktree)\n", name, item.Version)
		return nil
	}
	base, hasBase, err := loadBaselineTree(appDir, name)
	if err != nil {
		return err
	}
	if !hasBase || len(base) == 0 {
		return fmt.Errorf("no update baseline for %s (installed before baselines existed): re-install with `lynx-start add %s -o` to rebuild it (your files are overwritten by upstream; commit local changes first), then update is available again", name, name)
	}

	// upstream 渲染到临时目录：与 add 同一管线（改写导入路径、gofmt、
	// 排除工程文件），保证与应用/基线处于同一形态空间。
	tmp, err := os.MkdirTemp("", "lynx-start-upstream-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := renderUpstream(idx, *registryDir, item, m, tmp); err != nil {
		return err
	}
	other, err := readTree(tmp)
	if err != nil {
		return err
	}
	if len(other) == 0 {
		return fmt.Errorf("registry module %s rendered to an empty tree (item.json broken?)", name)
	}

	// local 树：基线与 upstream 文件并集对应的应用现状（缺 = 用户已删）。
	local := map[string][]byte{}
	for rel := range base {
		local[rel] = nil
	}
	for rel := range other {
		local[rel] = nil
	}
	for rel := range local {
		if content, err := os.ReadFile(filepath.Join(appDir, filepath.FromSlash(rel))); err == nil {
			local[rel] = content
		} else {
			delete(local, rel)
		}
	}

	res, err := merge.Tree(local, base, other)
	if err != nil {
		return err
	}

	// 冲突文件覆盖本地前留备份。
	backup := filepath.Join(appDir, ".lynx-start", "backup", name)
	for _, rel := range res.Conflicts {
		src := filepath.Join(appDir, filepath.FromSlash(rel))
		if content, err := os.ReadFile(src); err == nil {
			dst := filepath.Join(backup, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err == nil {
				_ = os.WriteFile(dst, content, 0o644)
			}
		}
	}
	rels := make([]string, 0, len(res.Updates))
	for rel := range res.Updates {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		target := filepath.Join(appDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, res.Updates[rel], 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}

	// 幂等注入重放：新版模块可能带来新的装配登记/配置段；锚点丢失不阻断
	// 源码更新（此时 add 时也修不好的锚点，交给用户比对 scaffold 模板）。
	if err := injectModulesEntry(appDir, name, m.ModulePath); err != nil {
		fmt.Printf("warn: modules.go injection skipped: %v\n", err)
	}
	if err := appendConfigFragment(appDir, name, item.ConfigFragment); err != nil {
		fmt.Printf("warn: config fragment skipped: %v\n", err)
	}

	files := make([]string, 0, len(other))
	for rel := range other {
		files = append(files, rel)
	}
	sort.Strings(files)
	if err := replaceBaselineTree(appDir, name, other); err != nil {
		return err
	}
	m.Set(manifest.Module{Name: name, Version: item.Version, Files: files})
	if err := m.Save(appDir); err != nil {
		return err
	}

	fmt.Printf("\n✓ %s %s → %s (%d merged, %d unchanged", name, installed.Version, item.Version, len(rels), res.Unchanged)
	if len(res.Conflicts) > 0 {
		fmt.Printf(", %d CONFLICTS", len(res.Conflicts))
	}
	fmt.Println(")")
	for _, rel := range res.Conflicts {
		fmt.Printf("  conflict %s  (markers: yours/baseline/theirs; pre-update copy in .lynx-start/backup/%s/%s)\n", rel, name, rel)
	}
	for _, rel := range res.Removed {
		fmt.Printf("  removed upstream, still present: %s (delete manually if unwanted)\n", rel)
	}
	if len(res.Conflicts) > 0 {
		fmt.Println("  resolve conflicts (search for <<<<<<<), then build: go build ./...")
	} else {
		fmt.Println(`Next steps (order matters):
  go mod tidy            # kernel/module deps may have moved
  mise run generate      # if .proto files changed
  go build ./... && go test ./...`)
	}
	return nil
}

// renderUpstream 用 add 同款管线把模块源码树渲染到 dst（改写导入路径为
// 应用 module path：本模块 + 声明依赖的前缀都拍平到应用路径下）。
func renderUpstream(idx *registry.Index, registryDir string, item *registry.Item, m *manifest.Manifest, dst string) error {
	prefixes := [][]byte{[]byte(item.CanonicalModulePath() + "/")}
	for _, dep := range item.ModuleDependencies {
		depItem, err := idx.Item(registryDir, dep)
		if err != nil {
			return err
		}
		prefixes = append(prefixes, []byte(depItem.CanonicalModulePath()+"/"))
	}
	appPath := []byte(m.ModulePath)
	_, err := render.Tree(item.Dir, dst, render.NewData(m.Name, m.ModulePath), render.Options{
		Overwrite: true,
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
	})
	return err
}

// readTree 把目录读为内存树（斜杠路径 → 内容）。
func readTree(root string) (map[string][]byte, error) {
	tree := map[string][]byte{}
	return tree, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[filepath.ToSlash(rel)] = content
		return nil
	})
}
