package cmd

import (
	"net"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/config"
)

func TestBaiRequests_Mounted(t *testing.T) {
	for _, path := range [][]string{
		{"requests", "login"}, {"requests", "logout"}, {"requests", "cr", "list"},
		{"requests", "a4h", "status"}, {"requests", "user", "info"}, {"breq", "cr", "get"},
	} {
		if c, _, err := rootCmd.Find(path); err != nil || c == rootCmd {
			t.Errorf("bai %v not found", path)
		}
	}
}

func TestRequestsTarget(t *testing.T) {
	cfg := &config.Config{BFFURL: "cli.bluefunda.com:443"}
	if got := requestsTarget(cfg); got != Breq.BFFURL {
		t.Errorf("bai requests target = %q, want the BlueRequests BFF %q, not bai's", got, Breq.BFFURL)
	}
	t.Setenv("BAI_REQUESTS_BFF", "localhost:9999")
	if got := requestsTarget(cfg); got != "localhost:9999" {
		t.Errorf("BAI_REQUESTS_BFF override = %q", got)
	}
	app.Set(Breq)
	t.Cleanup(func() { app.Set(app.Bai) })
	if got := requestsTarget(&config.Config{BFFURL: "trm.example.com:443"}); got != "trm.example.com:443" {
		t.Errorf("breq target = %q, want its own configured endpoint", got)
	}
}

// TestBaiRequests_UsesBaiSignIn runs `bai requests user info` against a fake
// BlueRequests BFF: the call must carry the bai token, with no separate login.
func TestBaiRequests_UsesBaiSignIn(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu     sync.Mutex
		method string
		authz  []string
	)
	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		mu.Lock()
		defer mu.Unlock()
		method, _ = grpc.MethodFromServerStream(stream)
		md, _ := metadata.FromIncomingContext(stream.Context())
		authz = md.Get("authorization")
		return status.Error(codes.Unavailable, "fake")
	}))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("BAI_ACCESS_TOKEN", streamTestJWT())
	t.Setenv("BAI_REQUESTS_BFF", lis.Addr().String())
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	rootCmd.SetArgs([]string{"requests", "user", "info"})
	_ = Execute()

	mu.Lock()
	defer mu.Unlock()
	if method != "/bff.v1.BFFService/GetUserInfo" {
		t.Fatalf("BlueRequests BFF got method %q, want GetUserInfo", method)
	}
	if len(authz) != 1 || authz[0] != "Bearer "+streamTestJWT() {
		t.Errorf("authorization = %v, want the bai session token", authz)
	}
	if app.Current().Name != "bai" {
		t.Errorf("identity = %q, want bai", app.Current().Name)
	}
}

func TestBaiRequests_NotSignedInPointsAtBaiLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BAI_NO_KEYCHAIN", "1")
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	rootCmd.SetArgs([]string{"requests", "cr", "list"})
	err := Execute()
	if err == nil || !strings.Contains(err.Error(), "run `bai login`") {
		t.Errorf("err = %v, want a hint to run `bai login`", err)
	}
}
