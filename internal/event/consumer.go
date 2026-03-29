package event

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
	"unigroup-test-task/internal"
	"unigroup-test-task/internal/config"
)

type Consumer struct {
	logger internal.Logger
	client *redis.Client

	streamCfg  *config.StreamConfig
	backoffCfg *config.BackoffConfig
}

type ConsumerResult struct {
	MessageID  string
	EntityType string
	Entity     string
}

func NewConsumer(l internal.Logger, client *redis.Client, streamCfg *config.StreamConfig, backoffCfg *config.BackoffConfig) (*Consumer, error) {
	if l == nil {
		return nil, internal.ErrNilLogger
	}
	if client == nil {
		return nil, errors.New("redis client is nil")
	}
	if streamCfg == nil {
		return nil, errors.New("streamCfg is nil")
	}

	return &Consumer{
		logger:     l,
		client:     client,
		streamCfg:  streamCfg,
		backoffCfg: backoffCfg,
	}, nil
}

func (c *Consumer) Consume(ctx context.Context) error {
	c.logger.Info("Consumer started")

	err := c.createGroup(ctx, c.streamCfg.ID, c.streamCfg.Group.ID)
	if err != nil {
		c.logger.Error("Failed to create group", "err", err)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping consumer")
			return ctx.Err()
		default:
			res, err := c.consume(ctx)

			if err != nil {
				return err
			}

			if res.Entity == "" {
				continue
			}

			ctx = internal.WithMessageID(ctx, res.MessageID)

			c.logger.InfoContext(ctx, "Received message from stream")

			switch res.EntityType {
			case "OnProductCreated":
				createdProduct := internal.OnProductCreated{}
				err := json.Unmarshal([]byte(res.Entity), &createdProduct)
				if err != nil {
					c.logger.ErrorContext(ctx, "Failed to unmarshal product created event", "error", err)
				} else {
					c.logger.InfoContext(ctx, "Received product created event", "created_product", createdProduct)
				}
			case "OnProductDeleted":
				deletedProduct := internal.OnProductDeleted{}
				err := json.Unmarshal([]byte(res.Entity), &deletedProduct)
				if err != nil {
					c.logger.ErrorContext(ctx, "Failed to unmarshal product deleted event", "error", err)
				} else {
					c.logger.InfoContext(ctx, "Received product deleted event", "deleted_product_id", deletedProduct.ID)
				}
			default:
				c.logger.InfoContext(ctx, "Received unknown event type.", "entity_type", res.EntityType)
			}

			ackCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			ackErr := c.ack(ackCtx, c.streamCfg.ID, c.streamCfg.Group.ID, res.MessageID)
			cancel()

			if ackErr != nil {
				c.logger.Error("Failed to ack message", "error", ackErr)
			}
		}
	}
}

func (c *Consumer) createGroup(ctx context.Context, stream, group string) error {

	const EarliestMessage = "0" // Redis specific alias: start from the beginning of the stream

	_, err := c.client.XGroupCreateMkStream(ctx, stream, group, EarliestMessage).Result()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		c.logger.Warn("Failed to create consumer group.", "error", err, "stream", stream, "group", group)
		return err
	}
	c.logger.Info("Created consumer group", "stream", stream, "group", group)

	return nil
}

// consume Consumes a message from the specified stream. Returns Headers, MessageID, Data, Error
func (c *Consumer) consume(ctx context.Context) (ConsumerResult, error) {

	const undeliveredMessages = ">" // Redis specific alias: starts from the unconsumed message

	res, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Streams:  []string{c.streamCfg.ID, undeliveredMessages},
		Group:    c.streamCfg.Group.ID,
		Consumer: "0",
		Count:    c.streamCfg.ReadCount,
		Block:    c.streamCfg.BlockTime,
	}).Result()

	var result ConsumerResult
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return result, nil
		}
		if errors.Is(err, context.Canceled) {
			return result, nil
		}

		c.logger.Error("Failed to consume message.", "error", err, "stream", c.streamCfg.ID, "group", c.streamCfg.Group.ID)
		return result, err
	}

	if len(res) == 0 || len(res[0].Messages) == 0 {
		return result, nil
	}

	message := res[0].Messages[0]

	var dataType string
	var data string

	for k, v := range message.Values {
		if strVal, ok := v.(string); ok {
			if k == "data" {
				data = strVal
			} else if k == "dataType" {
				dataType = strVal
			}
		}
	}

	c.logger.Debug("Received message", "stream", c.streamCfg.ID, "group", c.streamCfg.Group.ID, "data", data)
	result.MessageID = message.ID
	result.EntityType = dataType
	result.Entity = data

	return result, nil
}

func (c *Consumer) ack(ctx context.Context, stream, group, messageId string) error {
	_, err := c.client.XAck(ctx, stream, group, messageId).Result()
	if err != nil {
		c.logger.Error("Acknowledgment failed", "stream", stream, "group", group, "messageId", messageId, "error", err)
		return err
	}
	return nil
}
