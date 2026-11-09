package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
)

func echoTool(name string) Tool {
	return Tool{
		Name:        name,
		Description: "echo the input",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
		Handler: func(_ context.Context, args string) (string, error) {
			return "echo:" + args, nil
		},
	}
}

func schemaNames(t *testing.T, schemas string) []string {
	t.Helper()
	var decoded []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal([]byte(schemas), &decoded); err != nil {
		t.Fatalf("decode schemas: %v", err)
	}
	names := make([]string, len(decoded))
	for i, d := range decoded {
		names[i] = d.Function.Name
	}
	return names
}

func TestToolSchemas_AppendsCustomTools(t *testing.T) {
	r := New(Options{Tools: []Tool{echoTool("adt_echo")}})
	schemas, err := r.toolSchemas()
	if err != nil {
		t.Fatalf("toolSchemas: %v", err)
	}
	names := schemaNames(t, schemas)
	if names[len(names)-1] != "adt_echo" {
		t.Errorf("custom tool not last: %v", names)
	}
	if !contains(schemas, `"bash"`) {
		t.Error("built-in tools missing")
	}
}

func TestToolSchemas_DisableBuiltins(t *testing.T) {
	r := New(Options{Tools: []Tool{echoTool("adt_echo"), {Name: "noargs", Handler: echoTool("x").Handler}}, DisableBuiltins: true})
	schemas, err := r.toolSchemas()
	if err != nil {
		t.Fatalf("toolSchemas: %v", err)
	}
	names := schemaNames(t, schemas)
	if len(names) != 2 || names[0] != "adt_echo" || names[1] != "noargs" {
		t.Errorf("names = %v, want [adt_echo noargs]", names)
	}
	if !contains(schemas, `"parameters":{"type":"object","properties":{}}`) {
		t.Errorf("nil Parameters should default to an empty object schema: %s", schemas)
	}
}

func TestToolSchemas_RejectsBadTools(t *testing.T) {
	cases := map[string][]Tool{
		"duplicate custom":  {echoTool("a"), echoTool("a")},
		"shadows built-in":  {echoTool("bash")},
		"missing handler":   {{Name: "x"}},
		"missing tool name": {{Handler: echoTool("x").Handler}},
	}
	for name, ts := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := New(Options{Tools: ts}).toolSchemas(); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestWrapToolFn_RoutesCustomTools(t *testing.T) {
	var intercepted []string
	r := New(Options{
		Tools: []Tool{echoTool("adt_echo")},
		OnToolCall: func(tc ToolCall) (ToolResult, error) {
			intercepted = append(intercepted, tc.Name)
			return ToolResult{Output: "intercepted"}, nil
		},
	})
	fn := r.wrapToolFn()

	out, err := fn(context.Background(), "adt_echo", `{"text":"hi"}`)
	if err != nil || out != `echo:{"text":"hi"}` {
		t.Errorf("custom tool = %q, %v", out, err)
	}
	if out, _ := fn(context.Background(), "bash", `{}`); out != "intercepted" {
		t.Errorf("built-in should go to OnToolCall, got %q", out)
	}
	if len(intercepted) != 1 || intercepted[0] != "bash" {
		t.Errorf("OnToolCall saw %v, want [bash]", intercepted)
	}
}

func TestWrapToolFn_DisableBuiltinsRejectsUnknown(t *testing.T) {
	fn := New(Options{Tools: []Tool{echoTool("adt_echo")}, DisableBuiltins: true}).wrapToolFn()
	if _, err := fn(context.Background(), "bash", `{"command":"rm -rf /"}`); err == nil {
		t.Error("expected unknown-tool error for a built-in when builtins are disabled")
	}
}

// ── End-to-end against an in-process BFF ─────────────────────────────────────

type fakeBFF struct {
	pb.UnimplementedBFFServiceServer
	mu      sync.Mutex
	calls   int
	auth    []string
	prompts []string
}

func (f *fakeBFF) Chat(req *pb.ChatRequest, stream grpc.ServerStreamingServer[pb.ChatEvent]) error {
	f.mu.Lock()
	f.calls++
	call := f.calls
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		f.auth = append(f.auth, md.Get("authorization")...)
	}
	f.prompts = append(f.prompts, req.GetPrompt())
	f.mu.Unlock()

	if call == 1 {
		data, _ := json.Marshal(map[string]string{
			"tool_call_id": "t1", "tool_name": "adt_echo", "arguments": `{"text":"hi"}`,
		})
		return stream.Send(&pb.ChatEvent{Type: "tool_call", Data: string(data)})
	}
	if err := stream.Send(&pb.ChatEvent{Type: "content", Content: "done"}); err != nil {
		return err
	}
	return stream.Send(&pb.ChatEvent{Type: "done"})
}

func fakeJWT() string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." +
		enc([]byte(`{"sub":"u1","iss":"https://auth.example.com/realms/individual"}`)) + ".sig"
}

func TestRun_CustomToolsAndTokenFunc(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	bff := &fakeBFF{}
	pb.RegisterBFFServiceServer(srv, bff)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	token := fakeJWT()
	var events []Event
	r := New(Options{
		MaxTurns:        3,
		Tools:           []Tool{echoTool("adt_echo")},
		DisableBuiltins: true,
		TokenFunc:       func() (string, error) { return token, nil },
		BFFURL:          lis.Addr().String(),
		OnEvent:         func(ev Event) { events = append(events, ev) },
	})
	defer func() { _ = r.Close() }()

	if err := r.Run(context.Background(), "go"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if bff.calls != 2 {
		t.Fatalf("BFF calls = %d, want 2 (tool call, then final answer)", bff.calls)
	}
	for _, a := range bff.auth {
		if a != "Bearer "+token {
			t.Errorf("authorization = %q, want the TokenFunc token", a)
		}
	}
	if !strings.Contains(bff.prompts[0], "adt_echo") || strings.Contains(bff.prompts[0], `\"bash\"`) {
		t.Errorf("first request should offer only adt_echo: %s", bff.prompts[0])
	}

	var toolOut string
	for _, ev := range events {
		if ev.Type == "tool_result" && ev.ToolName == "adt_echo" {
			toolOut = ev.ToolOutput
		}
	}
	if toolOut != `echo:{"text":"hi"}` {
		t.Errorf("tool_result = %q", toolOut)
	}
}

func TestRun_TokenFuncError(t *testing.T) {
	r := New(Options{
		TokenFunc: func() (string, error) { return "", errors.New("not signed in — run `abaper login`") },
		BFFURL:    "127.0.0.1:1",
		MaxTurns:  1,
	})
	defer func() { _ = r.Close() }()
	err := r.Run(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "abaper login") {
		t.Errorf("err = %v, want the TokenFunc error to surface", err)
	}
}
