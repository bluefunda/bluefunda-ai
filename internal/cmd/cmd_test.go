package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
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
			RpmUsed:           2,
			RpmLimit:          5,
			RpmPercentage:     40.0,
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

func (t *testBFF) GetChatContext(_ context.Context, req *pb.GetChatContextRequest) (*pb.GetChatContextResponse, error) {
	return &pb.GetChatContextResponse{
		Messages: []*pb.ChatMessage{
			{Role: "system", Content: "You are helpful.", CreatedAt: "2025-01-01T00:00:00Z"},
		},
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

func TestRateLimit_RPMFields(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.QueryRateLimit(ctx, &pb.QueryRateLimitRequest{})
	if err != nil {
		t.Fatalf("QueryRateLimit: %v", err)
	}

	stats := resp.GetUserStats()
	if stats == nil {
		t.Fatal("expected UserStats to be set")
	}
	if stats.GetRpmUsed() != 2 {
		t.Errorf("RpmUsed: want 2, got %d", stats.GetRpmUsed())
	}
	if stats.GetRpmLimit() != 5 {
		t.Errorf("RpmLimit: want 5, got %d", stats.GetRpmLimit())
	}
	if stats.GetRpmPercentage() != 40.0 {
		t.Errorf("RpmPercentage: want 40.0, got %f", stats.GetRpmPercentage())
	}
}

func TestRateLimit_NoHourlyPercentage(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.QueryRateLimit(ctx, &pb.QueryRateLimitRequest{})
	if err != nil {
		t.Fatalf("QueryRateLimit: %v", err)
	}

	// Field 2 (hourly_percentage) is reserved — GetHourlyPercentage should not exist.
	// This test verifies the testBFF stub compiles without it. The compile-time
	// absence of HourlyPercentage in UserLimitStats is the real assertion.
	stats := resp.GetUserStats()
	if stats == nil {
		t.Fatal("expected UserStats")
	}
	// daily and monthly should still be populated
	if stats.GetDailyPercentage() != 25.0 {
		t.Errorf("DailyPercentage: want 25.0, got %f", stats.GetDailyPercentage())
	}
	if stats.GetMonthlyPercentage() != 5.2 {
		t.Errorf("MonthlyPercentage: want 5.2, got %f", stats.GetMonthlyPercentage())
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
		// HTTP 413
		{"413 request too large", true},
		// Explicit context/token keywords (OpenAI, Anthropic, etc.)
		{"context length exceeded", true},
		{"context window exceeded", true},
		{"token limit exceeded", true},
		{"token length exceeded", true},
		{"max tokens exceeded", true},
		// Bare "LLM error" from Groq (context overflow wrapped in generic phrase)
		{"LLM error", true},
		{"llm error", true},
		// Routing failures must NOT be retried (wrong model name)
		{"LLM routing failed", false},
		// Other errors that should not trigger context retry
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

// --- ResolveModelAlias Tests ---

func TestResolveModelAlias(t *testing.T) {
	cases := []struct {
		alias string
		want  string
	}{
		{"auto", ""},
		{"", ""},
		{"fast", "groq"},
		{"think", ":think"},
		{"thinking", ":think"},
		{"openai", "openai"},
		{"anthropic", "anthropic"},
		{"groq:llama-3.3-70b", "groq:llama-3.3-70b"},
		{"claude-3-sonnet", "claude-3-sonnet"},
	}
	for _, tc := range cases {
		got := resolveModelAlias(tc.alias)
		if got != tc.want {
			t.Errorf("resolveModelAlias(%q) = %q, want %q", tc.alias, got, tc.want)
		}
	}
}

// --- FormatVersion Tests ---

func TestFormatVersion(t *testing.T) {
	cases := []struct {
		ver  string
		want string
	}{
		{"", ""},
		{"dev", "dev"},
		{"1.2.3", "v1.2.3"},
		{"1.35.1", "v1.35.1"},
	}
	for _, tc := range cases {
		got := formatVersion(tc.ver)
		if got != tc.want {
			t.Errorf("formatVersion(%q) = %q, want %q", tc.ver, got, tc.want)
		}
	}
}

// --- FindContextFile Tests ---

func TestFindContextFile_BaiContext(t *testing.T) {
	dir := t.TempDir()
	baiDir := filepath.Join(dir, ".bai")
	if err := os.MkdirAll(baiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baiDir, "context.md"), []byte("project context"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Simulate git root so walk stops here.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := findContextFile(dir)
	if got != "project context" {
		t.Errorf("expected 'project context', got %q", got)
	}
}

func TestFindContextFile_AgentsMd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := findContextFile(dir)
	if got != "agents content" {
		t.Errorf("expected 'agents content', got %q", got)
	}
}

func TestFindContextFile_WalksUp(t *testing.T) {
	root := t.TempDir()
	baiDir := filepath.Join(root, ".bai")
	if err := os.MkdirAll(baiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baiDir, "context.md"), []byte("root context"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got := findContextFile(sub)
	if got != "root context" {
		t.Errorf("expected 'root context', got %q", got)
	}
}

func TestFindContextFile_NoneFound(t *testing.T) {
	dir := t.TempDir()
	// No .bai/context.md, no AGENTS.md, but has .git so walk stops.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := findContextFile(dir)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- LoadContextFiles Tests ---

func TestLoadContextFiles_ProjectOnly(t *testing.T) {
	dir := t.TempDir()
	baiDir := filepath.Join(dir, ".bai")
	if err := os.MkdirAll(baiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baiDir, "context.md"), []byte("project ctx"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := loadContextFiles(dir)
	if !strings.Contains(got, "project ctx") {
		t.Errorf("expected 'project ctx' in output, got %q", got)
	}
}

func TestLoadContextFiles_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := loadContextFiles(dir)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- FindRecentSession Tests ---

func TestFindRecentSession_NoChats(t *testing.T) {
	// Use a testBFF that returns no chats.
	srv, conn := startTestServerRaw(t, &emptyBFF{})
	defer srv.Stop()

	cid, title := findRecentSession(conn)
	if cid != "" || title != "" {
		t.Errorf("expected empty result, got chatID=%q title=%q", cid, title)
	}
}

// emptyBFF returns no chats.
type emptyBFF struct{ pb.UnimplementedBFFServiceServer }

func (e *emptyBFF) GetChatIds(_ context.Context, _ *pb.GetChatIdsRequest) (*pb.GetChatIdsResponse, error) {
	return &pb.GetChatIdsResponse{}, nil
}

// startTestServerRaw starts a gRPC server with the given handler and returns a Conn.
func startTestServerRaw(t *testing.T, svc pb.BFFServiceServer) (*grpc.Server, *caigrpc.Conn) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterBFFServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })

	dialer := func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }
	cc, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return srv, &caigrpc.Conn{Client: pb.NewBFFServiceClient(cc)}
}

// testPrinter returns a Printer that writes to a buffer instead of stdout.
func testPrinter(format ui.OutputFormat) (*ui.Printer, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &ui.Printer{Out: buf, Err: buf, Format: format}, buf
}

// --- ChatListRPC Tests ---

func TestChatListRPC_Table(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatTable)

	if err := chatListRPC(conn, p); err != nil {
		t.Fatalf("chatListRPC: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "chat-1") {
		t.Errorf("expected chat-1 in table output, got: %s", out)
	}
	if !strings.Contains(out, "First Chat") {
		t.Errorf("expected 'First Chat' in table output, got: %s", out)
	}
	if !strings.Contains(out, "gpt-4") {
		t.Errorf("expected model 'gpt-4' in table output, got: %s", out)
	}
}

func TestChatListRPC_JSON(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatJSON)

	if err := chatListRPC(conn, p); err != nil {
		t.Fatalf("chatListRPC: %v", err)
	}

	if !strings.Contains(buf.String(), "chat-1") {
		t.Errorf("expected chat-1 in JSON output, got: %s", buf.String())
	}
}

// --- ChatHistoryRPC Tests ---

func TestChatHistoryRPC_Table(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatTable)

	if err := chatHistoryRPC(conn, "chat-1", p); err != nil {
		t.Fatalf("chatHistoryRPC: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "user") {
		t.Errorf("expected role 'user' in output, got: %s", out)
	}
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected 'Hello' in output, got: %s", out)
	}
}

func TestChatHistoryRPC_JSON(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatJSON)

	if err := chatHistoryRPC(conn, "chat-1", p); err != nil {
		t.Fatalf("chatHistoryRPC: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected 'Hello' in JSON output, got: %s", out)
	}
}

// --- ChatTitleRPC Tests ---

func TestChatTitleRPC_Table(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatTable)

	if err := chatTitleRPC(conn, "chat-1", "hint", p); err != nil {
		t.Fatalf("chatTitleRPC: %v", err)
	}

	if !strings.Contains(buf.String(), "Generated Title") {
		t.Errorf("expected 'Generated Title' in output, got: %s", buf.String())
	}
}

func TestChatTitleRPC_JSON(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatJSON)

	if err := chatTitleRPC(conn, "chat-1", "", p); err != nil {
		t.Fatalf("chatTitleRPC: %v", err)
	}

	if !strings.Contains(buf.String(), "Generated Title") {
		t.Errorf("expected 'Generated Title' in JSON output, got: %s", buf.String())
	}
}

// --- ChatStopRPC Tests ---

func TestChatStopRPC_Success(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatTable)

	if err := chatStopRPC(conn, "chat-1", p); err != nil {
		t.Fatalf("chatStopRPC: %v", err)
	}

	if !strings.Contains(buf.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %s", buf.String())
	}
}

func TestChatStopRPC_Failure(t *testing.T) {
	_, conn := startTestServerRaw(t, &failStopBFF{})
	p, buf := testPrinter(ui.FormatTable)

	if err := chatStopRPC(conn, "chat-1", p); err != nil {
		t.Fatalf("chatStopRPC: %v", err)
	}

	if !strings.Contains(buf.String(), "Failed") {
		t.Errorf("expected 'Failed' in output, got: %s", buf.String())
	}
}

type failStopBFF struct{ pb.UnimplementedBFFServiceServer }

func (f *failStopBFF) StopChat(_ context.Context, _ *pb.StopChatRequest) (*pb.StopChatResponse, error) {
	return &pb.StopChatResponse{Success: false}, nil
}

// --- ModelListRPC Tests ---

func TestModelListRPC_Table(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatTable)

	if err := modelListRPC(conn, p); err != nil {
		t.Fatalf("modelListRPC: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gpt-4") {
		t.Errorf("expected 'gpt-4' in output, got: %s", out)
	}
	if !strings.Contains(out, "openai") {
		t.Errorf("expected 'openai' in output, got: %s", out)
	}
	if !strings.Contains(out, "claude-3") {
		t.Errorf("expected 'claude-3' in output, got: %s", out)
	}
}

func TestModelListRPC_JSON(t *testing.T) {
	client := startTestServer(t)
	conn := &caigrpc.Conn{Client: client}
	p, buf := testPrinter(ui.FormatJSON)

	if err := modelListRPC(conn, p); err != nil {
		t.Fatalf("modelListRPC: %v", err)
	}

	var m map[string]interface{}
	// ProtoJSON wraps arrays; just check content is present.
	if !strings.Contains(buf.String(), "gpt-4") {
		t.Errorf("expected 'gpt-4' in JSON output, got: %s", buf.String())
	}
	_ = m
}

// Ensure json import is used (it's referenced in TestModelList_JSON above).
var _ = json.NewEncoder

// ============================================================
// --- drainPrintStream Tests (headless print mode, #77) ---
// ============================================================

// feedEvents sends events to a channel and closes it.
func feedEvents(events []tui.StreamEvent) <-chan tui.StreamEvent {
	ch := make(chan tui.StreamEvent, len(events)+1)
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch
}

func TestDrainPrintStream_TextMode_Chunk(t *testing.T) {
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "chunk", Chunk: "Hello, "},
		{Kind: "chunk", Chunk: "world!"},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	code := drainPrintStream(ch, "text", &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if out.String() != "Hello, world!" {
		t.Errorf("text mode output = %q, want %q", out.String(), "Hello, world!")
	}
}

func TestDrainPrintStream_TextMode_Error(t *testing.T) {
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "error", ErrMsg: "something failed"},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	code := drainPrintStream(ch, "text", &out, &errOut)
	if code != 1 {
		t.Errorf("expected exit code 1 on error, got %d", code)
	}
	if !strings.Contains(errOut.String(), "something failed") {
		t.Errorf("expected error in stderr, got: %s", errOut.String())
	}
}

func TestDrainPrintStream_JSONMode(t *testing.T) {
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "chunk", Chunk: "The answer is 42."},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	code := drainPrintStream(ch, "json", &out, &errOut)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	var result []map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("json mode output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result entry, got %d", len(result))
	}
	if result[0]["text"] != "The answer is 42." {
		t.Errorf("text = %q, want %q", result[0]["text"], "The answer is 42.")
	}
	if result[0]["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason = %q, want %q", result[0]["stop_reason"], "end_turn")
	}
}

func TestDrainPrintStream_JSONMode_AccumulatesChunks(t *testing.T) {
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "chunk", Chunk: "part1 "},
		{Kind: "chunk", Chunk: "part2 "},
		{Kind: "chunk", Chunk: "part3"},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	drainPrintStream(ch, "json", &out, &errOut)

	var result []map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("json output invalid: %v", err)
	}
	if result[0]["text"] != "part1 part2 part3" {
		t.Errorf("accumulated text = %q, want %q", result[0]["text"], "part1 part2 part3")
	}
}

func TestDrainPrintStream_StreamJSONMode_Chunk(t *testing.T) {
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "chunk", Chunk: "hello"},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	drainPrintStream(ch, "stream-json", &out, &errOut)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	// Expect chunk line + done line.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 NDJSON lines, got: %s", out.String())
	}
	var ev map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &ev); err != nil {
		t.Fatalf("line 0 not valid JSON: %v", err)
	}
	if ev["type"] != "text" || ev["text"] != "hello" {
		t.Errorf("chunk event = %v, want type=text text=hello", ev)
	}
}

func TestDrainPrintStream_StreamJSONMode_ToolCall(t *testing.T) {
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "tool_call", ToolName: "read_file", ToolArgs: `{"path":"main.go"}`},
		{Kind: "tool_exec", ToolName: "read_file", Status: "ok", DurationMs: 12},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	drainPrintStream(ch, "stream-json", &out, &errOut)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	// Should have tool_use, tool_result, result lines.
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 NDJSON lines, got %d: %s", len(lines), out.String())
	}
	var toolUse map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &toolUse); err != nil {
		t.Fatalf("tool_use line invalid JSON: %v", err)
	}
	if toolUse["type"] != "tool_use" {
		t.Errorf("expected type=tool_use, got %v", toolUse["type"])
	}
	if toolUse["name"] != "read_file" {
		t.Errorf("expected name=read_file, got %v", toolUse["name"])
	}

	var toolResult map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &toolResult); err != nil {
		t.Fatalf("tool_result line invalid JSON: %v", err)
	}
	if toolResult["type"] != "tool_result" {
		t.Errorf("expected type=tool_result, got %v", toolResult["type"])
	}
}

func TestDrainPrintStream_StreamJSONMode_Error(t *testing.T) {
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "error", ErrMsg: "rate limited"},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	code := drainPrintStream(ch, "stream-json", &out, &errOut)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	var errEv map[string]any
	if err := json.Unmarshal([]byte(strings.Split(strings.TrimSpace(out.String()), "\n")[0]), &errEv); err != nil {
		t.Fatalf("error event not valid JSON: %v", err)
	}
	if errEv["type"] != "error" {
		t.Errorf("expected type=error, got %v", errEv["type"])
	}
}

func TestDrainPrintStream_DefaultMode_ToolEventsIgnored(t *testing.T) {
	// tool_call and tool_exec in text mode should produce no output.
	ch := feedEvents([]tui.StreamEvent{
		{Kind: "tool_call", ToolName: "bash", ToolArgs: `{}`},
		{Kind: "tool_exec", ToolName: "bash", Status: "ok", DurationMs: 5},
		{Kind: "chunk", Chunk: "result"},
		{Kind: "done"},
	})
	var out, errOut bytes.Buffer
	drainPrintStream(ch, "text", &out, &errOut)
	if out.String() != "result" {
		t.Errorf("expected only 'result', got: %s", out.String())
	}
}

// ============================================================
// --- rateLimitDelay Tests (#83) ---
// ============================================================

func TestRateLimitDelay_Exponential(t *testing.T) {
	cases := []struct {
		retry int
		want  time.Duration
	}{
		{0, 10 * time.Second},
		{1, 20 * time.Second},
		{2, 40 * time.Second},
	}
	for _, tc := range cases {
		got := rateLimitDelay(tc.retry)
		if got != tc.want {
			t.Errorf("rateLimitDelay(%d) = %v, want %v", tc.retry, got, tc.want)
		}
	}
}

func TestRateLimitDelay_Capped(t *testing.T) {
	// retry=5: 10s * 32 = 320s > rateLimitMaxDelay (300s) → capped.
	got := rateLimitDelay(5)
	if got != rateLimitMaxDelay {
		t.Errorf("rateLimitDelay(5) = %v, want %v (capped at max)", got, rateLimitMaxDelay)
	}
}
