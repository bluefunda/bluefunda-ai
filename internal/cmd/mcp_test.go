package cmd

import (
	"context"
	"fmt"
	"testing"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
)

func TestSelectMcpServers_AllSucceed(t *testing.T) {
	var got []string
	results := selectMcpServers([]string{"a", "b", "c"}, true, func(name string, subscribe bool) error {
		got = append(got, name)
		if !subscribe {
			t.Errorf("expected subscribe=true for %s", name)
		}
		return nil
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.success {
			t.Errorf("expected %s to succeed, got err=%v", r.name, r.err)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected selectFn called 3 times, got %d", len(got))
	}
}

func TestSelectMcpServers_ContinuesPastFailure(t *testing.T) {
	results := selectMcpServers([]string{"good", "bad", "also-good"}, false, func(name string, subscribe bool) error {
		if name == "bad" {
			return fmt.Errorf("boom")
		}
		return nil
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results (continuing past the failure), got %d", len(results))
	}
	if !results[0].success || results[0].name != "good" {
		t.Errorf("expected 'good' to succeed, got %+v", results[0])
	}
	if results[1].success || results[1].name != "bad" {
		t.Errorf("expected 'bad' to fail, got %+v", results[1])
	}
	if !results[2].success || results[2].name != "also-good" {
		t.Errorf("expected 'also-good' to succeed despite the earlier failure, got %+v", results[2])
	}
}

func TestSelectMcpServers_Empty(t *testing.T) {
	results := selectMcpServers(nil, true, func(name string, subscribe bool) error {
		t.Fatal("selectFn should not be called for an empty name list")
		return nil
	})
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestSelectMcpRPC_SubscribeTrueForAdd(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.SelectMcp(ctx, &pb.SelectMcpRequest{McpInfo: &pb.MCPInfo{Name: "test-mcp", Subscribe: true}})
	if err != nil {
		t.Fatalf("SelectMcp: %v", err)
	}
	if !resp.GetSuccess() {
		t.Errorf("expected success, got error: %s", resp.GetError())
	}
}

func TestSelectMcpRPC_SubscribeFalseForRemove(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.SelectMcp(ctx, &pb.SelectMcpRequest{McpInfo: &pb.MCPInfo{Name: "test-mcp", Subscribe: false}})
	if err != nil {
		t.Fatalf("SelectMcp: %v", err)
	}
	if !resp.GetSuccess() {
		t.Errorf("expected success, got error: %s", resp.GetError())
	}
}

func TestSelectMcpRPC_UnknownServerReturnsError(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.SelectMcp(ctx, &pb.SelectMcpRequest{McpInfo: &pb.MCPInfo{Name: "bad-server", Subscribe: true}})
	if err != nil {
		t.Fatalf("SelectMcp: %v", err)
	}
	if resp.GetSuccess() {
		t.Error("expected success=false for an unknown server")
	}
	if resp.GetError() == "" {
		t.Error("expected a non-empty error message")
	}
}
