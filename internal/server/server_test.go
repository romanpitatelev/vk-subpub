package server_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/romanpitatelev/vk-subpub/internal/configs"
	"github.com/romanpitatelev/vk-subpub/internal/server"
	"github.com/romanpitatelev/vk-subpub/internal/subpub"
	"github.com/romanpitatelev/vk-subpub/pkg/client"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	testPort = ":9001"
	timeout  = 5 * time.Second
)

type IntegrationTestSuite struct {
	suite.Suite
	cancelFunc context.CancelFunc
	subPub     subpub.SubPub
	server     *server.Server
}

func (s *IntegrationTestSuite) SetupSuite() {
	cfg := configs.Config{
		BindAddress:   testPort,
		Timeout:       timeout,
		MessageBuffer: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	s.subPub = subpub.NewSubPub(cfg.MessageBuffer)

	s.server = server.New(server.Config{
		Port:          cfg.BindAddress,
		Timeout:       cfg.Timeout,
		MessageBuffer: cfg.MessageBuffer,
	}, s.subPub)

	go func() {
		if err := s.server.Run(ctx); err != nil {
			s.Require().NoError(err) //nolint:testifylint
		}
	}()

	s.Require().Eventually(func() bool {
		conn, err := net.DialTimeout("tcp", "localhost"+testPort, 100*time.Millisecond)
		if err != nil {
			return false
		}

		if err := conn.Close(); err != nil {
			s.T().Logf("failed to close test connection: %v", err)
		}

		return true
	}, 5*time.Second, 100*time.Millisecond)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

func (s *IntegrationTestSuite) NewClient() (*client.Client, func()) {
	conn, err := grpc.NewClient(
		"localhost"+testPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	s.Require().NoError(err)

	client, err := client.New(context.Background(), "localhost"+testPort)
	s.Require().NoError(err)

	return client, func() {
		if err := client.Close(); err != nil {
			s.T().Logf("failed to close client: %v", err)
		}

		if err := conn.Close(); err != nil {
			s.T().Logf("failed to close connection: %v", err)
		}
	}
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) TestBasicOperations() {
	subClient, cleanupSub := s.NewClient()
	defer cleanupSub()

	pubClient, cleanupPub := s.NewClient()
	defer cleanupPub()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	eventChan, err := subClient.Subscribe(ctx, "topic-one")
	s.Require().NoError(err)

	err = pubClient.Publish(ctx, "topic-one", "message-one")
	s.Require().NoError(err)

	select {
	case event := <-eventChan:
		s.Equal("message-one", event.GetData())
	case <-time.After(5 * time.Second):
		s.Fail("timeout waiting for message")
	}
}

func (s *IntegrationTestSuite) TestInvalidRequests() {
	tests := []struct {
		name        string
		action      func(*client.Client) error
		expectedErr codes.Code
	}{
		{
			name: "empty subscribe key",
			action: func(c *client.Client) error {
				_, err := c.Subscribe(context.Background(), "")

				return err //nolint:wrapcheck
			},
			expectedErr: codes.InvalidArgument,
		},
		{
			name: "empty publish key",
			action: func(c *client.Client) error {
				return c.Publish(context.Background(), "", "some-data")
			},
			expectedErr: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			client, cleanup := s.NewClient()
			defer cleanup()

			err := tt.action(client)
			s.Require().Error(err, "Test case: %s", tt.name)

			st, ok := status.FromError(err)
			s.Require().True(ok, "Expected gRPC status error")
			s.Equal(tt.expectedErr, st.Code(),
				"Expected status code %v, got %v for case: %s",
				tt.expectedErr, st.Code(), tt.name)
		})
	}
}
