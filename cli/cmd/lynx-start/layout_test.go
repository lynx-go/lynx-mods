package main

import (
	"strings"
	"testing"
)

// TestRelocateModule 断言布局映射（ADR-0006：装入 modules/<m>/ 并拍平尾段）。
func TestRelocateModule(t *testing.T) {
	cases := map[string]string{
		"domain/identity/user.go":                  "modules/identity/domain/user.go",
		"internal/app/identity/identity.go":        "modules/identity/internal/app/identity.go",
		"internal/api/identity/module.go":          "modules/identity/internal/api/module.go",
		"internal/infra/identity/repo.go":          "modules/identity/internal/infra/repo.go",
		"internal/infra/identity/migrations/x.sql": "modules/identity/internal/infra/migrations/x.sql",
		"proto/identity/v1/identity.proto":         "proto/identity/v1/identity.proto",
		"proto/grpcapi/v1/policy.proto":            "proto/grpcapi/v1/policy.proto",
	}
	for in, want := range cases {
		got, err := relocateModule("identity", in)
		if err != nil {
			t.Fatalf("relocate %s: %v", in, err)
		}
		if got != want {
			t.Errorf("relocate(%s) = %s, want %s", in, got, want)
		}
	}
	// 布局约定外的文件 fail-closed。
	for _, bad := range []string{"README.md", "cmd/x/main.go", "domain/other/file.go"} {
		if _, err := relocateModule("identity", bad); err == nil {
			t.Errorf("relocate(%s) should fail (outside layout contract)", bad)
		}
	}
}

// TestModuleRewriter 断言五条结构化改写（含 genproto 与跨模块依赖）。
func TestModuleRewriter(t *testing.T) {
	canonicals := map[string]string{
		"session":  "github.com/lynx-go/lynx-start/modules/session",
		"identity": "github.com/lynx-go/lynx-start/modules/identity",
	}
	rw := moduleRewriter("example.com/app", canonicals)
	goSrc := `package x

import (
	"github.com/lynx-go/lynx-start/modules/session/domain/session"
	appsession "github.com/lynx-go/lynx-start/modules/session/internal/app/session"
	sessionv1 "github.com/lynx-go/lynx-start/modules/session/genproto/session/v1"
	"github.com/lynx-go/lynx-start/modules/identity/domain/identity"
)

var _ = session.X
var _ = appsession.Y
var _ = sessionv1.Z
var _ = identity.User{}
`
	got := string(rw("internal/app/session/x.go", []byte(goSrc)))
	// 尾段拍平：canonical …/domain/session → app modules/session/domain
	//（genproto 保留 <m>/v1——buf 输出相对路径决定）。
	for _, want := range []string{
		"\"example.com/app/modules/session/domain\"",
		"appsession \"example.com/app/modules/session/internal/app\"",
		"sessionv1 \"example.com/app/modules/session/genproto/session/v1\"",
		"\"example.com/app/modules/identity/domain\"",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewrite missing %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, canonicalModulesPrefix) {
		t.Errorf("canonical prefix survived:\n%s", got)
	}

	protoSrc := `option go_package = "github.com/lynx-go/lynx-start/modules/session/genproto/session/v1;sessionv1";`
	got = string(rw("proto/session/v1/session.proto", []byte(protoSrc)))
	if !strings.Contains(got, `"example.com/app/modules/session/genproto/session/v1;sessionv1"`) {
		t.Errorf("go_package rewrite wrong: %s", got)
	}
}

// TestCanonicalResidue 残留扫描只在带引号的 import/go_package 形态命中
// （注释里裸写前缀不误报）。
func TestCanonicalResidue(t *testing.T) {
	if _, bad := canonicalResidue("a.go", []byte("// see "+canonicalModulesPrefix+"foo")); bad {
		t.Error("bare comment mention must not trip the residue check")
	}
	if _, bad := canonicalResidue("a.go", []byte(`import "`+canonicalModulesPrefix+`foo/bar"`)); !bad {
		t.Error("quoted import residue must be detected")
	}
	if _, bad := canonicalResidue("a.proto", []byte(`option go_package = "`+canonicalModulesPrefix+`x/y";`)); !bad {
		t.Error("quoted go_package residue must be detected")
	}
}

// TestBridgeModule 桥接包编译形态：转发 Module/Assemble、显式别名。
func TestBridgeModule(t *testing.T) {
	got := string(bridgeModule("example.com/app", "identity"))
	for _, want := range []string{
		"package identity",
		`api "example.com/app/modules/identity/internal/api"`,
		"func Module() kernel.Module { return api.Module() }",
		"func Assemble(deps kernel.Deps) (kernel.Assembly, error) { return api.Assemble(deps) }",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("bridge missing %q:\n%s", want, got)
		}
	}
}

// TestBaselineCacheDir 缓存寻址：<appModulePath>/<m>@<commit12>。
func TestBaselineCacheDir(t *testing.T) {
	got := baselineCacheDir("example.com/app", "identity", "0123456789abcdef0123456789abcdef")
	if !strings.Contains(got, "example.com/app") || !strings.HasSuffix(got, "identity@0123456789ab") {
		t.Errorf("baselineCacheDir = %s", got)
	}
}
