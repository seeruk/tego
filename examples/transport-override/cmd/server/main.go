package main

import (
	"context"
	"log"
	"net"

	"github.com/seeruk/tego/examples/transport-override/override"
	"github.com/seeruk/tego/examples/transport-override/overridepbv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type profileService struct {
	override.UnimplementedProfileService
}

func (profileService) GetProfile(ctx context.Context, id string) (override.Profile, error) {
	return override.Profile{ID: id, DisplayName: "Ada"}, nil
}

type profileServer struct {
	overridepbv1.UnimplementedProfileServiceServer
	*override.ProfileServiceGRPCAdapter
}

func newProfileServer(service override.ProfileService) overridepbv1.ProfileServiceServer {
	return &profileServer{ProfileServiceGRPCAdapter: override.NewProfileServiceGRPCAdapter(service)}
}

func (s *profileServer) GetProfile(ctx context.Context, request *overridepbv1.GetProfileRequest) (*overridepbv1.GetProfileResponse, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		log.Printf("request metadata: %v", md.Get("x-example-request"))
	}
	if err := grpc.SetHeader(ctx, metadata.Pairs("x-example-response", "transport-override")); err != nil {
		return nil, err
	}
	return s.AdaptGetProfile(ctx, request)
}

func main() {
	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	overridepbv1.RegisterProfileServiceServer(server, newProfileServer(profileService{}))

	log.Printf("serving transport override example on %s", listener.Addr())
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
