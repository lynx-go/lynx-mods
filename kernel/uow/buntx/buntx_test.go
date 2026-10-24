package buntx

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

// zeroTx 是零值 bun.Tx：传播逻辑只做类型断言，不触库。
func zeroTx() bun.Tx { return bun.Tx{} }

func TestConnSelectsTxFromContext(t *testing.T) {
	ctx := context.Background()
	var db *bun.DB

	if got := Conn(ctx, db); got != bun.IDB(db) {
		t.Error("no tx in ctx: Conn should return root db")
	}
	txCtx := WithTx(ctx, zeroTx())
	if _, ok := Conn(txCtx, db).(bun.Tx); !ok {
		t.Error("tx in ctx: Conn should return the tx")
	}
	if !InTx(txCtx) || InTx(ctx) {
		t.Error("InTx mismatch")
	}
}
