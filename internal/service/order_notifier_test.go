package service

import (
	"sync/atomic"
	"testing"

	"github.com/kha/foods-drinks/internal/dto"
)

// stubNotifier is a simple OrderNotifier that counts how many times it was called.
type stubNotifier struct {
	count int64
}

func (s *stubNotifier) NotifyNewOrderAsync(_ *dto.OrderResponse) {
	atomic.AddInt64(&s.count, 1)
}

func TestNewMultiOrderNotifier_NilWhenEmpty(t *testing.T) {
	t.Parallel()

	if got := NewMultiOrderNotifier(); got != nil {
		t.Fatalf("expected nil for no notifiers, got %T", got)
	}
}

func TestNewMultiOrderNotifier_NilFiltered(t *testing.T) {
	t.Parallel()

	if got := NewMultiOrderNotifier(nil, nil); got != nil {
		t.Fatalf("expected nil when all notifiers are nil, got %T", got)
	}
}

func TestNewMultiOrderNotifier_SingleReturnsDirectly(t *testing.T) {
	t.Parallel()

	n := &stubNotifier{}
	got := NewMultiOrderNotifier(nil, n, nil)
	if got != OrderNotifier(n) {
		t.Fatalf("expected the single non-nil notifier to be returned directly")
	}
}

func TestNewMultiOrderNotifier_MultipleNotifiersAllCalled(t *testing.T) {
	t.Parallel()

	a := &stubNotifier{}
	b := &stubNotifier{}
	multi := NewMultiOrderNotifier(a, nil, b)
	if multi == nil {
		t.Fatal("expected non-nil multi notifier")
	}

	order := &dto.OrderResponse{OrderNumber: "ORDER-001"}
	multi.NotifyNewOrderAsync(order)

	if a.count != 1 {
		t.Fatalf("notifier a called %d times, want 1", a.count)
	}
	if b.count != 1 {
		t.Fatalf("notifier b called %d times, want 1", b.count)
	}
}
