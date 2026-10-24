// Package clock 提供时间源抽象：domain/app 层依赖 Clock 而非 time.Now，
// 测试注入 Fake 控制时间（过期、轮换窗口、时序边界）。
package clock

import "time"

// Clock 是时间源端口。
type Clock interface {
	Now() time.Time
}

// System 是生产实现（UTC）。
type System struct{}

// Now 实现 Clock。
func (System) Now() time.Time { return time.Now().UTC() }

// Fake 是测试实现：可设定、可推进。
type Fake struct{ t time.Time }

// NewFake 构造固定起点（UTC 归一）。
func NewFake(start time.Time) *Fake { return &Fake{t: start.UTC()} }

// Now 实现 Clock。
func (f *Fake) Now() time.Time { return f.t }

// Set 跳到指定时刻。
func (f *Fake) Set(t time.Time) { f.t = t.UTC() }

// Advance 推进时长。
func (f *Fake) Advance(d time.Duration) { f.t = f.t.Add(d) }
