package requests

import (
	"context"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	baipb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	trmpb "github.com/bluefunda/bluefunda-ai/api/proto/trm"
)

// The TRM server serves bff.v1.BFFService. Our copy of the proto uses its own
// package so it can share a binary with bai's BFF, and generate-proto.sh
// rewrites the client's method paths back. Every RPC must still go out as
// /bff.v1.BFFService/<Method>, or requests.bluefunda.com rejects it.
func TestClientUsesServerWirePaths(t *testing.T) {
	var (
		mu   sync.Mutex
		seen = map[string]bool{}
	)
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		m, _ := grpc.MethodFromServerStream(stream)
		mu.Lock()
		seen[m] = true
		mu.Unlock()
		return nil
	}))
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	cc, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cc.Close() }()

	client := reflect.ValueOf(trmpb.NewBFFServiceClient(cc))
	ctx := reflect.ValueOf(context.Background())
	want := map[string]bool{}
	for i := 0; i < client.NumMethod(); i++ {
		name := client.Type().Method(i).Name
		want["/bff.v1.BFFService/"+name] = true
		m := client.Method(i)
		req := reflect.New(m.Type().In(1).Elem())
		out := m.Call([]reflect.Value{ctx, req})
		// Server-streaming calls only reach the server once the stream is read.
		if s, ok := out[0].Interface().(interface{ RecvMsg(any) error }); ok && out[1].IsNil() {
			_ = s.RecvMsg(new(trmpb.Event))
		}
	}
	if len(want) != 18 {
		t.Errorf("TRM client has %d RPCs, want 18 — update this test if the proto changed", len(want))
	}
	for m := range want {
		if !seen[m] {
			t.Errorf("server never saw %s", m)
		}
	}
	for m := range seen {
		if !strings.HasPrefix(m, "/bff.v1.BFFService/") {
			t.Errorf("RPC sent as %s, want /bff.v1.BFFService/*", m)
		}
	}
}

// bai's BFF proto and the TRM proto are linked into the same binaries
// (bai requests, breq). Referencing both here makes this test binary
// register both; a descriptor conflict would panic at init.
func TestBothBFFProtosCoexist(t *testing.T) {
	bai := (&baipb.ChatRequest{}).ProtoReflect().Descriptor().FullName()
	trm := (&trmpb.ListChangeRequestsRequest{}).ProtoReflect().Descriptor().FullName()
	if bai.Parent() == trm.Parent() {
		t.Errorf("both protos use package %s", bai.Parent())
	}
}
