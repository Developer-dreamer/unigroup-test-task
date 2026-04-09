package event

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"math/rand"
	"time"
	"unigroup-test-task/internal"
	"unigroup-test-task/internal/config"
)

var ErrNilOutbox = errors.New("outbox is nil")

type Outbox interface {
	GetAllPendingEvents(ctx context.Context, count int) ([]Event, error)
	ChangeEventStatus(ctx context.Context, eventID uuid.UUID, eventStatus Status) error
	IncrementRetryCount(ctx context.Context, eventID uuid.UUID, errorMessage string) error
}

type TransactionManager interface {
	WithinTransaction(ctx context.Context, tFunc func(ctx context.Context) error) error
}

type Producer struct {
	logger internal.Logger
	client *redis.Client
	config *config.StreamConfig
}

func NewProducer(l internal.Logger, client *redis.Client, streamCfg *config.StreamConfig) (*Producer, error) {
	if l == nil {
		return nil, internal.ErrNilLogger
	}
	if client == nil {
		return nil, errors.New("redis client is nil")
	}
	if streamCfg == nil {
		return nil, errors.New("redis stream config is nil")
	}

	return &Producer{
		logger: l,
		client: client,
		config: streamCfg,
	}, nil
}

func (p *Producer) Publish(ctx context.Context, dataType string, data json.RawMessage) error {
	values := map[string]interface{}{
		"data_type": dataType,
		"data":      string(data),
	}

	_, err := p.client.XAdd(ctx, &redis.XAddArgs{
		MaxLen: p.config.MaxBacklog,
		Approx: p.config.UseDelApprox,
		Stream: p.config.ID,
		Values: values,
	}).Result()

	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to publish message.", "error", err, "stream", p.config.ID, "data", data)
		return err
	}

	p.logger.DebugContext(ctx, "Published message", "stream", p.config.ID, "data", data)
	return nil
}

type Relay struct {
	logger     internal.Logger
	tx         TransactionManager
	repo       Outbox
	producer   *Producer
	backoffCfg *config.BackoffConfig
}

func NewRelayService(l internal.Logger, tx TransactionManager, repo Outbox, producer *Producer, backoffCfg *config.BackoffConfig) (*Relay, error) {
	if l == nil {
		return nil, internal.ErrNilLogger
	}
	if tx == nil {
		return nil, errors.New("nil transaction")
	}
	if repo == nil {
		return nil, ErrNilOutbox
	}
	if producer == nil {
		return nil, errors.New("nil producer")
	}
	if backoffCfg == nil {
		return nil, errors.New("nil config")
	}

	return &Relay{
		logger:     l,
		tx:         tx,
		repo:       repo,
		producer:   producer,
		backoffCfg: backoffCfg,
	}, nil
}

func (r *Relay) Start(ctx context.Context) error {
	r.logger.Info("Publisher started")

	currentBackoff := r.backoffCfg.Min
	maxEvents := 10

	ticker := time.NewTicker(r.backoffCfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping producer")
			return ctx.Err()
		case <-ticker.C:
			events, err := r.repo.GetAllPendingEvents(ctx, maxEvents)
			if err != nil {
				newBackOff, backOffErr := r.backOff(ctx, currentBackoff)
				currentBackoff = newBackOff

				if backOffErr != nil {
					return errors.Join(err, backOffErr)
				}

				continue
			}

			currentBackoff = r.backoffCfg.Min
			if len(events) == 0 {
				continue
			}

			for _, event := range events {
				r.processSingleEvent(ctx, event)
			}
		}
	}
}

func (r *Relay) processSingleEvent(ctx context.Context, event Event) {
	r.logger.InfoContext(ctx, "Sending message to stream", "message_id", event.ID)

	err := r.producer.Publish(ctx, event.AggregateType, event.Payload)
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to publish message", "message_id", event.ID, "error", err)

		_ = r.saveProcessingError(ctx, event, err)
		return
	}

	if err := r.repo.ChangeEventStatus(ctx, event.ID, Processed); err != nil {
		r.logger.ErrorContext(ctx, "Failed to mark event as processed", "error", err)
	}
}

func (r *Relay) saveProcessingError(ctx context.Context, event Event, err error) error {
	return r.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if event.RetryCount > 5 {
			if dbErr := r.repo.ChangeEventStatus(ctx, event.ID, Failed); dbErr != nil {
				r.logger.ErrorContext(ctx, "Failed to mark event as processed", "error", err)

				return dbErr
			}
		} else {
			if dbErr := r.repo.IncrementRetryCount(ctx, event.ID, err.Error()); dbErr != nil {
				r.logger.ErrorContext(ctx, "Failed to increment retry count", "error", err)

				return dbErr
			}

		}
		return nil
	})
}

func (r *Relay) backOff(ctx context.Context, currentBackoff time.Duration) (time.Duration, error) {
	jitter := time.Duration(rand.Int63n(int64(currentBackoff) / 5))
	sleepTime := currentBackoff + jitter

	r.logger.Info("Backoff active", "sleep_time", sleepTime.String())

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(sleepTime):
		// Time's up - continuing work
	}

	currentBackoff *= time.Duration(r.backoffCfg.Factor)
	if currentBackoff > r.backoffCfg.Max {
		currentBackoff = r.backoffCfg.Max
	}

	return currentBackoff, nil
}
