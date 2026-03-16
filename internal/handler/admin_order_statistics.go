package handler

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/service"
)

const (
	adminOrderStatsMenu  = "order_statistics"
	adminOrderStatsTitle = "Thống kê đơn hàng"
	adminOrderStatsTpl   = "order_statistics"
)

type AdminOrderStatisticsHandler struct {
	orderService *service.OrderService
	statsTmpl    *template.Template
}

func NewAdminOrderStatisticsHandler(orderService *service.OrderService, funcMap template.FuncMap) *AdminOrderStatisticsHandler {
	layout := "templates/admin/layout.html"
	return &AdminOrderStatisticsHandler{
		orderService: orderService,
		statsTmpl: template.Must(
			template.New(adminOrderStatsTpl).Funcs(funcMap).ParseFiles(layout, "templates/admin/orders/statistics.html"),
		),
	}
}

func (h *AdminOrderStatisticsHandler) render(c *gin.Context, status int, tmpl *template.Template, data gin.H) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		_ = c.Error(err)
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}

func (h *AdminOrderStatisticsHandler) List(c *gin.Context) {
	q := dto.AdminOrderStatisticsRequest{
		Status:   strings.TrimSpace(c.Query("status")),
		FromDate: strings.TrimSpace(c.Query("from_date")),
		ToDate:   strings.TrimSpace(c.Query("to_date")),
		GroupBy:  strings.TrimSpace(c.DefaultQuery("group_by", "month")),
	}

	result, err := h.orderService.GetStatisticsForAdmin(&q)
	if err != nil {
		h.render(c, http.StatusInternalServerError, h.statsTmpl, gin.H{
			"Title":       adminOrderStatsTitle,
			"ActiveMenu":  adminOrderStatsMenu,
			"Flash":       &flash{Type: flashTypeErr, Message: "Lỗi khi tải dữ liệu thống kê: " + err.Error()},
			"Query":       q,
			"Summary":     dto.AdminOrderStatisticsSummary{},
			"Series":      []dto.AdminOrderStatisticsPoint{},
			"LabelsJSON":  template.JS("[]"),
			"OrdersJSON":  template.JS("[]"),
			"RevenueJSON": template.JS("[]"),
		})
		return
	}

	labels, orders, revenue := buildChartData(result.Series)

	h.render(c, http.StatusOK, h.statsTmpl, gin.H{
		"Title":       adminOrderStatsTitle,
		"ActiveMenu":  adminOrderStatsMenu,
		"Query":       q,
		"Summary":     result.Summary,
		"Series":      result.Series,
		"LabelsJSON":  labels,
		"OrdersJSON":  orders,
		"RevenueJSON": revenue,
	})
}

func buildChartData(points []dto.AdminOrderStatisticsPoint) (template.JS, template.JS, template.JS) {
	labels := make([]string, 0, len(points))
	orders := make([]int64, 0, len(points))
	revenue := make([]float64, 0, len(points))

	for _, point := range points {
		labels = append(labels, point.PeriodLabel)
		orders = append(orders, point.OrdersCount)
		revenue = append(revenue, point.RevenueAmount)
	}

	labelsJSON := mustJSON(labels)
	ordersJSON := mustJSON(orders)
	revenueJSON := mustJSON(revenue)

	return template.JS(labelsJSON), template.JS(ordersJSON), template.JS(revenueJSON)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
