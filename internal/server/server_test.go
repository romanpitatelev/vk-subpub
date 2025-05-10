package server_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/romanpitatelev/vk-subpub/internal/configs"
	"github.com/romanpitatelev/vk-subpub/internal/server"
	"github.com/romanpitatelev/vk-subpub/pkg/subscription-service/client"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	testPort = 9001
	timeout  = 5 * time.Second
)

type IntegrationTestSuite struct {
	suite.Suite
	cancelFunc context.CancelFunc
	server     *server.Server
	client     *client.Client
	grpcConn   *grpc.ClientConn
}

func (s *IntegrationTestSuite) SetupSuite() {
	cfg := configs.Config{
		AppPort:       testPort,
		Timeout:       timeout,
		MessageBuffer: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel

	s.server = server.New(server.Config{
		Port:          ":" + strconv.Itoa(testPort),
		Timeout:       timeout,
		MessageBuffer: cfg.MessageBuffer,
	})

	go func() {
		if err := s.server.Run(ctx); err != nil {
			s.Require().NoError(err)
		}
	}()

	s.Require().Eventually(func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", strconv.Itoa(testPort)), 100*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 5*time.Second, 100*time.Millisecond)

	conn, err := grpc.NewClient(
		net.JoinHostPort("localhost", strconv.Itoa(testPort)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	s.Require().NoError(err)
	s.grpcConn = conn

	s.client, err = client.New(context.Background(), net.JoinHostPort("localhost", strconv.Itoa(testPort)))
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}

	if s.grpcConn != nil {
		s.grpcConn.Close()
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) TestBasicOperations() {

}
