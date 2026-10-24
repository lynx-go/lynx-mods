// Package testutil 是模块与应用共享的测试基座（骨架，随首批模块充实）：
// 数据库 DSN 读取与按需跳过。设计取向：集成测试直连真实 Postgres（DSN 由
// 环境变量提供），不内置 docker 编排——应用侧 mise 任务负责起依赖。
package testutil

import (
	"os"
	"testing"
)

// EnvPgDSN 是集成测试 Postgres DSN 的环境变量名。
const EnvPgDSN = "LYNXSTART_TEST_PG_DSN"

// PgDSN 返回测试数据库 DSN；未设置时返回空串。
func PgDSN() string { return os.Getenv(EnvPgDSN) }

// SkipUnlessPg 无 DSN 时跳过测试（t.Skip 而非 Fail：本地无依赖属常态）。
func SkipUnlessPg(t testing.TB) string {
	t.Helper()
	dsn := PgDSN()
	if dsn == "" {
		t.Skipf("%s not set; skipping database test", EnvPgDSN)
	}
	return dsn
}
