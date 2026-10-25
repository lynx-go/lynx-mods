package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/lynx-go/lynx-start/cli/internal/render"
)

// 基线快照（update 三方合并的 base）存应用内 .lynx-start/baseline/<module>/，
// 内容是「最近一次从 registry 同步的最终形态」——已过导入路径改写与 gofmt，
// 与应用文件同处一个路径空间，合并不会产生虚假冲突。目录点开头，
// go build ./... 与 gopls 均不扫描；建议随应用提交（跨机器可 update）。
//
// 不变量：基线 ≡ upstream 最近一次同步形态。冲突文件解决后 baseline 仍为
// upstream 形态（下次 update 把用户的解决结果视为本地改动自然保留）。

func baselineDir(appDir, module string) string {
	return filepath.Join(appDir, ".lynx-start", "baseline", module)
}

// syncBaseline 在 add 后同步基线：本次被 upstream 写入的文件（Created）刷新
// 为应用现状；被跳过的文件仅在基线缺项时补记（重装不覆盖时保留旧基线，
// 不把本地漂移误当作「已同步」）。
func syncBaseline(appDir, module string, res *render.Result) error {
	dir := baselineDir(appDir, module)
	for _, rel := range res.Created {
		if err := copyIntoBaseline(appDir, dir, rel); err != nil {
			return err
		}
	}
	for _, rel := range res.Skipped {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
			continue
		}
		if err := copyIntoBaseline(appDir, dir, rel); err != nil {
			return err
		}
	}
	return nil
}

func copyIntoBaseline(appDir, dir, rel string) error {
	src := filepath.Join(appDir, filepath.FromSlash(rel))
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, content, 0o644)
}

// loadBaselineTree 读基线为内存树（斜杠路径 → 内容）。目录缺失返回空树
// 与 ok=false（update 据此给出「重装建立基线」的可行动提示）。
func loadBaselineTree(appDir, module string) (map[string][]byte, bool, error) {
	dir := baselineDir(appDir, module)
	tree := map[string][]byte{}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
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
	return tree, true, err
}

// replaceBaselineTree 用 upstream 形态整树替换基线并清掉 upstream 已不存在的
// 旧条目（维持「基线 ≡ upstream 最近同步形态」不变量）。
func replaceBaselineTree(appDir, module string, upstream map[string][]byte) error {
	dir := baselineDir(appDir, module)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	rels := make([]string, 0, len(upstream))
	for rel := range upstream {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, upstream[rel], 0o644); err != nil {
			return fmt.Errorf("write baseline %s: %w", rel, err)
		}
	}
	return nil
}
