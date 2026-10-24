// Package secretkey 提供「主密钥 + purpose 域隔离」的子密钥派生
// （torchwood pkg/jwtparser DeriveKey 的上移）：同一 master 下不同用途
// 派生互不通用——一个用途泄钥不波及其余。页 token 签名、JWT、将来
// cookie/HMAC 文件令牌等全部经此入口。
package secretkey

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"strings"
)

// Derive 以 HMAC-SHA256(master, purpose) 派生 32 字节用途子密钥。
// purpose 必须非空；更换 master 或 purpose 标签即作废该用途全部凭据。
func Derive(master, purpose string) []byte {
	if purpose == "" {
		panic("secretkey: empty purpose")
	}
	mac := hmac.New(sha256.New, []byte(master))
	_, _ = mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

// DeriveHex 是 Derive 的 hex 编码形态（落库/比较场景）。
func DeriveHex(master, purpose string) string {
	const hexdigits = "0123456789abcdef"
	key := Derive(master, purpose)
	out := make([]byte, 0, len(key)*2)
	for _, b := range key {
		out = append(out, hexdigits[b>>4], hexdigits[b&0x0f])
	}
	return string(out)
}

// Verify 是恒定时间的 HMAC 比较助手（防时序侧信道）。
func Verify(key, data, wantMAC []byte) bool {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return hmac.Equal(mac.Sum(nil), wantMAC)
}

// MustValidate 检查 master 可用性（非空白）；组合根启动期调用。
func MustValidate(master string, what string) error {
	if strings.TrimSpace(master) == "" {
		return fmt.Errorf("%s requires a non-empty master secret", what)
	}
	return nil
}
