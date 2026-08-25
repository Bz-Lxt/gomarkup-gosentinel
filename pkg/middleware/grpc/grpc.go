package grpcsentinel

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gosentinel/pkg/sentinel"
)

func UnaryInterceptor(g *sentinel.Guard) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		tok := g.Entry(info.FullMethod, "UNARY")
		if tok.Blocked {
			tok.Exit(sentinel.ResultFallback)
			code := codes.ResourceExhausted
			if tok.Reason == sentinel.ReasonCircuitOpen {
				code = codes.Unavailable
			}
			return nil, status.Error(code, string(tok.Reason))
		}
		resp, err := handler(ctx, req)
		if err != nil {
			if st, ok := status.FromError(err); ok {
				switch st.Code() {
				case codes.Internal, codes.Unavailable, codes.DeadlineExceeded:
					tok.Exit(sentinel.ResultError)
					return resp, err
				}
			}
			tok.Exit(sentinel.ResultBusiness)
			return resp, err
		}
		tok.Exit(sentinel.ResultOK)
		return resp, nil
	}
}
