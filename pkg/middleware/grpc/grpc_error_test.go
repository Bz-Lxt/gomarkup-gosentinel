package grpcsentinel_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gosentinel/internal/rule"
	grpcsentinel "gosentinel/pkg/middleware/grpc"
	"gosentinel/pkg/sentinel"
)

func TestUnaryInterceptorWrappedStatusTripsCircuit(t *testing.T) {
	r := rule.Default()
	r.Service = "checkout"
	r.Resource = "/checkout.Order/Place"
	r.Method = "UNARY"
	r.MinRequests = 1
	r.ErrorRate = 0.01

	guard := sentinel.New(sentinel.Options{
		Service: "checkout",
		Rules:   []rule.Snapshot{r},
	})
	interceptor := grpcsentinel.UnaryInterceptor(guard)
	info := &grpc.UnaryServerInfo{FullMethod: "/checkout.Order/Place"}

	var calls atomic.Int32
	handler := func(context.Context, any) (any, error) {
		if calls.Add(1) == 1 {
			return nil, fmt.Errorf("inventory lookup: %w", status.Error(codes.Internal, "backend unavailable"))
		}
		return "unexpected success", nil
	}

	if _, err := interceptor(context.Background(), nil, info, handler); status.Code(err) != codes.Internal {
		t.Fatalf("first call code = %s, want %s", status.Code(err), codes.Internal)
	}
	if _, err := interceptor(context.Background(), nil, info, handler); status.Code(err) != codes.Unavailable {
		t.Fatalf("second call code = %s, want %s", status.Code(err), codes.Unavailable)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("handler calls = %d, want 1", got)
	}
}
