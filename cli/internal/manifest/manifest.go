// Package manifest 读写应用侧安装清单 lynx-start.json（shadcn components.json
// 的对应物）：记录应用身份与已装模块的版本 pin。
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// 文件名固定；schema v1。
const FileName = "lynx-start.json"

// Module 记录一条已装模块。
type Module struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Manifest 是应用安装清单。
type Manifest struct {
	Schema     int      `json:"schema"`
	Name       string   `json:"name"`
	ModulePath string   `json:"modulePath"`
	Registry   string   `json:"registry"`
	Modules    []Module `json:"modules"`
}

// Load 读取 dir/lynx-start.json。
func Load(dir string) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found: run lynx-start init in the app directory first", FileName)
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", FileName, err)
	}
	return &m, nil
}

// Save 写回 dir/lynx-start.json。
func (m *Manifest) Save(dir string) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(filepath.Join(dir, FileName), raw, 0o644)
}

// Has 报告模块是否已装。
func (m *Manifest) Has(name string) bool {
	for _, mod := range m.Modules {
		if mod.Name == name {
			return true
		}
	}
	return false
}
