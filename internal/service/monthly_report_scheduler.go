package service

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/robfig/cron/v3"
)

const defaultMonthlyReportTemplatePath = "templates/email/monthly_report.html"

// MonthlyReportScheduler schedules and sends the monthly order statistics email report.
type MonthlyReportScheduler struct {
	cfg          *config.SchedulerConfig
	emailCfg     *config.EmailConfig
	orderService *OrderService
	tpl          *template.Template
	c            *cron.Cron
}

// monthlyReportData is the template data passed to monthly_report.html.
type monthlyReportData struct {
	MonthLabel string
	Summary    dto.AdminOrderStatisticsSummary
	Series     []dto.AdminOrderStatisticsPoint
}

// NewMonthlyReportScheduler creates a scheduler but does not start it yet.
// Returns nil when cfg is nil or disabled.
func NewMonthlyReportScheduler(
	cfg *config.SchedulerConfig,
	emailCfg *config.EmailConfig,
	orderService *OrderService,
) *MonthlyReportScheduler {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if emailCfg == nil || !emailCfg.Enabled {
		log.Println("[scheduler] monthly report disabled: email is not configured")
		return nil
	}

	tplPath := strings.TrimSpace(cfg.ReportTemplatePath)
	if tplPath == "" {
		tplPath = defaultMonthlyReportTemplatePath
	}

	funcMap := template.FuncMap{
		"formatVND": formatVNDEmail,
		"isEven": func(i int) bool {
			return i%2 == 0
		},
	}

	tpl, err := template.New("monthly_report").Funcs(funcMap).ParseFiles(tplPath)
	if err != nil {
		log.Printf("[scheduler] failed to parse monthly report template %q: %v", tplPath, err)
		return nil
	}

	cronExpr := strings.TrimSpace(cfg.MonthlyCron)
	if cronExpr == "" {
		// Default: 05:00 on the 1st of every month.
		cronExpr = "0 5 1 * *"
	}

	return &MonthlyReportScheduler{
		cfg:          cfg,
		emailCfg:     emailCfg,
		orderService: orderService,
		tpl:          tpl,
		c:            cron.New(),
	}
}

// Start registers the cron job and begins the scheduler.
func (s *MonthlyReportScheduler) Start() {
	if s == nil {
		return
	}

	cronExpr := strings.TrimSpace(s.cfg.MonthlyCron)
	if cronExpr == "" {
		cronExpr = "0 5 1 * *"
	}

	_, err := s.c.AddFunc(cronExpr, func() {
		// Report on the previous month.
		s.RunReport(time.Now().AddDate(0, -1, 0))
	})
	if err != nil {
		log.Printf("[scheduler] failed to register monthly report cron %q: %v", cronExpr, err)
		return
	}

	s.c.Start()
	log.Printf("[scheduler] monthly report cron started with expression %q", cronExpr)
}

// Stop gracefully stops the scheduler.
func (s *MonthlyReportScheduler) Stop() {
	if s == nil {
		return
	}
	ctx := s.c.Stop()
	<-ctx.Done()
}

// RunReport generates and sends the statistics report for the month that contains t.
// It is exported so it can be triggered manually (e.g. in tests or one-off scripts).
func (s *MonthlyReportScheduler) RunReport(t time.Time) {
	start := time.Now()
	month := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	monthLabel := month.Format("01/2006")

	log.Printf("[scheduler] generating monthly report for %s", monthLabel)

	fromDate := month.Format("2006-01-02")
	lastDay := month.AddDate(0, 1, 0).Add(-time.Second)
	toDate := lastDay.Format("2006-01-02")

	req := &dto.AdminOrderStatisticsRequest{
		FromDate: fromDate,
		ToDate:   toDate,
		GroupBy:  "week",
	}

	resp, err := s.orderService.GetStatisticsForAdmin(req)
	if err != nil {
		log.Printf("[scheduler] monthly report %s: failed to get statistics: %v (elapsed %s)", monthLabel, err, time.Since(start))
		return
	}

	htmlBody, err := s.renderReport(monthLabel, resp)
	if err != nil {
		log.Printf("[scheduler] monthly report %s: failed to render template: %v (elapsed %s)", monthLabel, err, time.Since(start))
		return
	}

	subject := s.buildSubject(monthLabel)
	if err := s.sendHTMLEmail(subject, htmlBody); err != nil {
		log.Printf("[scheduler] monthly report %s: failed to send email: %v (elapsed %s)", monthLabel, err, time.Since(start))
		return
	}

	log.Printf("[scheduler] monthly report %s sent successfully (elapsed %s)", monthLabel, time.Since(start))
}

func (s *MonthlyReportScheduler) renderReport(monthLabel string, resp *dto.AdminOrderStatisticsResponse) (string, error) {
	data := monthlyReportData{
		MonthLabel: monthLabel,
		Summary:    resp.Summary,
		Series:     resp.Series,
	}
	var buf bytes.Buffer
	tplName := s.tpl.Templates()[0].Name()
	if err := s.tpl.ExecuteTemplate(&buf, tplName, data); err != nil {
		return "", fmt.Errorf("template execution: %w", err)
	}
	return buf.String(), nil
}

func (s *MonthlyReportScheduler) buildSubject(monthLabel string) string {
	prefix := strings.TrimSpace(s.emailCfg.SubjectPrefix)
	if prefix == "" {
		prefix = "[Foods & Drinks]"
	}
	return fmt.Sprintf("%s Báo cáo thống kê tháng %s", prefix, monthLabel)
}

func (s *MonthlyReportScheduler) sendHTMLEmail(subject, htmlBody string) error {
	host := strings.TrimSpace(s.emailCfg.SMTPHost)
	port := s.emailCfg.SMTPPort
	if host == "" || port <= 0 {
		return fmt.Errorf("invalid smtp config")
	}

	fromEmail := strings.TrimSpace(s.emailCfg.FromEmail)
	if fromEmail == "" {
		return fmt.Errorf("from_email is required")
	}

	recipient := strings.TrimSpace(s.cfg.AdminRecipient)
	if recipient == "" {
		recipient = strings.TrimSpace(s.emailCfg.AdminRecipient)
	}
	if recipient == "" {
		return fmt.Errorf("admin_recipient is required for monthly report")
	}

	fromHeader := fromEmail
	if name := strings.TrimSpace(s.emailCfg.FromName); name != "" {
		fromHeader = fmt.Sprintf("%s <%s>", name, fromEmail)
	}

	headers := []string{
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"From: " + fromHeader,
		"To: " + recipient,
		"Subject: " + subject,
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody

	username := strings.TrimSpace(s.emailCfg.Username)
	password := strings.TrimSpace(s.emailCfg.Password)
	var auth smtp.Auth
	if username != "" || password != "" {
		if username == "" || password == "" {
			return fmt.Errorf("both smtp username and password are required when smtp auth is enabled")
		}
		auth = smtp.PlainAuth("", username, password, host)
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	return smtp.SendMail(addr, auth, fromEmail, []string{recipient}, []byte(message))
}

// formatVNDEmail formats a float64 amount as Vietnamese currency string (e.g. "1.250.000đ").
// This is a local copy used in email templates, keeping the email package self-contained.
func formatVNDEmail(amount float64) string {
	value := int64(math.Round(amount))
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}

	raw := strconv.FormatInt(value, 10)
	n := len(raw)
	if n <= 3 {
		return sign + raw + "đ"
	}

	sepCount := (n - 1) / 3
	buf := make([]byte, n+sepCount)
	read := n - 1
	write := len(buf) - 1
	digitCount := 0

	for read >= 0 {
		buf[write] = raw[read]
		read--
		write--
		digitCount++

		if digitCount%3 == 0 && read >= 0 {
			buf[write] = '.'
			write--
		}
	}

	return sign + string(buf) + "đ"
}
