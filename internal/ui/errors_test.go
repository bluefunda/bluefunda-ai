package ui

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcErr(code codes.Code, msg string) error {
	return status.Error(code, msg)
}

func TestRewriteError_Nil(t *testing.T) {
	if got := RewriteError(nil); got != "" {
		t.Errorf("expected empty string for nil error, got %q", got)
	}
}

func TestRewriteError_NonGRPC(t *testing.T) {
	err := errors.New("plain error")
	got := RewriteError(err)
	if got != "plain error" {
		t.Errorf("expected 'plain error', got %q", got)
	}
}

func TestRewriteError_Unauthenticated(t *testing.T) {
	got := RewriteError(grpcErr(codes.Unauthenticated, "token expired"))
	if !strings.Contains(got, "bai login") {
		t.Errorf("expected login hint, got %q", got)
	}
}

func TestRewriteError_PermissionDenied(t *testing.T) {
	got := RewriteError(grpcErr(codes.PermissionDenied, "forbidden"))
	if !strings.Contains(got, "subscription") {
		t.Errorf("expected subscription message, got %q", got)
	}
}

func TestRewriteError_Unavailable(t *testing.T) {
	got := RewriteError(grpcErr(codes.Unavailable, "connection refused"))
	if !strings.Contains(got, "bai doctor") {
		t.Errorf("expected doctor hint, got %q", got)
	}
}

func TestRewriteError_ResourceExhausted(t *testing.T) {
	got := RewriteError(grpcErr(codes.ResourceExhausted, "too many requests"))
	if !strings.Contains(got, "Rate limited") {
		t.Errorf("expected rate limit message, got %q", got)
	}
}

func TestRewriteError_DeadlineExceeded(t *testing.T) {
	got := RewriteError(grpcErr(codes.DeadlineExceeded, "timeout"))
	if !strings.Contains(got, "timed out") {
		t.Errorf("expected timeout message, got %q", got)
	}
}

func TestRewriteError_Internal_ContextLength(t *testing.T) {
	got := RewriteError(grpcErr(codes.Internal, "context length exceeded"))
	if !strings.Contains(got, "Context window") {
		t.Errorf("expected context window message, got %q", got)
	}
}

func TestRewriteError_Internal_Generic(t *testing.T) {
	got := RewriteError(grpcErr(codes.Internal, "something went wrong"))
	if !strings.Contains(got, "Internal server error") {
		t.Errorf("expected internal server error message, got %q", got)
	}
}

func TestRewriteError_Canceled(t *testing.T) {
	got := RewriteError(grpcErr(codes.Canceled, "request canceled"))
	if !strings.Contains(got, "canceled") {
		t.Errorf("expected canceled message, got %q", got)
	}
}

func TestRewriteError_Unknown_WithMessage(t *testing.T) {
	got := RewriteError(grpcErr(codes.NotFound, "resource not found"))
	if !strings.Contains(got, "resource not found") {
		t.Errorf("expected message passthrough, got %q", got)
	}
}
