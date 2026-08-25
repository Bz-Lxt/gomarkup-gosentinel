package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"gosentinel/internal/config"
	"gosentinel/internal/log"
	"gosentinel/internal/node"
	grpcsentinel "gosentinel/pkg/middleware/grpc"
	"gosentinel/pkg/sentinel"
)

type demoServer struct{}

func (demoServer) Work(ctx context.Context, in *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
	arg := ""
	if in != nil {
		arg = in.GetValue()
	}
	if arg == "fail" {
		return nil, status.Error(codes.Internal, "forced")
	}
	if n, err := strconv.Atoi(arg); err == nil && n > 0 {
		time.Sleep(time.Duration(n) * time.Millisecond)
	}
	return wrapperspb.String("ok"), nil
}

func main() {
	cfg := config.Load()
	log.Init(cfg.LogLevel, os.Stdout)
	service := env("GOSENTINEL_SERVICE", "demo-grpc")
	instance := env("HOSTNAME", "grpc-1")
	guard := sentinel.New(sentinel.Options{Service: service})

	client := &node.Client{
		URL:      env("GOSENTINEL_CONTROL", "ws://127.0.0.1:31482/ws/nodes"),
		NodeID:   service + "-" + instance,
		Service:  service,
		Instance: instance,
		Guard:    guard,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go client.Run(ctx)

	lis, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		log.Logger().Error("listen", "err", err)
		os.Exit(1)
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(grpcsentinel.UnaryInterceptor(guard)))
	hs := health.NewServer()
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, hs)
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "demo.Demo",
		HandlerType: (*workServer)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Work",
			Handler:    workHandler,
		}},
	}, demoServer{})

	go func() {
		log.Logger().Info("example-grpc listening", "addr", cfg.Listen)
		if err := s.Serve(lis); err != nil {
			log.Logger().Error("grpc serve", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		s.Stop()
	}
}

type workServer interface {
	Work(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error)
}

func workHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(wrapperspb.StringValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/demo.Demo/Work"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(workServer).Work(ctx, req.(*wrapperspb.StringValue))
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	return interceptor(ctx, in, info, handler)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
