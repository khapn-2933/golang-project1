package service

import (
	"strings"
	"testing"

	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/dto"
)

func newTestChatworkService() *ChatworkNotificationService {
	cfg := &config.ChatworkConfig{
		APIToken:      "dummy-token",
		RoomID:        "12345",
		MessagePrefix: "🔔 Test",
	}
	return &ChatworkNotificationService{cfg: cfg}
}

// ─── sanitizeChatworkText ────────────────────────────────────────────────────

func TestSanitizeChatworkText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "plain text", in: "Hello", want: "Hello"},
		{name: "trim whitespace", in: "  hello  ", want: "hello"},
		{name: "escape brackets", in: "[bold]text[/bold]", want: "&#91;bold&#93;text&#91;/bold&#93;"},
		{name: "normalize newlines", in: "line1\nline2\r\nline3", want: "line1 line2 line3"},
		{name: "mixed brackets and newlines", in: "[info]\nhello", want: "&#91;info&#93; hello"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeChatworkText(tc.in)
			if got != tc.want {
				t.Fatalf("sanitizeChatworkText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ─── formatMessage ───────────────────────────────────────────────────────────

func TestFormatMessage_BasicOrder(t *testing.T) {
	t.Parallel()

	svc := newTestChatworkService()
	order := &dto.OrderResponse{
		OrderNumber:     "ORDER-20260318-1234",
		Status:          "pending",
		TotalAmount:     150000,
		ShippingPhone:   "0901234567",
		ShippingAddress: "123 Main St",
	}

	msg := svc.formatMessage(order)

	if !strings.Contains(msg, "ORDER-20260318-1234") {
		t.Errorf("message missing order number: %q", msg)
	}
	if !strings.Contains(msg, "pending") {
		t.Errorf("message missing status: %q", msg)
	}
	if !strings.Contains(msg, "150000.00") {
		t.Errorf("message missing total amount: %q", msg)
	}
	if !strings.Contains(msg, "[info]") {
		t.Errorf("message missing [info] tag: %q", msg)
	}
	if !strings.Contains(msg, "[/info]") {
		t.Errorf("message missing [/info] tag: %q", msg)
	}
}

func TestFormatMessage_WithNotesAndItems(t *testing.T) {
	t.Parallel()

	svc := newTestChatworkService()
	notes := "Please deliver in the morning"
	order := &dto.OrderResponse{
		OrderNumber:     "ORDER-001",
		Status:          "confirmed",
		TotalAmount:     250000,
		ShippingPhone:   "0912345678",
		ShippingAddress: "456 Street",
		Notes:           &notes,
		Items: []dto.OrderItemResponse{
			{ProductName: "Pho Bo", Quantity: 2, Subtotal: 80000},
			{ProductName: "Tra Sua", Quantity: 1, Subtotal: 30000},
		},
	}

	msg := svc.formatMessage(order)

	if !strings.Contains(msg, "Please deliver in the morning") {
		t.Errorf("message missing notes: %q", msg)
	}
	if !strings.Contains(msg, "Pho Bo") {
		t.Errorf("message missing item name: %q", msg)
	}
	if !strings.Contains(msg, "Tra Sua") {
		t.Errorf("message missing item name: %q", msg)
	}
	if !strings.Contains(msg, "Items:") {
		t.Errorf("message missing Items section: %q", msg)
	}
}

func TestFormatMessage_WithPrefix(t *testing.T) {
	t.Parallel()

	svc := newTestChatworkService()
	svc.cfg.MessagePrefix = "STORE ALERT"
	order := &dto.OrderResponse{
		OrderNumber:     "ORDER-999",
		Status:          "pending",
		TotalAmount:     50000,
		ShippingPhone:   "0900000000",
		ShippingAddress: "Test addr",
	}

	msg := svc.formatMessage(order)

	if !strings.Contains(msg, "STORE ALERT") {
		t.Errorf("message missing prefix: %q", msg)
	}
}

func TestFormatMessage_EscapesBracketsInOrderFields(t *testing.T) {
	t.Parallel()

	svc := newTestChatworkService()
	order := &dto.OrderResponse{
		OrderNumber:     "ORDER-[TEST]",
		Status:          "pending",
		TotalAmount:     0,
		ShippingPhone:   "0900",
		ShippingAddress: "[Building A] Floor 3",
	}

	msg := svc.formatMessage(order)

	// Brackets in user data should be escaped so they don't break Chatwork markup
	if strings.Contains(msg, "ORDER-[TEST]") {
		t.Errorf("raw brackets in order number not sanitized: %q", msg)
	}
	if strings.Contains(msg, "[Building A]") {
		t.Errorf("raw brackets in address not sanitized: %q", msg)
	}
}
