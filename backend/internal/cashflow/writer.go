package cashflow

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func SaveToCSV(flows []CashFlow, outputDir string) error {
	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("не удалось создать папку %s: %w", outputDir, err)
	}

	now := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("cashflows_%s.csv", now)
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
		"load_dttm", "source_cd", "deal_id", "currency_cd", 
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
			fmt.Sprintf("%d", cf.DealID),
			cf.Currency,
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