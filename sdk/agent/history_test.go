package agent

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
)

// answerBFF answers every Chat call with one line of text and records the requests.
type answerBFF struct {
	pb.UnimplementedBFFServiceServer
	mu   sync.Mutex
	reqs []*pb.ChatRequest
}

func (f *answerBFF) Chat(req *pb.ChatRequest, stream grpc.ServerStreamingServer[pb.ChatEvent]) error {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	n := len(f.reqs)
	f.mu.Unlock()
	answer := map[int]string{1: "The answer is 42.", 2: "Sure."}[n]
	if err := stream.Send(&pb.ChatEvent{Type: "content", Content: answer}); err != nil {
		return err
	}
	return stream.Send(&pb.ChatEvent{Type: "done"})
}

// TestRun_FollowUpCarriesEarlierAnswer is the #390 regression for the SDK
// loop: the second Run must send the first answer, and only the first
// request of the chat is flagged as a new chat.
func TestRun_FollowUpCarriesEarlierAnswer(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	bff := &answerBFF{}
	pb.RegisterBFFServiceServer(srv, bff)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	token := fakeJWT()
	r := New(Options{
		MaxTurns:        3,
		DisableBuiltins: true,
		Tools:           []Tool{echoTool("adt_echo")},
		TokenFunc:       func() (string, error) { return token, nil },
		BFFURL:          lis.Addr().String(),
	})
	defer func() { _ = r.Close() }()

	for _, prompt := range []string{"what is the answer?", "say sure"} {
		if err := r.Run(context.Background(), prompt); err != nil {
			t.Fatalf("Run(%q): %v", prompt, err)
		}
	}

	if len(bff.reqs) != 2 {
		t.Fatalf("BFF calls = %d, want 2", len(bff.reqs))
	}
	var payload struct {
		History []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"history"`
	}
	if err := json.Unmarshal([]byte(bff.reqs[1].GetPrompt()), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	h := payload.History
	if len(h) != 3 || h[0].Role != "user" || h[1].Role != "assistant" || h[1].Content != "The answer is 42." || h[2].Role != "user" {
		t.Errorf("second request history = %+v, want user, assistant(\"The answer is 42.\"), user", h)
	}
	if !bff.reqs[0].GetIsNewChat() || bff.reqs[1].GetIsNewChat() {
		t.Errorf("IsNewChat = %v/%v, want true then false", bff.reqs[0].GetIsNewChat(), bff.reqs[1].GetIsNewChat())
	}
}
