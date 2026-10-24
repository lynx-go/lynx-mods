// Package bunx 是 bun 集成的标准姿势：连接引导（DSN→*bun.DB）、
// AIP-132 白名单排序解析、泛型列表执行（count + order + 分页 + 领域转换）。
// 过滤（AIP-160）不内置——动态查询语义属于模块，经 queryModifier 注入。
package bunx

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/lynx-go/lynx-start/kernel/crud"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Options 是 OpenDB 的连接选项。
type Options struct {
	MaxOpenConns int // <=0 不设置（驱动默认）
	Ping         bool
}

// OpenDB 从 Postgres DSN 构造 *bun.DB（pgdriver + pgdialect）。
// Ping=true 时探活失败即返回错误（组合根 fail-fast）。
func OpenDB(dsn string, opts Options) (*bun.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("bunx: dsn is empty")
	}
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	if opts.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.Ping {
		if err := sqldb.Ping(); err != nil {
			_ = sqldb.Close()
			return nil, fmt.Errorf("bunx: ping: %w", err)
		}
	}
	return bun.NewDB(sqldb, pgdialect.New()), nil
}

// OrderClause 是一个已解析的排序列：列表达式 + 方向。
type OrderClause struct {
	Expr     string // 白名单映射后的列表达式（含别名，如 "d.created_at"）
	Desc     bool
	RawField string // 原始请求字段名（错误信息用）
}

// ParseOrderBy 解析 AIP-132 排序串（"field desc, other"）到白名单列。
// fields: API 字段名 → SQL 列表达式；未知字段/方向即拒绝（fail-closed）。
func ParseOrderBy(orderBy string, fields map[string]string) ([]OrderClause, error) {
	s := strings.TrimSpace(orderBy)
	if s == "" {
		return nil, nil
	}
	var out []OrderClause
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("order_by: empty clause")
		}
		field, dir := part, "asc"
		if idx := strings.LastIndexAny(part, " \t"); idx >= 0 {
			field, dir = strings.TrimSpace(part[:idx]), strings.TrimSpace(part[idx+1:])
		}
		expr, ok := fields[field]
		if !ok {
			return nil, fmt.Errorf("order_by: unknown field %q", field)
		}
		switch strings.ToLower(dir) {
		case "asc":
		case "desc":
		default:
			return nil, fmt.Errorf("order_by: invalid direction %q", dir)
		}
		out = append(out, OrderClause{Expr: expr, Desc: strings.ToLower(dir) == "desc", RawField: field})
	}
	return out, nil
}

// ApplyOrder 把解析结果应用到查询（等值列自动追加主键决胜，保证分页稳定）。
func ApplyOrder(q *bun.SelectQuery, clauses []OrderClause, tiebreak ...string) *bun.SelectQuery {
	for _, c := range clauses {
		q = q.OrderExpr("? ?", bun.Ident(c.Expr), bun.Safe(directionSQL(c.Desc)))
	}
	for _, t := range tiebreak {
		q = q.OrderExpr("? ASC", bun.Ident(t))
	}
	return q
}

func directionSQL(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}

// ListSpec 描述一次模块列表查询的执行配置。
type ListSpec[M any, D any] struct {
	// Fields 是 API 字段名 → 列表达式白名单（排序用）。
	Fields map[string]string
	// DefaultOrder 是无排序请求时的兜底子句（如 "d.created_at DESC"）；空则必须由调用方保证确定性。
	DefaultOrder string
	// Tiebreak 列（幂等分页的决胜列，如主键）。
	Tiebreak []string
	// Query 在 count/scan 前注入过滤与关联（where/join）。
	Query func(*bun.SelectQuery) *bun.SelectQuery
	// Convert 是 PO → 领域对象转换。
	Convert func(*M) D
}

// Result 是列表执行产物（分页收尾由调用方用 crud.FinalizeOffsetPage 完成，
// 或直接用带 token 的 List）。
type Result[D any] struct {
	Items []D
	Total int
}

// List 执行 count + 排序 + 分页扫描 + 领域转换；params 来自
// crud.ParseListParams（token 一致性与 offset 上限已校验）。
func List[M any, D any](ctx context.Context, db *bun.DB, params crud.ListParams, spec ListSpec[M, D]) (*Result[D], error) {
	base := func() *bun.SelectQuery {
		q := db.NewSelect().Model((*M)(nil))
		if spec.Query != nil {
			q = spec.Query(q)
		}
		return q
	}

	total, err := base().Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("bunx: count: %w", err)
	}

	q := base()
	clauses, err := ParseOrderBy(params.OrderBy, spec.Fields)
	if err != nil {
		return nil, err
	}
	if len(clauses) > 0 {
		q = ApplyOrder(q, clauses, spec.Tiebreak...)
	} else if spec.DefaultOrder != "" {
		q = q.OrderExpr("? ", bun.Safe(spec.DefaultOrder))
		for _, t := range spec.Tiebreak {
			q = q.OrderExpr("? ASC", bun.Ident(t))
		}
	}
	if params.Offset > 0 {
		q = q.Offset(int64(params.Offset))
	}
	if params.PageSize > 0 {
		q = q.Limit(int64(params.PageSize))
	}

	var models []M
	if err := q.Scan(ctx, &models); err != nil {
		return nil, fmt.Errorf("bunx: scan: %w", err)
	}
	out := &Result[D]{Items: make([]D, 0, len(models)), Total: int(total)}
	for i := range models {
		out.Items = append(out.Items, spec.Convert(&models[i]))
	}
	return out, nil
}

// SortedFields 返回白名单字段名（错误信息/文档生成用，稳定排序）。
func SortedFields(fields map[string]string) []string {
	out := make([]string, 0, len(fields))
	for f := range fields {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
