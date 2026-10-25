// Package registry 读取 lynx-start 仓库的模块仓：registry/registry.json 索引 +
// 各模块目录的 item.json 清单。版本解析走仓库 git tag（modules/<name>/v<x.y.z>，
// 与 kernel/v<x.y.z> 同款子目录前缀形式）；远端 registry 后续接入。
package registry

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Index 是 registry/registry.json。
type Index struct {
	Name  string `json:"name"`
	Items []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"items"`
}

// Item 是模块目录里的 item.json（registry item 清单）。
type Item struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	// ModulePath 是模块源码的规范 Go module path（模块在本仓内以真实代码
	// 形态维护，自带 go.mod 与 genproto）；安装时 lynx-start add 把 .go/.proto 中
	// 的该路径改写为应用 module path。
	ModulePath         string   `json:"modulePath"`
	ModuleDependencies []string `json:"moduleDependencies"`
	GoDependencies     []string `json:"goDependencies"`
	ConfigFragment     string   `json:"configFragment"`
	PublishedEvents    []string `json:"publishedEvents"`
	// Dir 是模块源码模板目录（加载时填充）。
	Dir string `json:"-"`
}

// CanonicalModulePath 返回模块的规范路径（缺省推导）。
func (i *Item) CanonicalModulePath() string {
	if i.ModulePath != "" {
		return i.ModulePath
	}
	return "github.com/lynx-go/lynx-start/modules/" + i.Name
}

// Load 读 registryDir/registry/registry.json。
func Load(registryDir string) (*Index, error) {
	raw, err := os.ReadFile(filepath.Join(registryDir, "registry", "registry.json"))
	if err != nil {
		return nil, fmt.Errorf("load registry: lynx-start init/add need --registry pointing at the lynx-start repo directory (containing registry/registry.json): %w", err)
	}
	var idx Index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// Item 按名查模块：定位 item.json 与源码目录（registry 工作区最新形态）。
func (idx *Index) Item(registryDir, name string) (*Item, error) {
	path, err := idx.modulePath(name)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(registryDir, path)
	item, err := loadItem(dir, name)
	if err != nil {
		return nil, err
	}
	item.Dir = dir
	return item, nil
}

// ItemAt 按名与可选版本查模块：version 为空等价 Item（工作区最新）；
// 非空则从 git tag modules/<name>/v<version> 提取该版本源码树。
// 返回的 cleanup 释放临时目录，调用方必须 defer（工作区形态时为 nil）。
// tag 内 item.json 的 version 必须与请求一致（防 tag 与清单漂移）。
func (idx *Index) ItemAt(registryDir, name, version string) (*Item, func(), error) {
	if version == "" {
		item, err := idx.Item(registryDir, name)
		return item, func() {}, err
	}
	path, err := idx.modulePath(name)
	if err != nil {
		return nil, nil, err
	}
	tag := fmt.Sprintf("modules/%s/v%s", name, version)
	if err := exec.Command("git", "-C", registryDir, "rev-parse", "--verify", "--quiet", tag+"^{commit}").Run(); err != nil {
		return nil, nil, fmt.Errorf("module %s@%s not found: no git tag %s in the registry repo (available: git -C %s tag -l 'modules/%s/*')",
			name, version, tag, registryDir, name)
	}
	raw, err := exec.Command("git", "-C", registryDir, "archive", "--format=tar", tag, "--", path).Output()
	if err != nil {
		return nil, nil, fmt.Errorf("git archive %s: %w", tag, err)
	}
	tmp, err := os.MkdirTemp("", "lynx-start-registry-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	if err := untar(tmp, bytes.NewReader(raw)); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("extract %s: %w", tag, err)
	}
	dir := filepath.Join(tmp, path)
	item, err := loadItem(dir, name)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	if item.Version != version {
		cleanup()
		return nil, nil, fmt.Errorf("tag %s carries item.json version %q (tag/manifest drift): fix the tag or the manifest", tag, item.Version)
	}
	item.Dir = dir
	return item, cleanup, nil
}

func (idx *Index) modulePath(name string) (string, error) {
	for _, it := range idx.Items {
		if it.Name == name {
			return it.Path, nil
		}
	}
	return "", fmt.Errorf("module %q not found in registry (see lynx-start list)", name)
}

func loadItem(dir, name string) (*Item, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "item.json"))
	if err != nil {
		return nil, fmt.Errorf("read item.json: %w", err)
	}
	var item Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	if item.Name == "" {
		item.Name = name
	}
	return &item, nil
}

// untar 解 git archive 生成的 tar 流（仅文件与目录，拒绝异常路径）。
func untar(dst string, r io.Reader) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if filepath.IsAbs(name) || strings.HasPrefix(name, "..") {
			return fmt.Errorf("refusing archive entry %q", hdr.Name)
		}
		target := filepath.Join(dst, name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			// pax 扩展头（git archive 的元数据条目），忽略。
		default:
			return fmt.Errorf("unsupported archive entry %q (type %c)", hdr.Name, hdr.Typeflag)
		}
	}
}
