package server

import (
	"context"
	"fmt"
	"net"

	"github.com/romanpitatelev/vk-subpub/pkg/subpub"
	subscription_service "github.com/romanpitatelev/vk-subpub/pkg/subscription-service/gen/go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Config struct {
	Port          string
	MessageBuffer int
}

type Server struct {
	grpcServer *grpc.Server
	cfg        Config
	subPub     subpub.SubPub
	subscription_service.UnimplementedPubSubServer
}

func New(cfg Config) *Server {
	subPub := subpub.NewSubPub(cfg.MessageBuffer)

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
		s.grpcServer.GracefulStop()
	}()

	if err := s.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve in s.grpcServer(listener): %w", err)
	}

	return nil
}

func (s *Server) Subscribe(req *subscription_service.SubscribeRequest, stream grpc.ServerStreamingServer[subscription_service.Event]) error {
	if err := validateRequest(req); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to validate subscribe request: %v", err)
	}

	sub, err := s.subPub.Subscribe(req.Key, func(msg interface{}) {
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
	return nil
}

func (s *Server) Publish(ctx context.Context, req *subscription_service.PublishRequest) (*emptypb.Empty, error) {
	if err := validateRequest(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to validate publish request: %v", err)
	}

	event := &subscription_service.Event{
		Data: req.Data,
	}

	if err := s.subPub.Publish(req.Key, event); err != nil {
		switch err {
		case subpub.ErrSubPubClosed:
			return nil, status.Error(codes.Unavailable, "service is unavailable")
		default:
			return nil, status.Errorf(codes.Internal, "failed to publish: %v", err)
		}
	}

	return &emptypb.Empty{}, nil
}

type ValidateRequest interface {
	*subscription_service.PublishRequest | *subscription_service.SubscribeRequest
	GetKey() string
}

func validateRequest[T ValidateRequest](req T) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	if req.GetKey() == "" {
		return status.Error(codes.InvalidArgument, "key cannot be empty")
	}

	switch r := any(req).(type) {
	case *subscription_service.PublishRequest:
		if r.Data == "" {
			return status.Error(codes.InvalidArgument, "data cannot be empty")
		}
	}

	return nil
}
