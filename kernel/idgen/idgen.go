// Package idgen 提供随机 ID 生成（crypto/rand，零外部依赖）：
//   - New()        128bit hex（32 字符），通用实体 ID
//   - Prefixed(p)  "<p>_<128bit hex>"（usr_3f9a…），前缀自描述、便于日志排查
//   - Secret(n)    n 字节随机 hex（≥32 字节建议用于密钥材料）
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// prefixRe 约束前缀形态：小写字母开头，小写字母/数字/下划线。
var prefixRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// New 返回 128bit 随机 hex ID。
func New() string { return randomHex(16) }

// Prefixed 返回 "<prefix>_<128bit hex>"。
func Prefixed(prefix string) string {
	if !prefixRe.MatchString(prefix) {
		panic(fmt.Sprintf("idgen: invalid prefix %q (^[a-z][a-z0-9_]*$)", prefix))
	}
	return prefix + "_" + randomHex(16)
}

// Secret 返回 n 字节随机数的 hex（2n 字符）；n<1 或 >256 是编程错误。
func Secret(n int) string {
	if n < 1 || n > 256 {
		panic(fmt.Sprintf("idgen: Secret(%d) out of range [1,256]", n))
	}
	return randomHex(n)
}

func randomHex(nbytes int) string {
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		panic("idgen: rand: " + err.Error())
	}
	return strings.ToLower(hex.EncodeToString(b))
}
