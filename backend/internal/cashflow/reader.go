package cashflow

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"
)

var requiredDealCols = []string{
	"report_dttm", "deal_id", "currency_id", "current_principal_amt",
}

func StreamDeals(ctx context.Context, r io.Reader, out chan<- Deal, maxRowErrs int) (rowErrs []error, err error) {
	cr := csv.NewReader(r)
	cr.Comma = ';'
	cr.ReuseRecord = true
	cr.FieldsPerRecord = -1

	headers, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	col := make(map[string]int, len(headers))
	for i, h := range headers {
		col[strings.TrimSpace(h)] = i
	}
	for _, req := range requiredDealCols {
		if _, ok := col[req]; !ok {
			return nil, fmt.Errorf("missing required column %q", req)
		}
	}

	for line := 2; ; line++ {
		select {
		case <-ctx.Done():
			return rowErrs, ctx.Err()
		default:
		}

		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return rowErrs, fmt.Errorf("line %d: %w", line, err)
		}
		if len(rec) != len(headers) {
			if len(rowErrs) < maxRowErrs {
				rowErrs = append(rowErrs, fmt.Errorf("line %d: field count %d != %d", line, len(rec), len(headers)))
			}
			continue
		}

		d := dealFromRecord(rec, col)

		select {
		case out <- d:
		case <-ctx.Done():
			return rowErrs, ctx.Err()
		}
	}
	return rowErrs, nil
}

func ReadDeals(r io.Reader) ([]Deal, error) {
	ctx := context.Background()
	ch := make(chan Deal, 256)
	var deals []Deal
	done := make(chan struct{})
	go func() {
		defer close(done)
		for d := range ch {
			deals = append(deals, d)
		}
	}()

	rowErrs, err := StreamDeals(ctx, r, ch, 1000)
	close(ch)
	<-done

	if len(rowErrs) > 0 {
		log.Printf("ReadDeals: %d строк пропущено (первые: %v)", len(rowErrs), rowErrs[0])
	}
	return deals, err
}

func dealFromRecord(rec []string, col map[string]int) Deal {
	get := func(name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return rec[i]
		}
		return ""
	}

	return Deal{
		ReportDate:    parseTime(get("report_dttm")),
		Discriminator: parseCode("discriminator", get("discriminator")),
		DealID:        parseInt(get("deal_id")),
		Currency:      parseString(get("currency_id")),

		StartPrincipalAmt:   parseFloatPtr(get("start_principal_amt")),
		CurrentPrincipalAmt: parseFloat(get("current_principal_amt")),

		DealDate:     parseTime(get("deal_dttm")),
		MaturityDate: parseTime(get("maturity_dttm")),
		DealReportDt: parseTime(get("deal_report_dttm")),

		RateType:          parseCode("rate_type_cd", get("rate_type_cd")),
		InterestRate:      parseFloat(get("interest_rate")),
		BaseIndicator:     parseString(get("base_indicator")),
		BaseIndicatorRate: parseFloatPtr(get("base_indicator_rate")),
		DayConvention:     parseString(get("day_convention_cd")),
		AccruedInterest:   parseFloatPtr(get("accrued_interest")),

		InterestPayStart:  parseTimePtr(get("interest_payment_rule_start_dttm")),
		InterestPayPeriod: parseString(get("interest_payment_rule_period_cd")),
		InterestPayDayNum: parseIntPtr(get("interest_payment_rule_day_num")),

		RegularPayStart:  parseTimePtr(get("regular_payment_rule_start_dttm")),
		RegularPayPeriod: parseString(get("regular_payment_rule_period_cd")),
		RegularPayDayNum: parseIntPtr(get("regular_payment_rule_day_num")),

		RepricingRuleStart:  parseTimePtr(get("repricing_rule_start_dttm")),
		RepricingRulePeriod: parseString(get("repricing_rule_period_cd")),
		RepricingRuleDayNum: parseIntPtr(get("repricing_rule_day_num")),
		RepricingDttmFloat:  parseString(get("repricing_dttm_schedule")),
		RepricingRateFloat:  parseString(get("repricing_dttm_rate_schedule")),
		RepricingRateFixed:  parseString(get("repricing_dttm_rate_schedule_fixed")),

		Product:         parseString(get("product")),
		BehaviorModelCd: parseString(get("behavior_model_cd")),

		ManualInterestPayment:  parseString(get("interest_payment")),
		ManualPrincipalPayment: parseString(get("principal_payment")),

		CapLimitInterestRate:   parseFloatPtr(get("cap_limit_interest_rate")),
		FloorLimitInterestRate: parseFloatPtr(get("floor_limit_interest_rate")),

		SubRateTypeCd:             parseString(get("sub_rate_type_cd")),
		SubInterestRate:           parseFloatPtr(get("sub_interest_rate")),
		SubFloatIndicator:         parseString(get("sub_float_indicator")),
		SubPaymentRulePeriodCd:    parseString(get("sub_payment_rule_period_cd")),
		SubPaymentRuleDayNum:      parseIntPtr(get("sub_payment_rule_day_num")),
		SubPaymentRuleStartDttm:   parseTimePtr(get("sub_payment_rule_start_dttm")),
		SubRepricingRuleStartDttm: parseTimePtr(get("sub_repricing_rule_start_dttm")),
		SubRepricingRulePeriodCd:  parseString(get("sub_repricing_rule_period_cd")),
		SubRepricingRuleDayNum:    parseIntPtr(get("sub_repricing_rule_day_num")),
		SubCapLimitInterestRate:   parseFloatPtr(get("sub_cap_limit_interest_rate")),
		SubFloorLimitInterestRate: parseFloatPtr(get("sub_floor_limit_interest_rate")),
		SubCoeffIndex:             parseFloatPtr(get("sub_coeff_index")),
		SubRepricingFirstEventFlg: parseIntPtr(get("sub_repricing_rule_first_event_flg")),

		IndicatorLagCd:    parseString(get("indicator_lag_cd")),
		SubIndicatorLagCd: parseString(get("sub_indicator_lag_cd")),
	}
}

func logParseError(field, raw, reason string) {
	log.Printf("parse warning: field=%q value=%q: %s", field, raw, reason)
}

func parseString(s string) string {
	return strings.TrimSpace(s)
}

func parseCode(field, s string) string {
	raw := s
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		logParseError(field, raw, "empty")
		return ""
	}
	return s
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("02.01.2006", s)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга даты '%s': %v", s, err)
		return time.Time{}
	}
	return t
}

func parseTimePtr(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var t time.Time
	var err error
	if t, err = time.Parse("2006-01-02", s); err != nil {
		if t, err = time.Parse("02.01.2006", s); err != nil {
			log.Printf("ВНИМАНИЕ: ошибка парсинга даты '%s': %v", s, err)
			return nil
		}
	}
	return &t
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0.0
	}
	s = strings.Replace(s, ",", ".", -1)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга числа '%s': %v", s, err)
		return 0.0
	}
	return f
}

func parseFloatPtr(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.Replace(s, ",", ".", -1)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Printf("ВНИМАНИЕ: ошибка парсинга числа '%s': %v", s, err)
		return nil
	}
	return &f
}

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
