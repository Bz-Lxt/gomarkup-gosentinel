package grpcsentinel_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	grpcsentinel "gosentinel/pkg/middleware/grpc"
	"gosentinel/pkg/sentinel"
)

type waitServiceServer interface {
	Wait(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
}

type waitService struct {
	entered chan struct{}
	exited  chan struct{}
	release chan struct{}
}

func (s *waitService) Wait(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	close(s.entered)
	defer close(s.exited)
	select {
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	case <-s.release:
		return &emptypb.Empty{}, nil
	}
}

func waitHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(emptypb.Empty)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(waitServiceServer).Wait(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/test.WaitService/Wait"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(waitServiceServer).Wait(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, req, info, handler)
}

func TestUnaryInterceptorPropagatesClientCancellation(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	guard := sentinel.New(sentinel.Options{Service: "context-test"})
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcsentinel.UnaryInterceptor(guard)))
	svc := &waitService{
		entered: make(chan struct{}),
		exited:  make(chan struct{}),
		release: make(chan struct{}),
	}
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "test.WaitService",
		HandlerType: (*waitServiceServer)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Wait",
			Handler:    waitHandler,
		}},
	}, svc)
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	defer func() {
		server.Stop()
		_ = listener.Close()
		<-serveDone
	}()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelDial()
	conn, err := grpc.DialContext(
		dialCtx,
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rpcCtx, cancelRPC := context.WithCancel(context.Background())
	callDone := make(chan error, 1)
	go func() {
		callDone <- conn.Invoke(rpcCtx, "/test.WaitService/Wait", &emptypb.Empty{}, &emptypb.Empty{})
	}()

	select {
	case <-svc.entered:
	case <-time.After(time.Second):
		close(svc.release)
		t.Fatal("handler did not start")
	}
	cancelRPC()

	select {
	case err := <-callDone:
		if status.Code(err) != codes.Canceled {
			close(svc.release)
			t.Fatalf("Invoke() error code = %v, want %v", status.Code(err), codes.Canceled)
		}
	case <-time.After(time.Second):
		close(svc.release)
		t.Fatal("client call did not return after cancellation")
	}

	select {
	case <-svc.exited:
		close(svc.release)
	case <-time.After(250 * time.Millisecond):
		close(svc.release)
		<-svc.exited
		t.Error("server handler remained active after the RPC context was canceled")
	}
}
