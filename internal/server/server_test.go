package server_test

import (
	"context"
	"fmt"
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
			s.Require().NoError(err)
		}
	}()

	s.Require().Eventually(func() bool {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost%s", testPort), 100*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 5*time.Second, 100*time.Millisecond)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

func (s *IntegrationTestSuite) newClient() (*client.Client, func()) {
	conn, err := grpc.NewClient(
		fmt.Sprintf("localhost%s", testPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	s.Require().NoError(err)

	client, err := client.New(context.Background(), fmt.Sprintf("localhost%s", testPort))
	s.Require().NoError(err)

	return client, func() {
		client.Close()
		conn.Close()
	}
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) TestBasicOperations() {
	subClient, cleanupSub := s.newClient()
	defer cleanupSub()

	pubClient, cleanupPub := s.newClient()
	defer cleanupPub()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	eventChan, err := subClient.Subscribe(ctx, "topic-one")
	s.Require().NoError(err)

	err = pubClient.Publish(ctx, "topic-one", "message-one")
	s.Require().NoError(err)

	select {
	case event := <-eventChan:
		s.Equal("message-one", event.Data)
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
				return err
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
			client, cleanup := s.newClient()
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
