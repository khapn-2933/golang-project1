package service

import "github.com/kha/foods-drinks/internal/dto"

type MultiOrderNotifier struct {
	notifiers []OrderNotifier
}

func NewMultiOrderNotifier(notifiers ...OrderNotifier) OrderNotifier {
	filtered := make([]OrderNotifier, 0, len(notifiers))
	for _, notifier := range notifiers {
		if notifier != nil {
			filtered = append(filtered, notifier)
		}
	}

	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return &MultiOrderNotifier{notifiers: filtered}
	}
}

func (n *MultiOrderNotifier) NotifyNewOrderAsync(order *dto.OrderResponse) {
	if n == nil {
		return
	}

	for _, notifier := range n.notifiers {
		notifier.NotifyNewOrderAsync(order)
	}
}
