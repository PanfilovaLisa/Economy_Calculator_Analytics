package cashflow

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"time"
)

func SaveToCSV(flows []CashFlow, outputDir string, sourceName string) error {
	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("не удалось создать папку %s: %w", outputDir, err)
	}

	now := time.Now().Format("2006-01-02_15-04-05")

	base := filepath.Base(sourceName)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = sanitizeFileName(base)

	suffix := ""
	if base != "" {
		suffix = "_" + base
	}

	fileName := fmt.Sprintf("cashflows_%s%s.csv", now, suffix)
	filePath := filepath.Join(outputDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("не удалось создать файл: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	headers := []string{
		"load_dttm", "source_cd", "discriminator", "deal_id", "currency_cd",
		"report_dttm",
		"start_dttm", "payment_dttm", "days_to_payment", "time_bucket",
		"payment_type", "principal_amt", "interest_amt", "remaining_balance",
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	for _, cf := range flows {
		record := []string{
		cf.LoadDate.Format("2006-01-02 15:04:05"),
		cf.SourceCode,
		cf.Discriminator,
		fmt.Sprintf("%d", cf.DealID),
		cf.Currency,
		cf.ReportDate.Format("02.01.2006"),
		cf.StartDate.Format("02.01.2006"),
		cf.PaymentDate.Format("02.01.2006"),
		fmt.Sprintf("%d", cf.DaysToPayment),
		cf.TimeBucket,
		cf.PaymentType,
		fmt.Sprintf("%.2f", cf.PrincipalAmount),
		fmt.Sprintf("%.2f", cf.InterestAmount),
		fmt.Sprintf("%.2f", cf.RemainingBalance),
	}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	fmt.Printf("История сохранена в файл: %s\n", filePath)
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