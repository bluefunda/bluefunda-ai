package agent

import (
	"io"
	"testing"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
)

func TestLocalTimezoneName_TZEnvOverride(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	if got := localTimezoneName(); got != "America/New_York" {
		t.Errorf("localTimezoneName() = %q, want %q", got, "America/New_York")
	}
}

func TestLocalTimezoneName_EmptyTZFallsBackToLocaltime(t *testing.T) {
	t.Setenv("TZ", "")
	// Can't control /etc/localtime in a test, so just assert it never returns
	// the misleading literal "Local" that time.Local.String() would produce.
	if got := localTimezoneName(); got == "Local" {
		t.Errorf("localTimezoneName() returned literal %q, want an IANA zone or empty string", got)
	}
}

// fakeChatStream feeds a canned sequence of ChatEvents. pumpStream only
// requires a Recv() method, so no gRPC/network mocking is needed.
type fakeChatStream struct {
	events []*pb.ChatEvent
	i      int
}

func (f *fakeChatStream) Recv() (*pb.ChatEvent, error) {
	if f.i >= len(f.events) {
		return nil, io.EOF
	}
	ev := f.events[f.i]
	f.i++
	return ev, nil
}

// Regression test: a "rate_limited" ChatEvent used to match no case in
// pumpStream's switch, so the loop fell through to EOF and returned a
// spurious end_turn success instead of surfacing the rejection.
func TestPumpStream_RateLimitedSurfacesAsError(t *testing.T) {
	stream := &fakeChatStream{events: []*pb.ChatEvent{
		{Type: "rate_limited", Content: "daily limit reached"},
	}}

	_, _, err := pumpStream(stream, func() {}, func(Event) {})
	if err == nil {
		t.Fatal("expected pumpStream to return an error for a rate_limited event, got nil")
	}
	if got := err.Error(); got != "rate limited: daily limit reached" {
		t.Errorf("unexpected error message: %q", got)
	}
}

func TestPumpStream_RateLimitedFallsBackToErrorField(t *testing.T) {
	stream := &fakeChatStream{events: []*pb.ChatEvent{
		{Type: "rate_limited", Error: "monthly limit reached"},
	}}

	_, _, err := pumpStream(stream, func() {}, func(Event) {})
	if err == nil || err.Error() != "rate limited: monthly limit reached" {
		t.Errorf("expected fallback to Error field, got %v", err)
	}
}
