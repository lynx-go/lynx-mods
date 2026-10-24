package kernel

import (
	"io/fs"
	"testing"
)

func mod(name string, deps ...string) Module {
	return Module{Name: name, DependsOn: deps}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"demo", "identity", "my-module", "m1"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "Demo", "my_module", "mod.x", "模块"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", bad)
		}
	}
}

func TestValidateModules(t *testing.T) {
	if err := ValidateModules([]Module{mod("a"), mod("b", "a"), mod("c", "b")}); err != nil {
		t.Fatalf("valid DAG: %v", err)
	}
	if err := ValidateModules([]Module{mod("a"), mod("a")}); err == nil {
		t.Error("duplicate name: want error")
	}
	if err := ValidateModules([]Module{mod("a", "missing")}); err == nil {
		t.Error("missing dep: want error")
	}
	if err := ValidateModules([]Module{mod("a", "b"), mod("b", "a")}); err == nil {
		t.Error("cycle: want error")
	}
	if err := ValidateModules([]Module{mod("a"), mod("b", "a"), mod("c", "b"), mod("d", "a")}); err != nil {
		t.Errorf("complex DAG: %v", err)
	}
}

func TestConfigSpecKeyOrName(t *testing.T) {
	if got := (ConfigSpec{}).KeyOrName("demo"); got != "demo" {
		t.Errorf("default key = %q, want demo", got)
	}
	if got := (ConfigSpec{Key: "custom"}).KeyOrName("demo"); got != "custom" {
		t.Errorf("explicit key = %q, want custom", got)
	}
}

func TestPolicyFiles(t *testing.T) {
	mods := []Module{
		{Name: "a", Policies: nil},
		{Name: "b"},
	}
	if got := len(PolicyFiles(mods)); got != 0 {
		t.Errorf("empty policies: got %d, want 0", got)
	}
	_ = fs.FS(nil) // Migrations 类型存在性编译检查
}
