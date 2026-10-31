package merge

import (
	"os/exec"
	"strings"
	"testing"
)

func gitAvailable(t *testing.T) {
	t.Helper()
	if err := exec.Command("git", "merge-file", "--help").Run(); err != nil {
		t.Skip("git not available")
	}
}

func TestFileCleanMerge(t *testing.T) {
	gitAvailable(t)
	base := "line1\nline2\nline3\nline4\n"
	local := "LOCAL-EDIT\nline2\nline3\nline4\n"    // 改第一行
	other := "line1\nline2\nline3\nUPSTREAM-EDIT\n" // 改最后一行
	merged, n, err := File([]byte(local), []byte(base), []byte(other))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 conflicts, got %d in:\n%s", n, merged)
	}
	want := "LOCAL-EDIT\nline2\nline3\nUPSTREAM-EDIT\n"
	if string(merged) != want {
		t.Fatalf("merged = %q, want %q", merged, want)
	}
}

func TestFileConflict(t *testing.T) {
	gitAvailable(t)
	base := "same\n"
	local := "yours-change\n"
	other := "theirs-change\n"
	merged, n, err := File([]byte(local), []byte(base), []byte(other))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if n == 0 {
		t.Fatal("want conflicts")
	}
	for _, marker := range []string{"<<<<<<< yours", "=======", ">>>>>>> theirs"} {
		if !strings.Contains(string(merged), marker) {
			t.Fatalf("merged output missing %q:\n%s", marker, merged)
		}
	}
}

func TestTree(t *testing.T) {
	gitAvailable(t)
	base := map[string][]byte{
		"unchanged.go":  []byte("package x\n"),
		"local-edit.go": []byte("a\nb\nc\nd\n"),
		"up-deletes.go": []byte("gone\n"),
		// upstream 删除 + 本地改动：必须保守提示，不自动删。
		"up-deletes-local-touched.go": []byte("keep-me\n"),
	}
	local := map[string][]byte{
		"unchanged.go":                []byte("package x\n"),
		"local-edit.go":               []byte("A\nb\nc\nd\n"), // 改首行
		"up-deletes.go":               []byte("gone\n"),
		"up-deletes-local-touched.go": []byte("KEEP-ME\n"),
		"mine-only.go":                []byte("mine\n"), // 用户自建，不碰
	}
	other := map[string][]byte{
		"unchanged.go":  []byte("package x\n"),
		"local-edit.go": []byte("a\nb\nc\nD\n"), // 改末行
		"new-file.go":   []byte("package y\n"),
	}
	res, err := Tree(local, base, other)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("want no conflicts, got %v", res.Conflicts)
	}
	if got := string(res.Updates["local-edit.go"]); got != "A\nb\nc\nD\n" {
		t.Fatalf("local-edit.go = %q, want %q", got, "A\nb\nc\nD\n")
	}
	if got := string(res.Updates["new-file.go"]); got != "package y\n" {
		t.Fatalf("new-file.go = %q", got)
	}
	// upstream 删除且本地未改 → 安全删除（vendored policy.proto 弃用场景）。
	if len(res.Deleted) != 1 || res.Deleted[0] != "up-deletes.go" {
		t.Fatalf("Deleted = %v", res.Deleted)
	}
	// upstream 删除但本地改过 → 保守提示。
	if len(res.Removed) != 1 || res.Removed[0] != "up-deletes-local-touched.go" {
		t.Fatalf("Removed = %v", res.Removed)
	}
	if _, touched := res.Updates["mine-only.go"]; touched {
		t.Fatal("user-only file must not be touched")
	}
	if _, touched := res.Updates["unchanged.go"]; touched {
		t.Fatal("unchanged file must not be rewritten")
	}
}

func TestTreeSameLineConflict(t *testing.T) {
	gitAvailable(t)
	base := map[string][]byte{"f.go": []byte("v1\n")}
	local := map[string][]byte{"f.go": []byte("v2-local\n")}
	other := map[string][]byte{"f.go": []byte("v2-upstream\n")}
	res, err := Tree(local, base, other)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "f.go" {
		t.Fatalf("Conflicts = %v", res.Conflicts)
	}
	if !strings.Contains(string(res.Updates["f.go"]), "<<<<<<< yours") {
		t.Fatalf("conflict file missing markers:\n%s", res.Updates["f.go"])
	}
}

func TestTreeUserCreatedCollidesWithUpstreamNew(t *testing.T) {
	gitAvailable(t)
	res, err := Tree(
		map[string][]byte{"n.go": []byte("mine\n")},
		map[string][]byte{},
		map[string][]byte{"n.go": []byte("theirs\n")},
	)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if len(res.Conflicts) != 1 {
		t.Fatalf("want collision conflict, got %v", res.Conflicts)
	}
}
