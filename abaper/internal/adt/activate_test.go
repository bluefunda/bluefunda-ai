package adt

import (
	"net/http"
	"testing"
)

// TestActivateObject_Success covers the standalone ActivateObject method
// (used by the REST server's /api/v1/activate route, and thus by `abaper
// deploy`) with no messages at all — a clean pass.
func TestActivateObject_Success(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(activationMessagesXML("", "")))
	})

	result, err := c.ActivateObject(t.Context(), "program", "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if len(result.Messages) != 0 {
		t.Errorf("expected no messages, got %+v", result.Messages)
	}
}

// TestActivateObject_ErrorMessage is the exact live repro from this
// session: HTTP 200, but the checklist body carries a real compiler error
// (unknown field reference) — ActivateObject must report Success=false with
// the real message text, not a false positive.
func TestActivateObject_ErrorMessage(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // SAP answers 200 even when activation failed.
		_, _ = w.Write([]byte(activationMessagesXML("E", `No component exists with the name "CARRID".`)))
	})

	result, err := c.ActivateObject(t.Context(), "program", "ZFOO")
	if err != nil {
		t.Fatalf("unexpected transport-level error: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false for a type=E message")
	}
	if len(result.Messages) != 1 || result.Messages[0].Text != `No component exists with the name "CARRID".` {
		t.Errorf("expected the real error text to survive, got: %+v", result.Messages)
	}
}

// TestActivateObject_WarningOnlyStillSucceeds confirms a warning-severity
// message (e.g. "Activation was cancelled... Editing canceled") alone does
// not flip Success to false — only E/A severities do.
func TestActivateObject_WarningOnlyStillSucceeds(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(activationMessagesXML("W", "Activation was cancelled.")))
	})

	result, err := c.ActivateObject(t.Context(), "program", "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected a warning-only message to still count as success")
	}
}

func TestActivateObject_HTTPFailure(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	})

	result, err := c.ActivateObject(t.Context(), "program", "ZFOO")
	if err != nil {
		t.Fatalf("ActivateObject itself should not return a Go error for a non-transport HTTP failure: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false for a non-200 response")
	}
	if len(result.Messages) == 0 {
		t.Error("expected a synthesized error message when the response body had none")
	}
}

func TestActivateObject_UnsupportedType(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request expected for an unsupported object type")
	})
	if _, err := c.ActivateObject(t.Context(), "bogus_type", "ZFOO"); err == nil {
		t.Fatal("expected an error for an unsupported object type")
	}
}

func TestActivateObject_NotAuthenticated(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request should be made when not authenticated")
	})
	if _, err := c.ActivateObject(t.Context(), "program", "ZFOO"); err == nil {
		t.Fatal("expected an error")
	}
}

// TestActivateWithRetry_SucceedsOnFirstAttempt covers the chkl:messages
// retry-loop path used internally by CreateInterface/CreateFunctionGroup/
// CreateInclude/CreateStructure/CreateTable (createAndPopulate ->
// activateWithRetry), for the immediate-success case (0 delay).
func TestActivateWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	attempts := 0
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(activationChecklistSuccessBody))
	})

	if err := c.activateWithRetry(t.Context(), "/sap/bc/adt/oo/interfaces/zfoo", "ZFOO"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt for an immediate success, got %d", attempts)
	}
}

// TestActivateWithRetry_ConfirmsAfterNotExecuted reproduces the real,
// documented race: SAP answers checkExecuted="false" (no messages at all)
// on the first attempt or two — because activation was attempted moments
// after unlocking a just-created object — then genuinely runs the check on
// a later attempt. This must be treated as "keep retrying," not success.
func TestActivateWithRetry_ConfirmsAfterNotExecuted(t *testing.T) {
	attempts := 0
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
		if attempts < 3 {
			_, _ = w.Write([]byte(activationChecklistNotExecutedBody))
			return
		}
		_, _ = w.Write([]byte(activationChecklistSuccessBody))
	})

	if err := c.activateWithRetry(t.Context(), "/sap/bc/adt/oo/interfaces/zfoo", "ZFOO"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected activation to be confirmed on the 3rd attempt, got %d attempts", attempts)
	}
}

// TestActivateWithRetry_NeverConfirmed exhausts all retry attempts without
// SAP ever actually running the check, and must report that distinctly
// rather than silently succeeding. This is the slowest test in the package
// (~5s, the full activationRetryDelays backoff) since it must exhaust every
// attempt.
func TestActivateWithRetry_NeverConfirmed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow retry-exhaustion test in -short mode")
	}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(activationChecklistNotExecutedBody))
	})

	err := c.activateWithRetry(t.Context(), "/sap/bc/adt/oo/interfaces/zfoo", "ZFOO")
	if err == nil {
		t.Fatal("expected an error when SAP never confirms the check ran")
	}
}

// TestActivateWithRetry_ErrorStopsRetrying confirms a genuine type=E error
// message (once the check has run) is reported immediately rather than
// exhausting the retry budget.
func TestActivateWithRetry_ErrorStopsRetrying(t *testing.T) {
	attempts := 0
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(activationChecklistErrorBody))
	})

	err := c.activateWithRetry(t.Context(), "/sap/bc/adt/ddic/srvd/sources/zfoo", "ZFOO")
	if err == nil {
		t.Fatal("expected an error")
	}
	if attempts != 1 {
		t.Errorf("expected a genuine error to stop retrying immediately, got %d attempts", attempts)
	}
}
