package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/romanpitatelev/vk-subpub/internal/subpub"
	subscription_service "github.com/romanpitatelev/vk-subpub/pkg/subscription-service/gen/go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	timeout = 5 * time.Second
)

type Config struct {
	Port          string
	Timeout       time.Duration
	MessageBuffer int
}

type SubPub interface {
	Subscribe(key string, callback func(msg interface{})) (subpub.Subscription, error)
	Publish(subject string, msg interface{}) error
	Close(ctx context.Context) error
}

type Server struct {
	grpcServer *grpc.Server
	cfg        Config
	subPub     subpub.SubPub
	subscription_service.UnimplementedPubSubServer
}

func New(cfg Config, subPub subpub.SubPub) *Server {
	s := &Server{
		grpcServer: grpc.NewServer(),
		cfg:        cfg,
		subPub:     subPub,
	}

	return s
}

func (s *Server) Run(ctx context.Context) error {
	log.Info().Msgf("Starting grpc server on %s", s.cfg.Port)

	subscription_service.RegisterPubSubServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	listener, err := net.Listen("tcp", s.cfg.Port)
	if err != nil {
		return fmt.Errorf("net.Listen(tcp, s.cfg.Port): %w", err)
	}

	go func() {
		<-ctx.Done()
		log.Info().Msg("shutting down server ...")

		closeCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		timer := time.AfterFunc(timeout, func() {
			log.Info().Msg("server could not stop gracefully in time. Doing force stop.")
			s.grpcServer.Stop()
		})
		defer timer.Stop()

		//nolint:contextcheck
		if err := s.subPub.Close(closeCtx); err != nil {
			log.Error().Err(err).Msg("failed to close subpub")
		}

		s.grpcServer.GracefulStop()
		log.Info().Msg("server stopped gracefully")
	}()

	log.Info().Msg("server is running")

	if err := s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve in s.grpcServer(listener): %w", err)
	}

	return nil
}

func (s *Server) Subscribe(req *subscription_service.SubscribeRequest, stream grpc.ServerStreamingServer[subscription_service.Event]) error {
	if err := validateRequest(req); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to validate subscribe request: %v", err)
	}

	sub, err := s.subPub.Subscribe(req.GetKey(), func(msg interface{}) {
		event, ok := msg.(*subscription_service.Event)
		if !ok {
			log.Error().Msg("invalid message type in callback function")

			return
		}

		if err := stream.Send(event); err != nil {
			log.Error().Err(err).Msg("failed to send event to stream")
		}
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to subscribe: %v", err)
	}

	<-stream.Context().Done()
	sub.Unsubscribe()
	log.Info().Str("key", req.GetKey()).Msg("subscription ended")

	return nil
}

func (s *Server) Publish(_ context.Context, req *subscription_service.PublishRequest) (*emptypb.Empty, error) {
	if err := validateRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to validate publish request: %v", err)
	}

	event := &subscription_service.Event{
		Data: req.GetData(),
	}

	if err := s.subPub.Publish(req.GetKey(), event); err != nil {
		if errors.Is(err, subpub.ErrSubPubClosed) {
			return nil, status.Errorf(codes.Unavailable, "service is unavailable: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
	}

	return &emptypb.Empty{}, nil
}

type ValidateRequest interface {
	GetKey() string
}

func validateRequest(req ValidateRequest) error {
	if req == nil {
		err := status.Error(codes.InvalidArgument, "request cannot be nil")

		return fmt.Errorf("validation failed: %w", err)
	}

	if req.GetKey() == "" {
		err := status.Error(codes.InvalidArgument, "key cannot be empty")

		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}
