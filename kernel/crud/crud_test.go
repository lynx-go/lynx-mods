package crud

import (
	"errors"
	"testing"
	"time"
)

func TestPageTokenRoundtripNoSigning(t *testing.T) {
	tok, err := EncodePageToken(120)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	off, err := DecodePageToken(tok)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if off != 120 {
		t.Errorf("offset = %d, want 120", off)
	}
	if _, err := DecodePageToken("v1:120"); err == nil {
		t.Error("legacy simple format: want error")
	}
}

func TestPageTokenSigning(t *testing.T) {
	if err := InitPageTokenSigning("test-master-secret-0123456789"); err != nil {
		t.Fatalf("init signing: %v", err)
	}
	if !PageTokenSigningEnabled() {
		t.Fatal("signing should be enabled")
	}

	tok, err := EncodePageToken(40)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if off, err := DecodePageToken(tok); err != nil || off != 40 {
		t.Fatalf("decode signed: off=%d err=%v", off, err)
	}

	// 篡改（换个合法 token 的签名）必须被拒。
	tampered, err := DecodePageTokenFull(tok)
	if err != nil {
		t.Fatalf("decode full: %v", err)
	}
	other, err := EncodePageToken(41)
	if err != nil {
		t.Fatalf("encode other: %v", err)
	}
	otherData, err := DecodePageTokenFull(other)
	if err != nil {
		t.Fatalf("decode other: %v", err)
	}
	tampered.Sig = otherData.Sig
	if _, err := decodeTokenData(encodeRaw(t, *tampered)); err == nil {
		_ = err // decodeTokenData 只解 base64，验签在下一步
	}
	if _, err := DecodePageToken(encodeRaw(t, *tampered)); err == nil {
		t.Error("tampered token: want error")
	}

	// 无签名 token 在已启用签名的进程被拒（fail-closed）。
	plain := PageTokenData{Version: PageTokenVersion, Mode: TokenModeLegacy, Offset: 7, Created: time.Now().UTC()}
	if _, err := DecodePageToken(encodeRaw(t, plain)); err == nil {
		t.Error("unsigned token under signing: want error")
	}

	// 过期 token 被拒。
	expired := PageTokenData{Version: PageTokenVersion, Mode: TokenModeLegacy, Offset: 7,
		Created: time.Now().UTC().Add(-2 * DefaultTokenTTL)}
	if err := signForTest(&expired); err != nil {
		t.Fatalf("sign expired: %v", err)
	}
	if _, err := DecodePageToken(encodeRaw(t, expired)); err == nil {
		t.Error("expired token: want error")
	}
}

func encodeRaw(t *testing.T, d PageTokenData) string {
	t.Helper()
	s, err := encodeTokenData(d)
	if err != nil {
		t.Fatalf("encodeTokenData: %v", err)
	}
	return s
}

func signForTest(d *PageTokenData) error {
	key := pageTokenKey()
	sig, err := SignPageToken(d, key)
	if err != nil {
		return err
	}
	d.Sig = sig
	return nil
}

func TestParseListParams(t *testing.T) {
	p, err := ParseListParams(0, "", "", "")
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if p.PageSize != DefaultPageSizeNoToken {
		t.Errorf("default page size = %d", p.PageSize)
	}

	p, err = ParseListParams(5000, "", "", "")
	if err != nil {
		t.Fatalf("oversize: %v", err)
	}
	if p.PageSize != MaxPageSize {
		t.Errorf("clamped page size = %d, want %d", p.PageSize, MaxPageSize)
	}

	tok, err := EncodePageToken(60)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// token 不携带 digest 时（EncodePageToken 现状）不做绑定校验；
	// 携带 digest 的 token 由 DecodePageTokenFull 路径校验（上方签名测试覆盖）。
	if _, err := ParseListParams(10, tok, "a=b", ""); err != nil {
		t.Errorf("token without digest should accept filter: %v", err)
	}
}

func TestFilterDigest(t *testing.T) {
	if FilterDigest("") != "" {
		t.Error("empty filter digest should be empty")
	}
	if FilterDigest("A = B") != FilterDigest("a = b") {
		t.Error("digest should be case-insensitive-stable")
	}
}

func TestFinalizeOffsetPage(t *testing.T) {
	params := ListParams{PageSize: 10, Offset: 0}
	page, err := FinalizeOffsetPage(params, 25, 10)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if page.NextToken == "" {
		t.Error("next token expected")
	}
	if page.PrevToken != "" {
		t.Error("first page: no prev token")
	}

	params.Offset = 10
	page, err = FinalizeOffsetPage(params, 20, 10)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if page.NextToken != "" {
		t.Error("full page reaching total: no next token (termination contract)")
	}
	if page.PrevToken == "" {
		t.Error("prev token expected")
	}

	// total 未知（-1）：不发 next token。
	page, err = FinalizeOffsetPage(ListParams{PageSize: 10}, -1, 10)
	if err != nil {
		t.Fatalf("finalize unknown total: %v", err)
	}
	if page.NextToken != "" {
		t.Error("unknown total: no next token")
	}
}

func TestSliceOffsetPage(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	slice, page, err := SliceOffsetPage(items, ListParams{PageSize: 2, Offset: 1})
	if err != nil {
		t.Fatalf("slice: %v", err)
	}
	if len(slice) != 2 || slice[0] != 2 || slice[1] != 3 {
		t.Errorf("slice = %v", slice)
	}
	if page.NextToken == "" || page.TotalCount != 5 {
		t.Errorf("page = %+v", page)
	}
}

func TestTimeCursor(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 123456789, time.UTC)
	tok := EncodeTimeCursor(TimeCursor{Time: now, Tiebreak: "id-7"})
	c, err := DecodeTimeCursor(tok)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if c.Ascending || !c.Time.Equal(now) || c.Tiebreak != "id-7" {
		t.Errorf("roundtrip = %+v", c)
	}

	if _, err := DecodeTimeCursorDirection(tok, true); !errors.Is(err, ErrCursorDirection) {
		t.Errorf("direction mismatch: err = %v, want ErrCursorDirection", err)
	}
	if c, err := DecodeTimeCursorDirection(tok, false); err != nil || c.Tiebreak != "id-7" {
		t.Errorf("direction match: c=%+v err=%v", c, err)
	}
	if c, err := DecodeTimeCursorDirection("", true); err != nil || !c.Time.IsZero() {
		t.Errorf("empty token: c=%+v err=%v", c, err)
	}

	asc := EncodeTimeCursor(TimeCursor{Time: now, Ascending: true})
	if c, err := DecodeTimeCursor(asc); err != nil || !c.Ascending || c.Tiebreak != "" {
		t.Errorf("asc roundtrip = %+v err=%v", c, err)
	}
}
