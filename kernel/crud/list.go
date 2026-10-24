package crud

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// DefaultPageSize 是列表页大小默认值。
	DefaultPageSize = 50

	// MaxPageSize 是列表页大小上限。
	MaxPageSize = 1000

	// DefaultPageSizeNoToken 是无 page_token 时的默认页大小。
	DefaultPageSizeNoToken = 50
)

// ListParams 是 AIP-132/158 列表请求的解析产物。Filter/OrderBy 作为 token
// 绑定通道保留（跨页一致性校验）；解析与白名单校验由模块自己的列表端口
// 负责（动态查询语义属于模块，不属于 Kernel）。
type ListParams struct {
	PageSize  int32
	PageToken string
	Filter    string
	OrderBy   string
	Offset    int
}

// ParseListParams 解析并校验列表参数（AIP-158）：页大小钳制、token 签名验证、
// offset 上限、order_by/filter 与签发该 token 的请求一致。
func ParseListParams(pageSize int32, pageToken, filter, orderBy string) (ListParams, error) {
	params := ListParams{
		PageSize:  pageSize,
		PageToken: pageToken,
		Filter:    filter,
		OrderBy:   orderBy,
	}
	if params.PageSize <= 0 {
		if params.PageToken == "" {
			params.PageSize = DefaultPageSizeNoToken
		} else {
			params.PageSize = DefaultPageSize
		}
	}
	if params.PageSize > MaxPageSize {
		params.PageSize = MaxPageSize
	}
	if params.PageToken != "" {
		data, err := DecodePageTokenFull(params.PageToken)
		if err != nil {
			return ListParams{}, fmt.Errorf("invalid page_token: %w", err)
		}
		canonicalOrderBy := strings.TrimSpace(strings.ToLower(orderBy))
		if data.OrderBy != "" && strings.TrimSpace(strings.ToLower(data.OrderBy)) != canonicalOrderBy {
			return ListParams{}, fmt.Errorf("order_by must match the original request when using page_token")
		}
		if data.FilterDigest != "" && data.FilterDigest != FilterDigest(filter) {
			return ListParams{}, fmt.Errorf("filter must match the original request when using page_token")
		}
		if data.Offset > MaxQueryOffset {
			return ListParams{}, fmt.Errorf("page_token offset exceeds the maximum of %d", MaxQueryOffset)
		}
		params.Offset = data.Offset
	}
	return params, nil
}

// OffsetPage 是一次 offset 分页查询的收尾面：终止契约与双向 token 的单点裁决。
type OffsetPage struct {
	PageSize   int32
	TotalCount int    // <0 视为未知，不投影
	NextToken  string // 空 = 无下一页
	PrevToken  string // 空 = 无上一页
}

// FinalizeOffsetPage 对一页结果做分页收尾：裁决终止契约并编码双向 token。
// returned 是本页实际行数；total 未知传 -1（此时不发 next token）。
func FinalizeOffsetPage(params ListParams, total, returned int) (OffsetPage, error) {
	page := OffsetPage{PageSize: params.PageSize, TotalCount: total}
	if total >= 0 && params.Offset+returned < total {
		token, err := EncodePageToken(params.Offset + int(params.PageSize))
		if err != nil {
			return OffsetPage{}, err
		}
		page.NextToken = token
	}
	if params.Offset > 0 {
		prev := params.Offset - int(params.PageSize)
		if prev < 0 {
			prev = 0
		}
		token, err := EncodePageToken(prev)
		if err != nil {
			return OffsetPage{}, err
		}
		page.PrevToken = token
	}
	return page, nil
}

// SliceOffsetPage 在内存全量列表上切页并收尾（小表枚举端点的形状）。
func SliceOffsetPage[T any](items []T, params ListParams) ([]T, OffsetPage, error) {
	start := params.Offset
	if start > len(items) {
		start = len(items)
	}
	end := start + int(params.PageSize)
	if end > len(items) {
		end = len(items)
	}
	page, err := FinalizeOffsetPage(params, len(items), end-start)
	if err != nil {
		return nil, OffsetPage{}, err
	}
	return items[start:end], page, nil
}

// FilterDigest 返回 filter 表达式的确定性摘要（token 绑定）：归一化原文哈希。
func FilterDigest(filter string) string {
	normalized := strings.TrimSpace(strings.ToLower(filter))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
