package audit

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lynx-go/lynx-start/kernel/clock"
	"github.com/lynx-go/lynx-start/kernel/crud"
	"github.com/lynx-go/lynx-start/kernel/events"
	domainaudit "github.com/lynx-go/lynx-start/modules/audit/domain/audit"
)

// fakeRepo 记录调用供断言（List 固定返回一页）。
type fakeRepo struct {
	mu       sync.Mutex
	appended []*domainaudit.Record
}

func (f *fakeRepo) Append(_ context.Context, rec *domainaudit.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.appended = append(f.appended, rec)
	return nil
}

func (f *fakeRepo) AppendBatch(context.Context, []*domainaudit.Record) error { return nil }

func (f *fakeRepo) List(context.Context, domainaudit.Filter, crud.ListParams) ([]*domainaudit.Record, int, error) {
	return f.snapshot(), 1, nil
}

func (f *fakeRepo) snapshot() []*domainaudit.Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*domainaudit.Record, len(f.appended))
	copy(out, f.appended)
	return out
}

// fakeTx 直通执行（无事务语义）。
type fakeTx struct{}

func (fakeTx) Run(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// fakePublisher 记录事件并校验信封（与 BusPublisher 同语义，见坑清单）。
type fakePublisher struct {
	mu   sync.Mutex
	pubs []events.Envelope
	fail bool
}

func (f *fakePublisher) Publish(_ context.Context, ev events.Envelope) error {
	if err := ev.Validate(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return errors.New("publish failed")
	}
	f.pubs = append(f.pubs, ev)
	return nil
}

func newUC(repo domainaudit.Repository, pub events.Publisher) *UseCase {
	return New(repo, fakeTx{}, pub, clock.NewFake(time.Unix(1700000000, 0)))
}

func TestRecordPersistsAndPublishes(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{}
	uc := newUC(repo, pub)
	rec, err := uc.Record(context.Background(), "u1", "u1@example.com", "order.created", "order", "o-9", json.RawMessage(`{"amount":100}`))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.Source != domainaudit.SourceAPI || rec.OccurredAt.Unix() != 1700000000 {
		t.Fatalf("rec = %+v", rec)
	}
	if len(repo.appended) != 1 {
		t.Fatalf("appended %d records", len(repo.appended))
	}
	if len(pub.pubs) != 1 || pub.pubs[0].Name != domainaudit.EventLogRecorded {
		t.Fatalf("published %+v", pub.pubs)
	}
	// 载荷是记录 JSON 形态（消费方按目录文档解码）。
	var back domainaudit.Record
	if err := json.Unmarshal(pub.pubs[0].Payload, &back); err != nil || back.Action != "order.created" || back.ID != rec.ID {
		t.Fatalf("envelope payload = %s (err %v)", pub.pubs[0].Payload, err)
	}
}

func TestRecordTxRollbackOnPublishFailure(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePublisher{fail: true}
	uc := newUC(repo, pub)
	if _, err := uc.Record(context.Background(), "u1", "", "a.b", "", "", nil); err == nil {
		t.Fatal("want publish error to fail Record")
	}
	// fakeTx 无回滚语义；真实 tx.Run 下 Append 会被回滚——这里只断言
	// 错误向上传播（事务性由 DB 门控 repo 测试覆盖）。
}

func TestRecordValidationPropagates(t *testing.T) {
	uc := newUC(&fakeRepo{}, &fakePublisher{})
	if _, err := uc.Record(context.Background(), "u1", "", "", "", "", nil); !errors.Is(err, domainaudit.ErrEmptyAction) {
		t.Fatalf("want ErrEmptyAction, got %v", err)
	}
}
