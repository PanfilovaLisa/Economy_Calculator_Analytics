package cashflow

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var cashFlowHeaders = []string{
	"deal_id", "currency", "discriminator", "rate_type",
	"report_dttm", "payment_type", "days_to_payment", "time_bucket",
	"start_dttm", "payment_dttm", "principal_amt", "interest_amt", "remaining_balance",
	"prev_rate", "current_rate",
}

type CSVWriter struct {
	f    *os.File
	w    *csv.Writer
	path string
}

func NewCSVWriter(dir, sourceName string) (*CSVWriter, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	now := time.Now().Format("2006-01-02_15-04-05.000")
	base := strings.TrimSuffix(filepath.Base(sourceName), filepath.Ext(sourceName))
	base = sanitizeFileName(base)

	suffix := ""
	if base != "" {
		suffix = "_" + base
	}
	path := filepath.Join(dir, fmt.Sprintf("cashflows_%s%s.csv", now, suffix))

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path, err)
	}

	w := csv.NewWriter(f)
	w.Comma = ';'

	if err := w.Write(cashFlowHeaders); err != nil {
		f.Close()
		return nil, fmt.Errorf("write header: %w", err)
	}

	return &CSVWriter{f: f, w: w, path: path}, nil
}

func (w *CSVWriter) Path() string { return w.path }

func (w *CSVWriter) WriteFlows(flows []CashFlow) error {
	for _, cf := range flows {
		if err := w.w.Write(cashFlowRecord(cf)); err != nil {
			return fmt.Errorf("write row (deal %d): %w", cf.DealID, err)
		}
	}
	return nil
}

func (w *CSVWriter) Close() error {
	w.w.Flush()
	flushErr := w.w.Error()

	syncErr := w.f.Sync()
	closeErr := w.f.Close()

	if flushErr != nil {
		return fmt.Errorf("flush: %w", flushErr)
	}
	if syncErr != nil {
		return fmt.Errorf("sync: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close: %w", closeErr)
	}
	return nil
}

func cashFlowRecord(cf CashFlow) []string {
	return []string{
		strconv.Itoa(cf.DealID),
		cf.Currency,
		cf.Discriminator,
		cf.RateType,
		cf.ReportDate.Format("02.01.2006"),
		cf.PaymentType,
		strconv.Itoa(cf.DaysToPayment),
		cf.TimeBucket,
		cf.StartDate.Format("02.01.2006"),
		cf.PaymentDate.Format("02.01.2006"),
		fmtMoney(cf.PrincipalAmount),
		fmtMoney(cf.InterestAmount),
		fmtMoney(cf.RemainingBalance),
		strings.Replace(strconv.FormatFloat(cf.PrevRate,'f', -1, 64), ".", ",", 1), // не нужно округлять проценты
		strings.Replace(strconv.FormatFloat(cf.CurrentRate,'f', -1, 64), ".", ",", 1),
	}
}

func SaveToCSV(flows []CashFlow, outputDir string, sourceName string) error {
	w, err := NewCSVWriter(outputDir, sourceName)
	if err != nil {
		return err
	}
	if err := w.WriteFlows(flows); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	fmt.Printf("История сохранена в файл: %s\n", w.Path())
	return nil
}

func sanitizeFileName(s string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		isSafe := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_'
		if isSafe {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore {
			b.WriteRune('_')
			prevUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func fmtMoney(x float64) string {
	return strings.Replace(strconv.FormatFloat(x, 'f', 2, 64), ".", ",", 1)
}
