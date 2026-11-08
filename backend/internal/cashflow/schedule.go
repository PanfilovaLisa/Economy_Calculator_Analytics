package cashflow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// buildSchedule — даты платежей от start до maturity включительно.
// Гарантирует, что последняя дата в результате — maturity (даже если
// последний шаг периода не попадает ровно на maturity). Это нужно, чтобы
// последний период (от последней регулярной даты до maturity) был оплачен.
//
// Если start >= maturity — возвращает пустой срез.
func buildSchedule(start, maturity time.Time, period string, anchorDay int) []time.Time {
	var dates []time.Time
	if !start.Before(maturity) {
		return dates
	}
	cur := start
	for {
		next := nextDate(cur, period, anchorDay)
		if !next.After(cur) {
			break
		}
		if next.After(maturity) {
			break
		}
		dates = append(dates, next)
		cur = next
		if next.Equal(maturity) {
			break
		}
	}
	// Гарантируем maturity в конце.
	if len(dates) == 0 || !dates[len(dates)-1].Equal(maturity) {
		dates = append(dates, maturity)
	}
	return dates
}

// nextDate — следующая дата после cur по period: "1M", "45D", "1Y".
// "ENDDATE" и пустое значение — возвращаем cur (сигнал «остановиться»).
// anchorDay: >0 — фиксированный день месяца; -1 — последний день месяца; 0 — без фиксации.
// Анкор применяется только для M/Y.
func nextDate(cur time.Time, period string, anchorDay int) time.Time {
	period = strings.ToUpper(strings.TrimSpace(period))
	if period == "" || period == "ENDDATE" {
		return cur
	}

	var next time.Time
	switch {
	case strings.HasSuffix(period, "D"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "D"))
		if n <= 0 {
			return cur
		}
		next = cur.AddDate(0, 0, n)
	case strings.HasSuffix(period, "M"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "M"))
		if n <= 0 {
			return cur
		}
		next = addMonths(cur, n)
	case strings.HasSuffix(period, "Y"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "Y"))
		if n <= 0 {
			return cur
		}
		next = addYears(cur, n)
	default:
		next = addMonths(cur, 1)
	}

	if anchorDay == -1 {
		return time.Date(next.Year(), next.Month()+1, 0, 0, 0, 0, 0, next.Location())
	}
	if anchorDay > 0 {
		lastDay := time.Date(next.Year(), next.Month()+1, 0, 0, 0, 0, 0, next.Location()).Day()
		d := anchorDay
		if d > lastDay {
			d = lastDay
		}
		return time.Date(next.Year(), next.Month(), d, 0, 0, 0, 0, next.Location())
	}
	return next
}

// addMonths — прибавление месяцев с клампингом дня (31-е в феврале -> 28/29).
func addMonths(t time.Time, months int) time.Time {
	year, month, day := t.Date()
	newMonth := month + time.Month(months)
	newYear := year + (int(newMonth)-1)/12
	nm := (int(newMonth)-1)%12 + 1
	lastDay := time.Date(newYear, time.Month(nm)+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(newYear, time.Month(nm), day, 0, 0, 0, 0, t.Location())
}

// addYears — прибавление лет с клампингом дня (29.02 в невисокосный -> 28.02).
func addYears(t time.Time, years int) time.Time {
	return addMonths(t, years*12)
}

// getAnchor — выбирает анкор из правила.
func getAnchor(dayNum *int, start *time.Time) (int, bool) {
	if dayNum != nil {
		if *dayNum > 0 {
			return *dayNum, true
		}
		if *dayNum == -1 {
			return -1, true
		}
	}
	if start != nil {
		return start.Day(), true
	}
	return 0, false
}

// parseDate — парсит дату в одном из поддерживаемых форматов:
// ISO "2006-01-02" или "02.01.2006". Если не подошёл ни один — ошибка.
func parseDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Parse("02.01.2006", s)
}

// ruleStart — вычисляет дату старта правила: max(ctx.StartDate, ruleStart).
func ruleStart(ctx flowContext, rs *time.Time) time.Time {
	if rs != nil && rs.After(ctx.StartDate) {
		return *rs
	}
	return ctx.StartDate
}

// manualPayment — одна запись ручного графика тела (дата + сумма).
type manualPayment struct {
	Date   time.Time
	Amount float64
}

// manualStep — одна запись ручного графика ставок (дата + ставка в долях).
type manualStep struct {
	Date time.Time
	Rate float64
}

// parseManualSchedule — парсит principal_payment: "2022-12-31,250000 2023-01-31,250000".
func parseManualSchedule(s string) []manualPayment {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var list []manualPayment
	for _, tok := range strings.Fields(s) {
		parts := strings.SplitN(tok, ",", 2)
		if len(parts) != 2 {
			continue
		}
		dt, err := parseDate(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		amt, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			continue
		}
		list = append(list, manualPayment{Date: dt, Amount: amt})
	}
	return list
}

// parseManualDates — парсит interest_payment / repricing_dttm_schedule:
// "2022-09-30 2022-10-31 2022-11-30".
func parseManualDates(s string) []time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var list []time.Time
	for _, tok := range strings.Fields(s) {
		dt, err := parseDate(tok)
		if err != nil {
			continue
		}
		list = append(list, dt)
	}
	return list
}

// parseManualRates — парсит repricing_dttm_rate_schedule: "0.05 0.06 0.07".
// Возвращает только ставки (в долях).
func parseManualRates(s string) []float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var rates []float64
	for _, tok := range strings.Fields(s) {
		rate, err := strconv.ParseFloat(tok, 64)
		if err != nil || rate < 0 {
			continue
		}
		rates = append(rates, rate)
	}
	return rates
}

// parseManualSteps — парсит repricing_dttm_rate_schedule_fixed:
// "2022-01-01,0.10 2023-01-01,0.12".
func parseManualSteps(s string) []manualStep {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []manualStep
	for _, tok := range strings.Fields(s) {
		parts := strings.SplitN(tok, ",", 2)
		if len(parts) != 2 {
			continue
		}
		dt, err := parseDate(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil || rate < 0 {
			continue
		}
		out = append(out, manualStep{Date: dt, Rate: rate})
	}
	return out
}

// buildStepRate соединяет даты и ставки в единое расписание (сортирует по дате).
func buildStepRate(dates []time.Time, rates []float64) ([]manualStep, error) {
	if len(dates) != len(rates) {
		return nil, fmt.Errorf(
			"buildStepRate: len(dates)=%d != len(rates)=%d",
			len(dates), len(rates),
		)
	}
	if len(dates) == 0 {
		return nil, nil
	}
	out := make([]manualStep, len(dates))
	for i := range dates {
		out[i] = manualStep{Date: dates[i], Rate: rates[i]}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}