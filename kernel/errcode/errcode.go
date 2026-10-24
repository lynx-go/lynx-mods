// Package errcode 实现按模块分配的错误码区段（ADR-0004 契约⑤）。
//
// 区段的形态是 gRPC ErrorInfo 的 Domain/Reason 二元组：Domain=模块名，
// Reason=模块内错误名（UPPER_SNAKE）。Kernel 登记区段防跨模块撞名；模块
// 在自己的 domain 包里定义哨兵错误，app 层转 grpc/status 时附带 ErrorInfo
// （torchwood P3-18 取舍沿用：app 层允许直接使用 grpc/status）。
package errcode

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Info 是一条错误码的结构化信息，映射 google.rpc.ErrorInfo。
type Info struct {
	Domain  string // 模块名（区段）
	Reason  string // 模块内错误名，UPPER_SNAKE
	Message string // 人类可读说明（可选）
}

// Canonical 返回 "<DOMAIN>/<REASON>" 规范串（日志与断言用）。
func (i Info) Canonical() string {
	return i.Domain + "/" + i.Reason
}

// Error 实现 error：区段错误可直接当哨兵错误用。
func (i Info) Error() string {
	if i.Message != "" {
		return fmt.Sprintf("%s: %s", i.Canonical(), i.Message)
	}
	return i.Canonical()
}

var (
	mu      sync.Mutex
	spaces  = map[string]*Space{}
	codesMu sync.Mutex
	codes   = map[string]Info{}
)

// Space 是一个模块的错误码区段。
type Space struct {
	domain string
}

// Register 登记一个区段（模块名）；重名注册是编程错误，返回 error 由组合根
// fail-fast。安装器写入 item.json 的 errorRange 与此对账。
func Register(domain string) (*Space, error) {
	if domain == "" {
		return nil, fmt.Errorf("errcode: domain is empty")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := spaces[domain]; dup {
		return nil, fmt.Errorf("errcode: duplicate domain %q", domain)
	}
	s := &Space{domain: domain}
	spaces[domain] = s
	return s, nil
}

// MustRegister 是 Register 的 panic 版（模块包初始化期使用）。
func MustRegister(domain string) *Space {
	s, err := Register(domain)
	if err != nil {
		panic(err)
	}
	return s
}

// New 在区段内定义一条错误码并全局登记（同名同域重复定义是编程错误）。
func (s *Space) New(reason, message string) Info {
	if reason == "" {
		panic(fmt.Sprintf("errcode: empty reason in domain %q", s.domain))
	}
	up := strings.ToUpper(reason)
	codesMu.Lock()
	defer codesMu.Unlock()
	info := Info{Domain: s.domain, Reason: up, Message: message}
	if prev, dup := codes[info.Canonical()]; dup {
		panic(fmt.Sprintf("errcode: duplicate code %q (prev: %q)", info.Canonical(), prev.Error()))
	}
	codes[info.Canonical()] = info
	return info
}

// Domains 返回已登记的全部区段名（doctor 展示用）。
func Domains() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, 0, len(spaces))
	for d := range spaces {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Lookup 按规范串查错误码定义（guard 测试对账用）。
func Lookup(reason string) (Info, bool) {
	codesMu.Lock()
	defer codesMu.Unlock()
	i, ok := codes[reason]
	return i, ok
}
