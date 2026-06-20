package ui

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RewriteError converts raw gRPC status errors into user-actionable messages.
// Non-gRPC errors pass through unchanged.
func RewriteError(err error) string {
	if err == nil {
		return ""
	}
	st, ok := status.FromError(err)
	if !ok {
		return err.Error()
	}
	switch st.Code() {
	case codes.Unauthenticated:
		return "Not authenticated. Run: bai login"
	case codes.PermissionDenied:
		return "Access denied. Your subscription may not include this feature."
	case codes.Unavailable:
		return "Cannot reach BlueFunda. Check your network or run: bai doctor"
	case codes.ResourceExhausted:
		return "Rate limited by the server. Wait a moment and try again."
	case codes.DeadlineExceeded:
		return "Request timed out. The server may be busy — try again shortly."
	case codes.Internal:
		msg := st.Message()
		if strings.Contains(strings.ToLower(msg), "context length") ||
			strings.Contains(strings.ToLower(msg), "context window") ||
			strings.Contains(strings.ToLower(msg), "token") {
			return "Context window exceeded. Start a new session with: bai --new"
		}
		return "Internal server error: " + msg
	case codes.Canceled:
		return "Request canceled."
	default:
		if msg := st.Message(); msg != "" {
			return msg
		}
		return err.Error()
	}
}
