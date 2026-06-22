package keychain

import (
	"testing"
)

// mockBackend is an in-memory keychain used in tests.
type mockBackend struct {
	store     map[string]string
	available bool
}

func newMock(available bool) *mockBackend {
	return &mockBackend{store: make(map[string]string), available: available}
}

func (m *mockBackend) Available() bool { return m.available }

func (m *mockBackend) Set(key, value string) error {
	m.store[key] = value
	return nil
}

func (m *mockBackend) Get(key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *mockBackend) Delete(key string) error {
	delete(m.store, key)
	return nil
}

func TestMockBackend_SetGet(t *testing.T) {
	mock := newMock(true)
	SetBackend(mock)
	defer SetBackend(&osBackend{})

	if err := Set("access_token", "tok123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := Get("access_token")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "tok123" {
		t.Errorf("got %q, want %q", got, "tok123")
	}
}

func TestMockBackend_GetMissing(t *testing.T) {
	mock := newMock(true)
	SetBackend(mock)
	defer SetBackend(&osBackend{})

	_, err := Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockBackend_Delete(t *testing.T) {
	mock := newMock(true)
	SetBackend(mock)
	defer SetBackend(&osBackend{})

	_ = Set("refresh_token", "ref456")
	if err := Delete("refresh_token"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := Get("refresh_token")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMockBackend_DeleteMissing(t *testing.T) {
	mock := newMock(true)
	SetBackend(mock)
	defer SetBackend(&osBackend{})

	// Deleting a non-existent key should not error.
	if err := Delete("nope"); err != nil {
		t.Errorf("Delete missing key: %v", err)
	}
}

func TestMockBackend_Unavailable(t *testing.T) {
	mock := newMock(false)
	SetBackend(mock)
	defer SetBackend(&osBackend{})

	if Available() {
		t.Error("expected Available() = false")
	}
}

func TestMockBackend_Overwrite(t *testing.T) {
	mock := newMock(true)
	SetBackend(mock)
	defer SetBackend(&osBackend{})

	_ = Set("k", "v1")
	_ = Set("k", "v2")
	got, _ := Get("k")
	if got != "v2" {
		t.Errorf("got %q after overwrite, want %q", got, "v2")
	}
}
