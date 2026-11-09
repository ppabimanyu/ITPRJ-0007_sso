package csrf

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	items map[string]time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: map[string]time.Time{}}
}

func (s *memoryStore) Put(_ context.Context, hash string, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("ttl must be positive")
	}
	s.items[hash] = time.Now().Add(ttl)
	return nil
}

func (s *memoryStore) Exists(_ context.Context, hash string) (bool, error) {
	expires, ok := s.items[hash]
	if !ok || !expires.After(time.Now()) {
		return false, nil
	}
	return true, nil
}

func (s *memoryStore) Delete(_ context.Context, hash string) error {
	delete(s.items, hash)
	return nil
}

func TestIssueVerifyConsume(t *testing.T) {
	store := newMemoryStore()
	raw, err := Issue(context.Background(), store, time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if len(raw) < 40 {
		t.Fatalf("token is too short: %q", raw)
	}
	ok, err := Verify(context.Background(), store, raw)
	if err != nil || !ok {
		t.Fatalf("Verify() = %v, %v", ok, err)
	}
	if err := Consume(context.Background(), store, raw); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	ok, err = Verify(context.Background(), store, raw)
	if err != nil || ok {
		t.Fatalf("Verify() after Consume = %v, %v", ok, err)
	}
}

func TestVerifyRejectsWrongToken(t *testing.T) {
	store := newMemoryStore()
	if _, err := Issue(context.Background(), store, time.Minute); err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(context.Background(), store, "not-the-token")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Verify() accepted a token that was never issued")
	}
}

func TestEqual(t *testing.T) {
	if !Equal("abc", "abc") {
		t.Error("Equal() rejected identical tokens")
	}
	for _, pair := range [][2]string{{"abc", "abd"}, {"", ""}, {"a", "ab"}} {
		if Equal(pair[0], pair[1]) {
			t.Errorf("Equal(%q, %q) = true", pair[0], pair[1])
		}
	}
}

func TestMatchesHash(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !MatchesHash(raw, hash) {
		t.Error("MatchesHash() rejected the matching pair")
	}
	if MatchesHash(raw+"x", hash) {
		t.Error("MatchesHash() accepted a different token")
	}
}
