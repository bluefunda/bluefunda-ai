package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/audit"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/hooks"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
	"github.com/bluefunda/bluefunda-ai/internal/plugins"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// scriptedBFF answers each Chat call with the next scripted event list and
// records every request.
type scriptedBFF struct {
	pb.UnimplementedBFFServiceServer
	mu      sync.Mutex
	replies [][]*pb.ChatEvent
	reqs    []*pb.ChatRequest
}

func (s *scriptedBFF) Chat(req *pb.ChatRequest, stream grpc.ServerStreamingServer[pb.ChatEvent]) error {
	s.mu.Lock()
	s.reqs = append(s.reqs, req)
	reply := s.replies[len(s.reqs)-1]
	s.mu.Unlock()
	for _, ev := range reply {
		if err := stream.Send(ev); err != nil {
			return err
		}
	}
	return nil
}

// sentHistory decodes the history bai sent in request i.
func (s *scriptedBFF) sentHistory(t *testing.T, i int) []codeMessage {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	var payload struct {
		History []codeMessage `json:"history"`
	}
	if err := json.Unmarshal([]byte(s.reqs[i].GetPrompt()), &payload); err != nil {
		t.Fatalf("decode request %d payload: %v", i, err)
	}
	return payload.History
}

func startScriptedBFF(t *testing.T, bff *scriptedBFF) *caigrpc.Conn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterBFFServiceServer(srv, bff)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	cc, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return &caigrpc.Conn{Client: pb.NewBFFServiceClient(cc)}
}

func textReply(s string) []*pb.ChatEvent {
	return []*pb.ChatEvent{{Type: "content", Content: s}, {Type: "done"}}
}

// TestAgenticLoop_KeepsAssistantTextInHistory is the #390 regression: the
// assistant's text never reached history, so a follow-up prompt showed the
// earlier question as unanswered and the model went back to it.
func TestAgenticLoop_KeepsAssistantTextInHistory(t *testing.T) {
	toolCall, _ := json.Marshal(map[string]string{
		"tool_call_id": "c1", "tool_name": "bash", "arguments": `{"command":"echo hi"}`,
	})
	bff := &scriptedBFF{replies: [][]*pb.ChatEvent{
		{{Type: "content", Content: "Checking."}, {Type: "tool_call", Data: string(toolCall)}}, // prompt 1: text, then a tool
		textReply("It is 42."), // prompt 1: final answer
		textReply("Sure."),     // prompt 2
	}}
	conn := startScriptedBFF(t, bff)

	run := func(history []codeMessage, first bool) []codeMessage {
		ch := make(chan tui.StreamEvent, 256)
		go func() {
			for range ch { //nolint:revive // drain
			}
		}()
		defer close(ch)
		out, err := agenticLoopTUI(conn, nil, "chat-1", "", "[]", history, first,
			nil, nil, true, false, nil, 10, 0, 0, "", &audit.Logger{},
			hooks.New("", "test-session", "."), mcp.NewManager(context.Background(), nil),
			plugins.NewManager(t.TempDir()), &ui.Printer{Out: io.Discard, Err: io.Discard}, ch)
		if err != nil {
			t.Fatalf("agenticLoopTUI: %v", err)
		}
		return out
	}

	history := run([]codeMessage{{Role: "user", Content: "what is the answer?"}}, true)
	history = run(append(history, codeMessage{Role: "user", Content: "thanks, now say sure"}), false)

	// Request 2 (after the tool ran) must carry the text that came before the tool call.
	sent := bff.sentHistory(t, 1)
	if len(sent) != 3 || sent[1].Role != "assistant" || sent[1].Content != "Checking." || len(sent[1].ToolCalls) != 1 || sent[2].Role != "tool" {
		t.Errorf("request 2 history = %+v, want user, assistant(\"Checking.\" + tool call), tool", sent)
	}

	// Request 3 (the follow-up prompt) must show the first prompt as answered.
	sent = bff.sentHistory(t, 2)
	roles := make([]string, len(sent))
	for i, m := range sent {
		roles[i] = m.Role
	}
	want := []string{"user", "assistant", "tool", "assistant", "user"}
	if len(roles) != len(want) || sent[3].Content != "It is 42." {
		t.Fatalf("request 3 history roles = %v (answer %q), want %v with the first answer", roles, contentAt(sent, 3), want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Errorf("request 3 history roles = %v, want %v", roles, want)
			break
		}
	}

	// The returned history ends with the latest answer, so the session file and /retry see it too.
	if last := history[len(history)-1]; last.Role != "assistant" || last.Content != "Sure." {
		t.Errorf("last history message = %+v, want the assistant's \"Sure.\"", last)
	}

	if bff.reqs[0].GetIsNewChat() != true || bff.reqs[1].GetIsNewChat() || bff.reqs[2].GetIsNewChat() {
		t.Errorf("IsNewChat = %v/%v/%v, want true only on the first request",
			bff.reqs[0].GetIsNewChat(), bff.reqs[1].GetIsNewChat(), bff.reqs[2].GetIsNewChat())
	}
}

func contentAt(h []codeMessage, i int) string {
	if i < len(h) {
		return h[i].Content
	}
	return ""
}
