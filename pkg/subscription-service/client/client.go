package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	subscription_service "github.com/romanpitatelev/vk-subpub/pkg/subscription-service/gen/go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ctxTimeout = 5 * time.Second
)

type ClientInterface interface {
	Subscribe(ctx context.Context, key string) (<-chan *subscription_service.Event, error)
	Publish(ctx context.Context, key, data string) error
}

type Client struct {
	conn   *grpc.ClientConn
	client subscription_service.PubSubClient
}

func New(ctx context.Context, addr string, opts ...grpc.DialOption) (*Client, error) {
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return &Client{
		conn:   conn,
		client: subscription_service.NewPubSubClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("failed to close client connection: %w", err)
	}

	return nil
}

func (c *Client) Subscribe(ctx context.Context, key string) (<-chan *subscription_service.Event, error) {
	if key == "" {
		return nil, ErrKeyEmpty
	}

	subCtx, cancel := context.WithCancel(ctx)

	stream, err := c.client.Subscribe(subCtx, &subscription_service.SubscribeRequest{
		Key: key,
	})
	if err != nil {
		cancel()

		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	eventChan := make(chan *subscription_service.Event)

	go func() {
		defer close(eventChan)
		defer cancel()

		for {
			event, err := stream.Recv()
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.Debug().Str("ket", key).Msg("client has closed the subscription")

					return
				}

				log.Error().Err(err).Str("key", key).Msg("subscription error")

				return
			}

			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

func (c *Client) Publish(ctx context.Context, key, data string) error {
	if key == "" {
		return ErrKeyEmpty
	}

	if data == "" {
		return ErrDataEmpty
	}

	ctx, cancel := context.WithTimeout(ctx, ctxTimeout)
	defer cancel()

	_, err := c.client.Publish(ctx, &subscription_service.PublishRequest{
		Key:  key,
		Data: data,
	})
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}
