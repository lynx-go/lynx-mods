// Package registry 读取 lynx-start 仓库的模块仓：registry/registry.json 索引 +
// 各模块目录的 item.json 清单（MVP 为本地目录形态；远端 git registry 后续接入）。
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	// 形态维护，自带 go.mod 与 genproto）；安装时 lyx add 把 .go/.proto 中
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
		return nil, fmt.Errorf("读取 registry：lyx init/add 需要 --registry 指向 lynx-start 仓库目录（含 registry/registry.json）: %w", err)
	}
	var idx Index
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// Item 按名查模块：定位 item.json 与源码目录。
func (idx *Index) Item(registryDir, name string) (*Item, error) {
	for _, it := range idx.Items {
		if it.Name != name {
			continue
		}
		dir := filepath.Join(registryDir, it.Path)
		raw, err := os.ReadFile(filepath.Join(dir, "item.json"))
		if err != nil {
			return nil, fmt.Errorf("读取 %s/item.json: %w", it.Path, err)
		}
		var item Item
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		item.Dir = dir
		if item.Name == "" {
			item.Name = name
		}
		return &item, nil
	}
	return nil, fmt.Errorf("registry 中不存在模块 %q（lyx list 查看）", name)
}
