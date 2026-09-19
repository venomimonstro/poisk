package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/venomimonstro/poisk/internal/indexer/manticore"
	"github.com/venomimonstro/poisk/internal/indexer/outbox"
	"github.com/venomimonstro/poisk/internal/indexer/source"
)

const WebDocumentEntity = "WEB_DOCUMENT"

type Source interface {
	LoadVersion(ctx context.Context, urlID, version int64) (source.Document, error)
}

type Index interface {
	Apply(ctx context.Context, doc manticore.Document) (bool, error)
	Delete(ctx context.Context, id int64) error
}

type Ack interface {
	MarkProcessed(ctx context.Context, eventID int64, workerID string) error
	Retry(ctx context.Context, eventID int64, workerID, lastError string, retryAfter time.Duration) error
}

type Processor struct {
	Source    Source
	Index     Index
	Ack       Ack
	RetryBase time.Duration
	RetryMax  time.Duration
}

func (p Processor) Process(ctx context.Context, event outbox.Event) error {
	if event.EntityType != WebDocumentEntity {
		return p.fail(ctx, event, fmt.Errorf("unsupported entity type %q", event.EntityType))
	}
	if event.WorkerID == "" { return errors.New("leased event has empty worker id") }

	var err error
	switch event.Operation {
	case "DELETE":
		err = p.Index.Delete(ctx, event.EntityID)
	case "UPSERT":
		err = p.applyUpsert(ctx, event)
	default:
		err = fmt.Errorf("unsupported index operation %q", event.Operation)
	}
	if err != nil { return p.fail(ctx, event, err) }
	if err := p.Ack.MarkProcessed(ctx, event.ID, event.WorkerID); err != nil {
		return fmt.Errorf("mark processed after index success: %w", err)
	}
	return nil
}

func (p Processor) applyUpsert(ctx context.Context, event outbox.Event) error {
	doc, err := p.Source.LoadVersion(ctx, event.EntityID, event.EntityVersion)
	if errors.Is(err, source.ErrNotIndexable) {
		// A newer extraction can turn an already indexed page into noindex/thin.
		// DELETE is idempotent and prevents serving stale content.
		return p.Index.Delete(ctx, event.EntityID)
	}
	if err != nil { return err }
	_, err = p.Index.Apply(ctx, manticore.Document{
		ID:            doc.ID,
		EntityVersion: doc.Version,
		Title:         doc.Title,
		Description:   doc.Description,
		Body:          doc.Body,
		URL:           doc.URL,
		Host:          doc.Host,
		Lang:          doc.Lang,
		ContentHash:   doc.ContentHash,
		QualityScore:  doc.QualityScore,
		SpamScore:     doc.SpamScore,
		FetchedAtUnix: doc.FetchedAt.Unix(),
	})
	return err
}

func (p Processor) fail(ctx context.Context, event outbox.Event, cause error) error {
	if p.Ack == nil { return cause }
	delay := retryDelay(p.RetryBase, p.RetryMax, event.Attempts)
	if err := p.Ack.Retry(ctx, event.ID, event.WorkerID, truncateError(cause, 2048), delay); err != nil {
		return errors.Join(cause, fmt.Errorf("schedule index retry: %w", err))
	}
	return cause
}

func retryDelay(base, max time.Duration, attempts int32) time.Duration {
	if base <= 0 { base = time.Second }
	if max <= 0 { max = time.Minute }
	if base > max { base = max }
	d := base
	for i := int32(1); i < attempts; i++ {
		if d >= max/2 { return max }
		d *= 2
	}
	if d > max { return max }
	return d
}

func truncateError(err error, max int) string {
	if err == nil { return "" }
	s := err.Error()
	if len(s) <= max { return s }
	return s[:max]
}
