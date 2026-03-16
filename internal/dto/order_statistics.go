package dto

import (
	"net/url"
	"strings"
)

type AdminOrderStatisticsRequest struct {
	FromDate string `form:"from_date"`
	ToDate   string `form:"to_date"`
	Status   string `form:"status"`
	GroupBy  string `form:"group_by"`
}

type AdminOrderStatisticsPoint struct {
	PeriodLabel   string  `json:"period_label"`
	OrdersCount   int64   `json:"orders_count"`
	RevenueAmount float64 `json:"revenue_amount"`
}

type AdminOrderStatisticsSummary struct {
	OrdersCount    int64   `json:"orders_count"`
	RevenueAmount  float64 `json:"revenue_amount"`
	AverageOrder   float64 `json:"average_order"`
	DeliveredCount int64   `json:"delivered_count"`
	CancelledCount int64   `json:"cancelled_count"`
}

type AdminOrderStatisticsResponse struct {
	Summary AdminOrderStatisticsSummary `json:"summary"`
	Series  []AdminOrderStatisticsPoint `json:"series"`
}

func (q AdminOrderStatisticsRequest) URLParams() string {
	params := url.Values{}
	if strings.TrimSpace(q.FromDate) != "" {
		params.Set("from_date", strings.TrimSpace(q.FromDate))
	}
	if strings.TrimSpace(q.ToDate) != "" {
		params.Set("to_date", strings.TrimSpace(q.ToDate))
	}
	if strings.TrimSpace(q.Status) != "" {
		params.Set("status", strings.TrimSpace(q.Status))
	}
	if strings.TrimSpace(q.GroupBy) != "" {
		params.Set("group_by", strings.TrimSpace(q.GroupBy))
	}
	return params.Encode()
}
