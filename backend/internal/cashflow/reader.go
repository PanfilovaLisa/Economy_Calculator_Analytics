package cashflow

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
)

// читает CSV-файл и возвращает срез сделок
func ReadDeals(file io.Reader) ([]Deal, error) {
	reader := csv.NewReader(file)
	// !Разделитель точка с запятой!
	reader.Comma = ';'
	reader.LazyQuotes = true // Нужно, чтобы парсер не падал на текстах

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения заголовков: %w", err)
	}

	// Создаем словарь: название колонки -> её индекс в строке
	colIndex := make(map[string]int)
	for i, header := range headers {
		colIndex[strings.TrimSpace(header)] = i
	}

	// Проверяем наличие критически важных колонок, без которых парсинг невозможен
	requiredCols := []string{"report_dttm", "deal_id", "currency_id", "current_principal_amt"}
	for _, col := range requiredCols {
		if _, ok := colIndex[col]; !ok {
			return nil, fmt.Errorf("ошибка структуры CSV: отсутствует обязательная колонка '%s'", col)
		}
	}

	deals := make([]Deal, 0, 1000)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения строки: %w", err)
		}

		// Пропускаем пустые строки
		if len(record) < 2 {
			continue
		}

		// Заполняем структуру Deal
		deal := Deal{
			ReportDate:      parseTime(record[colIndex["report_dttm"]]),
			Discriminator:   parseString(record[colIndex["discriminator"]]),
			DealID:          parseInt(record[colIndex["deal_id"]]),
			Currency:        parseString(record[colIndex["currency_id"]]),
			
			StartPrincipalAmt:   parseFloatPtr(record[colIndex["start_principal_amt"]]),
			CurrentPrincipalAmt: parseFloat(record[colIndex["current_principal_amt"]]),

			DealDate:     parseTime(record[colIndex["deal_dttm"]]),
			MaturityDate: parseTime(record[colIndex["maturity_dttm"]]),
			DealReportDt: parseTime(record[colIndex["deal_report_dttm"]]),

			RateType:          parseString(record[colIndex["rate_type_cd"]]),
			InterestRate:      parseFloat(record[colIndex["interest_rate"]]),
			BaseIndicator:     parseString(record[colIndex["base_indicator"]]),
			BaseIndicatorRate: parseFloatPtr(record[colIndex["base_indicator_rate"]]),
			DayConvention:     parseString(record[colIndex["day_convention_cd"]]),
			AccruedInterest:   parseFloatPtr(record[colIndex["accrued_interest"]]),

			InterestPayStart:  parseTimePtr(record[colIndex["interest_payment_rule_start_dttm"]]),
			InterestPayPeriod: parseString(record[colIndex["interest_payment_rule_period_cd"]]),
			InterestPayDayNum: parseIntPtr(record[colIndex["interest_payment_rule_day_num"]]),

			RegularPayStart:  parseTimePtr(record[colIndex["regular_payment_rule_start_dttm"]]),
			RegularPayPeriod: parseString(record[colIndex["regular_payment_rule_period_cd"]]),
			RegularPayDayNum: parseIntPtr(record[colIndex["regular_payment_rule_day_num"]]),

			RepricingRuleStart:    parseTimePtr(record[colIndex["repricing_rule_start_dttm"]]),
			RepricingRulePeriod:   parseString(record[colIndex["repricing_rule_period_cd"]]),
			RepricingRuleDayNum:   parseIntPtr(record[colIndex["repricing_rule_day_num"]]),
			RepricingDttmSchedule: parseString(record[colIndex["repricing_dttm_schedule"]]),
			RepricingRateSchedule: parseString(record[colIndex["repricing_dttm_rate_schedule"]]),
			RepricingRateFixed:    parseString(record[colIndex["repricing_dttm_rate_schedule_fixed"]]),

			Product:         parseString(record[colIndex["product"]]),
			BehaviorModelCd: parseString(record[colIndex["behavior_model_cd"]]),

			ManualInterestPayment:  parseString(record[colIndex["interest_payment"]]),
			ManualPrincipalPayment: parseString(record[colIndex["principal_payment"]]),

			CapLimitInterestRate:   parseFloatPtr(record[colIndex["cap_limit_interest_rate"]]),
			FloorLimitInterestRate: parseFloatPtr(record[colIndex["floor_limit_interest_rate"]]),

			SubRateTypeCd:             parseString(record[colIndex["sub_rate_type_cd"]]),
			SubInterestRate:           parseFloatPtr(record[colIndex["sub_interest_rate"]]),
			SubFloatIndicator:         parseString(record[colIndex["sub_float_indicator"]]),
			SubPaymentRulePeriodCd:    parseString(record[colIndex["sub_payment_rule_period_cd"]]),
			SubPaymentRuleDayNum:      parseIntPtr(record[colIndex["sub_payment_rule_day_num"]]),
			SubPaymentRuleStartDttm:   parseTimePtr(record[colIndex["sub_payment_rule_start_dttm"]]),
			SubRepricingRuleStartDttm: parseTimePtr(record[colIndex["sub_repricing_rule_start_dttm"]]),
			SubRepricingRulePeriodCd:  parseString(record[colIndex["sub_repricing_rule_period_cd"]]),
			SubRepricingRuleDayNum:    parseIntPtr(record[colIndex["sub_repricing_rule_day_num"]]),
			SubCapLimitInterestRate:   parseFloatPtr(record[colIndex["sub_cap_limit_interest_rate"]]),
			SubFloorLimitInterestRate: parseFloatPtr(record[colIndex["sub_floor_limit_interest_rate"]]),
			SubCoeffIndex:             parseFloatPtr(record[colIndex["sub_coeff_index"]]),
			SubRepricingFirstEventFlg: parseIntPtr(record[colIndex["sub_repricing_rule_first_event_flg"]]),

			IndicatorLagCd:    parseString(record[colIndex["indicator_lag_cd"]]),
			SubIndicatorLagCd: parseString(record[colIndex["sub_indicator_lag_cd"]]),
		}

		deals = append(deals, deal)
	}

	return deals, nil
}

// удаляет лишние пробелы в начале и конце строки
func parseString(s string) string {
	return strings.TrimSpace(s)
}

// превращает строку "31.08.2022" в time.Time
func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	
	// Формат строго: ДД.ММ.ГГГГ
	t, err := time.Parse("02.01.2006", s)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга даты '%s': %v", s, err)
		return time.Time{}
	}
	return t
}

// превращает строку в *time.Time (если пусто или ошибка - nil)
func parseTimePtr(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := time.Parse("02.01.2006", s)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга даты '%s': %v", s, err)
		return nil
	}
	return &t
}

// превращает строку в float64 (если пусто - 0.0, при ошибке логирует)
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0.0
	}
	// Защита от запятых в числах ("3646037,89")
	s = strings.Replace(s, ",", ".", -1)
	
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга числа '%s': %v", s, err)
		return 0.0
	}
	return f
}

// превращает строку в *float64 (если пусто или ошибка - nil)
func parseFloatPtr(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.Replace(s, ",", ".", -1) // Защита от запятых
	
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга числа '%s': %v", s, err)
		return nil
	}
	return &f
}

// превращает строку в int (если пусто - 0, при ошибке логирует)
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга целого числа '%s': %v", s, err)
		return 0
	}
	return i
}

// превращает строку в *int (если пусто или ошибка - nil)
func parseIntPtr(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга целого числа '%s': %v", s, err)
		return nil
	}
	return &i
}