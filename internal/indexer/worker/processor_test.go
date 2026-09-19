package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
	"github.com/venomimonstro/poisk/internal/indexer/source"
)

type fakeSource struct {
	current bool
	doc     source.Document
	err     error
}
func (f fakeSource) IsCurrent(context.Context, int64, int64) (bool, error) { return f.current, nil }
func (f fakeSource) LoadVersion(context.Context, int64, int64) (source.Document, error) { return f.doc, f.err }

type fakeIndex struct {
	applied int
	deleted int
	err     error
}
func (f *fakeIndex) Apply(context.Context, manticore.Document) (bool, error) { f.applied++; return f.err == nil, f.err }
func (f *fakeIndex) Delete(context.Context, int64) error { f.deleted++; return f.err }

type fakeAck struct {
	processed int
	retried   int
	delay     time.Duration
}
func (f *fakeAck) MarkProcessed(context.Context, int64, string) error { f.processed++; return nil }
func (f *fakeAck) Retry(_ context.Context, _ int64, _ string, _ string, d time.Duration) error { f.retried++; f.delay = d; return nil }

func event() outbox.Event {
	return outbox.Event{ID: 1, EntityType: WebDocumentEntity, EntityID: 10, EntityVersion: 3, Operation: "UPSERT", Attempts: 1, WorkerID: "indexer-1"}
}

func TestProcessMarksOnlyAfterSuccessfulApply(t *testing.T) {
	idx := &fakeIndex{}
	ack := &fakeAck{}
	p := Processor{
		Source: fakeSource{current: true, doc: source.Document{ID: 10, Version: 3, Body: "text", FetchedAt: time.Unix(100, 0)}},
		Index: idx,
		Ack:   ack,
	}
	if err := p.Process(context.Background(), event()); err != nil { t.Fatal(err) }
	if idx.applied != 1 || ack.processed != 1 || ack.retried != 0 { t.Fatalf("index=%+v ack=%+v", idx, ack) }
}

func TestStaleEventIsAcknowledgedWithoutIndexMutation(t *testing.T) {
	idx := &fakeIndex{}
	ack := &fakeAck{}
	p := Processor{Source: fakeSource{current: false}, Index: idx, Ack: ack}
	if err := p.Process(context.Background(), event()); err != nil { t.Fatal(err) }
	if idx.applied != 0 || idx.deleted != 0 || ack.processed != 1 || ack.retried != 0 {
		t.Fatalf("index=%+v ack=%+v", idx, ack)
	}
}

func TestNoIndexDeletesExistingDocument(t *testing.T) {
	idx := &fakeIndex{}
	ack := &fakeAck{}
	p := Processor{Source: fakeSource{current: true, err: source.ErrNotIndexable}, Index: idx, Ack: ack}
	if err := p.Process(context.Background(), event()); err != nil { t.Fatal(err) }
	if idx.deleted != 1 || idx.applied != 0 || ack.processed != 1 { t.Fatalf("index=%+v ack=%+v", idx, ack) }
}

func TestIndexFailureSchedulesRetryWithoutAck(t *testing.T) {
	idx := &fakeIndex{err: errors.New("manticore unavailable")}
	ack := &fakeAck{}
	e := event(); e.Attempts = 3
	p := Processor{
		Source: fakeSource{current: true, doc: source.Document{ID: 10, Version: 3, Body: "text", FetchedAt: time.Now()}},
		Index: idx, Ack: ack, RetryBase: time.Second, RetryMax: 10*time.Second,
	}
	if err := p.Process(context.Background(), e); err == nil { t.Fatal("expected error") }
	if ack.processed != 0 || ack.retried != 1 || ack.delay != 4*time.Second { t.Fatalf("ack=%+v", ack) }
}

func TestDeleteEventIsIdempotentPath(t *testing.T) {
	idx := &fakeIndex{}
	ack := &fakeAck{}
	e := event(); e.Operation = "DELETE"
	p := Processor{Source: fakeSource{current: true}, Index: idx, Ack: ack}
	if err := p.Process(context.Background(), e); err != nil { t.Fatal(err) }
	if idx.deleted != 1 || ack.processed != 1 { t.Fatalf("index=%+v ack=%+v", idx, ack) }
}

func TestRetryDelayCaps(t *testing.T) {
	if got := retryDelay(time.Second, 5*time.Second, 10); got != 5*time.Second { t.Fatalf("delay=%s", got) }
}
