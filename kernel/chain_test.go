package kernel_test

import (
	"context"
	"log/slog"
	"testing"

	grpcapiauthn "github.com/lynx-go/grpcapi/authn"
	grpcapiinterceptor "github.com/lynx-go/grpcapi/interceptor"
	grpcapipolicy "github.com/lynx-go/grpcapi/policy"
	"github.com/lynx-go/lynx-start/kernel"
	"google.golang.org/grpc"
)

func dummyStage(order int) grpcapiinterceptor.Stage {
	return grpcapiinterceptor.Stage{
		Name:  "test",
		Order: order,
		Unary: func(_ context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			return h(context.Background(), req)
		},
	}
}

func TestDefaultChainNoStages(t *testing.T) {
	set, err := grpcapipolicy.NewSet()
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	chain, err := kernel.DefaultChain(slog.Default(), set, &kernel.Aggregated{}, grpcapiauthn.Options{})
	if err != nil {
		t.Fatalf("DefaultChain: %v", err)
	}
	if chain != nil {
		t.Error("chain should be nil when no stages are contributed (server.New builds its own default chain)")
	}
	chain, err = kernel.DefaultChain(slog.Default(), set, nil, grpcapiauthn.Options{})
	if err != nil || chain != nil {
		t.Errorf("nil aggregated should give (nil, nil), got (%v, %v)", chain, err)
	}
}

func TestDefaultChainWithStages(t *testing.T) {
	set, err := grpcapipolicy.NewSet()
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}
	agg := &kernel.Aggregated{Stages: []grpcapiinterceptor.Stage{dummyStage(50)}}
	chain, err := kernel.DefaultChain(slog.Default(), set, agg, grpcapiauthn.Options{})
	if err != nil {
		t.Fatalf("DefaultChain: %v", err)
	}
	if chain == nil {
		t.Fatal("chain should be built when stages are contributed")
	}
}
