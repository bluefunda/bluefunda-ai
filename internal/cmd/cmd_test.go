package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// testBFF is a fake BFF server for integration tests.
type testBFF struct {
	pb.UnimplementedBFFServiceServer
}

func (t *testBFF) GetUserInfo(_ context.Context, _ *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
	return &pb.GetUserInfoResponse{
		Sub:               "user-123",
		Name:              "Test User",
		Email:             "test@example.com",
		PreferredUsername: "testuser",
		EmailVerified:     true,
		GivenName:         "Test",
		FamilyName:        "User",
	}, nil
}

func (t *testBFF) GetLLMModels(_ context.Context, _ *pb.GetLLMModelsRequest) (*pb.GetLLMModelsResponse, error) {
	return &pb.GetLLMModelsResponse{
		Models: []*pb.LLMModel{
			{Name: "gpt-4", ModelId: 1, OwnedBy: "openai"},
			{Name: "claude-3", ModelId: 2, OwnedBy: "anthropic"},
		},
	}, nil
}

func (t *testBFF) GetChatIds(_ context.Context, _ *pb.GetChatIdsRequest) (*pb.GetChatIdsResponse, error) {
	return &pb.GetChatIdsResponse{
		Chats: []*pb.ChatMetadata{
			{ChatId: "chat-1", ChatTitle: "First Chat", Model: "gpt-4", CreatedAt: "2025-01-01"},
			{ChatId: "chat-2", ChatTitle: "Second Chat", Model: "claude-3", CreatedAt: "2025-01-02"},
		},
	}, nil
}

func (t *testBFF) GetChatHistory(_ context.Context, req *pb.GetChatHistoryRequest) (*pb.GetChatHistoryResponse, error) {
	return &pb.GetChatHistoryResponse{
		Messages: []*pb.ChatMessage{
			{Role: "user", Content: "Hello", CreatedAt: "2025-01-01T00:00:00Z"},
			{Role: "assistant", Content: "Hi there!", CreatedAt: "2025-01-01T00:00:01Z"},
		},
	}, nil
}

func (t *testBFF) StopChat(_ context.Context, req *pb.StopChatRequest) (*pb.StopChatResponse, error) {
	return &pb.StopChatResponse{Success: true}, nil
}

func (t *testBFF) GenerateTitle(_ context.Context, req *pb.GenerateTitleRequest) (*pb.GenerateTitleResponse, error) {
	return &pb.GenerateTitleResponse{GeneratedTitle: "Generated Title"}, nil
}

func (t *testBFF) Chat(_ *pb.ChatRequest, stream grpc.ServerStreamingServer[pb.ChatEvent]) error {
	_ = stream.Send(&pb.ChatEvent{Type: "content", Content: "Summary of the conversation so far."})
	_ = stream.Send(&pb.ChatEvent{Type: "done"})
	return nil
}

func (t *testBFF) QueryRateLimit(_ context.Context, _ *pb.QueryRateLimitRequest) (*pb.QueryRateLimitResponse, error) {
	return &pb.QueryRateLimitResponse{
		Allowed:   true,
		Remaining: 42,
		UserStats: &pb.UserLimitStats{
			PlanType:          "pro",
			HourlyPercentage:  10.5,
			DailyPercentage:   25.0,
			MonthlyPercentage: 5.2,
		},
	}, nil
}

func (t *testBFF) GetMcpInfo(_ context.Context, _ *pb.GetMcpInfoRequest) (*pb.GetMcpInfoResponse, error) {
	return &pb.GetMcpInfoResponse{
		McpServers: []*pb.MCPInfo{
			{ServerId: 1, Name: "test-mcp", Type: "sse", IsAvailable: true, ShortDescription: "A test MCP"},
		},
	}, nil
}

func (t *testBFF) GetStripeSubscription(_ context.Context, _ *pb.GetStripeSubscriptionRequest) (*pb.GetStripeSubscriptionResponse, error) {
	return &pb.GetStripeSubscriptionResponse{
		HasSubscription:    true,
		PlanName:           "Pro",
		SubscriptionStatus: "active",
		ExpirationDate:     "2026-01-01",
		DailyTokenLimit:    100000,
		MonthlyTokenLimit:  3000000,
	}, nil
}

func (t *testBFF) GetStripePlans(_ context.Context, _ *pb.GetStripePlansRequest) (*pb.GetStripePlansResponse, error) {
	return &pb.GetStripePlansResponse{
		Plans: []*pb.StripePlan{
			{PlanId: "free", Name: "Free", PriceCents: 0, BillingPeriod: "month", Features: []string{"Basic access"}},
			{PlanId: "pro", Name: "Pro", PriceCents: 2000, BillingPeriod: "month", Features: []string{"Unlimited", "Priority"}},
		},
	}, nil
}

func (t *testBFF) GetUserSettings(_ context.Context, _ *pb.GetUserSettingsRequest) (*pb.GetUserSettingsResponse, error) {
	return &pb.GetUserSettingsResponse{
		LlmModels: []*pb.LLMModel{{Name: "gpt-4", ModelId: 1}},
	}, nil
}

// startTestServer creates an in-process gRPC server and returns a client connection.
func startTestServer(t *testing.T) pb.BFFServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterBFFServiceServer(srv, &testBFF{})

	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(func() { srv.Stop() })

	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}
	cc, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })

	return pb.NewBFFServiceClient(cc)
}

// --- Model List Tests ---

func TestModelList_Table(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetLLMModels(ctx, &pb.GetLLMModelsRequest{})
	if err != nil {
		t.Fatalf("GetLLMModels: %v", err)
	}

	if len(resp.GetModels()) != 2 {
		t.Fatalf("expected 2 models, got %d", len(resp.GetModels()))
	}
	if resp.GetModels()[0].GetName() != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", resp.GetModels()[0].GetName())
	}
	if resp.GetModels()[1].GetName() != "claude-3" {
		t.Errorf("expected claude-3, got %s", resp.GetModels()[1].GetName())
	}
}

func TestModelList_JSON(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetLLMModels(ctx, &pb.GetLLMModelsRequest{})
	if err != nil {
		t.Fatalf("GetLLMModels: %v", err)
	}

	// Verify proto can be serialized to JSON.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")

	type modelJSON struct {
		Name    string `json:"name"`
		OwnedBy string `json:"owned_by"`
	}
	models := make([]modelJSON, 0, len(resp.GetModels()))
	for _, m := range resp.GetModels() {
		models = append(models, modelJSON{Name: m.GetName(), OwnedBy: m.GetOwnedBy()})
	}
	if err := enc.Encode(models); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if !strings.Contains(buf.String(), "gpt-4") {
		t.Errorf("expected gpt-4 in JSON output")
	}
}

// --- User Info Tests ---

func TestUserInfo(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetUserInfo(ctx, &pb.GetUserInfoRequest{})
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}

	if resp.GetName() != "Test User" {
		t.Errorf("expected 'Test User', got %q", resp.GetName())
	}
	if resp.GetEmail() != "test@example.com" {
		t.Errorf("expected 'test@example.com', got %q", resp.GetEmail())
	}
	if !resp.GetEmailVerified() {
		t.Error("expected email_verified to be true")
	}
}

// --- Chat List Tests ---

func TestChatList(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetChatIds(ctx, &pb.GetChatIdsRequest{})
	if err != nil {
		t.Fatalf("GetChatIds: %v", err)
	}

	if len(resp.GetChats()) != 2 {
		t.Fatalf("expected 2 chats, got %d", len(resp.GetChats()))
	}
	if resp.GetChats()[0].GetChatId() != "chat-1" {
		t.Errorf("expected chat-1, got %s", resp.GetChats()[0].GetChatId())
	}
}

// --- Chat History Tests ---

func TestChatHistory(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetChatHistory(ctx, &pb.GetChatHistoryRequest{ChatId: "chat-1"})
	if err != nil {
		t.Fatalf("GetChatHistory: %v", err)
	}

	if len(resp.GetMessages()) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.GetMessages()))
	}
	if resp.GetMessages()[0].GetRole() != "user" {
		t.Errorf("expected role 'user', got %q", resp.GetMessages()[0].GetRole())
	}
}

// --- Chat Stop Tests ---

func TestChatStop(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.StopChat(ctx, &pb.StopChatRequest{ChatId: "chat-1"})
	if err != nil {
		t.Fatalf("StopChat: %v", err)
	}

	if !resp.GetSuccess() {
		t.Error("expected success=true")
	}
}

// --- Generate Title Tests ---

func TestGenerateTitle(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GenerateTitle(ctx, &pb.GenerateTitleRequest{ChatId: "chat-1"})
	if err != nil {
		t.Fatalf("GenerateTitle: %v", err)
	}

	if resp.GetGeneratedTitle() != "Generated Title" {
		t.Errorf("expected 'Generated Title', got %q", resp.GetGeneratedTitle())
	}
}

// --- Rate Limit Tests ---

func TestRateLimit(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.QueryRateLimit(ctx, &pb.QueryRateLimitRequest{})
	if err != nil {
		t.Fatalf("QueryRateLimit: %v", err)
	}

	if !resp.GetAllowed() {
		t.Error("expected allowed=true")
	}
	if resp.GetRemaining() != 42 {
		t.Errorf("expected remaining=42, got %d", resp.GetRemaining())
	}
	if resp.GetUserStats().GetPlanType() != "pro" {
		t.Errorf("expected plan 'pro', got %q", resp.GetUserStats().GetPlanType())
	}
}

// --- MCP List Tests ---

func TestMCPList(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetMcpInfo(ctx, &pb.GetMcpInfoRequest{})
	if err != nil {
		t.Fatalf("GetMcpInfo: %v", err)
	}

	if len(resp.GetMcpServers()) != 1 {
		t.Fatalf("expected 1 MCP server, got %d", len(resp.GetMcpServers()))
	}
	if resp.GetMcpServers()[0].GetName() != "test-mcp" {
		t.Errorf("expected 'test-mcp', got %q", resp.GetMcpServers()[0].GetName())
	}
}

// --- Billing Tests ---

func TestBillingSubscription(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetStripeSubscription(ctx, &pb.GetStripeSubscriptionRequest{})
	if err != nil {
		t.Fatalf("GetStripeSubscription: %v", err)
	}

	if !resp.GetHasSubscription() {
		t.Error("expected has_subscription=true")
	}
	if resp.GetPlanName() != "Pro" {
		t.Errorf("expected plan 'Pro', got %q", resp.GetPlanName())
	}
}

func TestBillingPlans(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetStripePlans(ctx, &pb.GetStripePlansRequest{})
	if err != nil {
		t.Fatalf("GetStripePlans: %v", err)
	}

	if len(resp.GetPlans()) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(resp.GetPlans()))
	}
	if resp.GetPlans()[1].GetPriceCents() != 2000 {
		t.Errorf("expected 2000 cents, got %d", resp.GetPlans()[1].GetPriceCents())
	}
}

// --- Compact History Tests ---

// TestCompactHistory_SummaryIsSystemRole verifies that the summary injected by
// compactHistory uses role "system", not "assistant". An assistant turn with no
// preceding user turn violates the alternating-turn structure that most LLM
// providers enforce, causing "LLM error for a prompt" (#191).
func TestCompactHistory_SummaryIsSystemRole(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}

	history := []codeMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
		{Role: "user", Content: "Do something"},
		{Role: "assistant", Content: "Done"},
		{Role: "user", Content: "Latest prompt"},
	}

	result, err := compactHistory(conn, "test-chat", "", history)
	if err != nil {
		t.Fatalf("compactHistory: %v", err)
	}

	var summaryRole string
	for _, m := range result {
		if strings.Contains(m.Content, "[Conversation summary]") {
			summaryRole = m.Role
			break
		}
	}
	if summaryRole == "" {
		t.Fatal("no summary message found in compacted history")
	}
	if summaryRole != "system" {
		t.Errorf("summary message role = %q, want \"system\" (orphaned assistant turn triggers LLM error)", summaryRole)
	}

	// No assistant turn should appear before the first user turn.
	seenUser := false
	for _, m := range result {
		if m.Role == "user" {
			seenUser = true
		}
		if !seenUser && m.Role == "assistant" {
			t.Errorf("orphaned assistant message before first user turn: content=%q", truncate(m.Content, 60))
		}
	}
}

// --- isContextWindowError Tests ---

func TestIsContextWindowError(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"413 request too large", true},
		{"context length exceeded", true},
		{"context window exceeded", true},
		{"token limit exceeded", true},
		{"token length exceeded", true},
		{"max tokens exceeded", true},
		{"LLM error for a prompt", false},
		{"rate limit exceeded", false},
		{"internal server error", false},
		{"", false},
	}
	for _, tc := range cases {
		var err error
		if tc.msg != "" {
			err = fmt.Errorf("%s", tc.msg)
		}
		got := isContextWindowError(err)
		if got != tc.want {
			t.Errorf("isContextWindowError(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// --- Truncate Tests ---

func TestTruncate(t *testing.T) {
	cases := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a long string that should be truncated", 20, "this is a long st..."},
		{"has\nnewlines\nin it", 20, "has newlines in it"},
	}
	for _, tc := range cases {
		got := truncate(tc.input, tc.max)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
		}
	}
}
