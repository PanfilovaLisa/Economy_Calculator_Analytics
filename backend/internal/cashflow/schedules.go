package cashflow

import (
	"fmt"
	"sort"
	"time"
)

// scheduleItem — пара (дата, значение)
// Семантика Value зависит от поля schedules:
//   - Principal:   сумма платежа тела (или 0 — «остаток, посчитать формулой»)
//   - FixedSteps:  годовая ставка в долях (0.05 = 5%)
type scheduleItem struct {
	Date  time.Time
	Value float64
}

type schedules struct {
	Principal      []scheduleItem
	InterestDates  []time.Time
	RepricingDates []time.Time
	RepricingRates []float64
	FixedSteps     []scheduleItem
}

// buildSchedules — собирает все расписания
// Даты процентов строятся один раз и переиспользуются в buildPrincipal (fallback)
func buildSchedules(deal Deal, ctx flowContext) (schedules, error) {
	interestDates := buildInterestDates(deal, ctx)

	principal, err := buildPrincipal(deal, ctx, interestDates)
	if err != nil {
		return schedules{}, fmt.Errorf("buildSchedules: principal: %w", err)
	}

	repricingDates, repricingRates, err := buildRepricing(deal, ctx)
	if err != nil {
		return schedules{}, fmt.Errorf("buildSchedules: repricing: %w", err)
	}

	return schedules{
		Principal:      principal,
		InterestDates:  interestDates,
		RepricingDates: repricingDates,
		RepricingRates: repricingRates,
		FixedSteps:     buildFixedSteps(deal, ctx),
	}, nil
}

// buildPrincipal — расписание тела: даты + суммы.
// Приоритет:
//  1. principal_payment (ручной график).
//  2. regular_payment_rule_period_cd:
//     - ENDDATE -> [maturity] с Value = 0;
//     - другое -> правило от regular_payment_rule_start_dttm, Value = 0.
//  3. fallback — даты процентов, Value = 0.
//
// Везде гарантируется maturity в конце (иначе последний период не оплатится).
// Возвращает error, если ручной график задан, но после фильтра
// (даты >= StartDate) не осталось ни одной записи — почти наверняка ошибка конфигурации.
func buildPrincipal(deal Deal, ctx flowContext, interestDates []time.Time) ([]scheduleItem, error) {
	// 1. Ручной график.
	if manual := parseManualSchedule(deal.ManualPrincipalPayment); len(manual) > 0 {
		out := make([]scheduleItem, 0, len(manual)+1)
		for _, mp := range manual {
			if !mp.Date.Before(ctx.StartDate) {
				out = append(out, scheduleItem{Date: mp.Date, Value: mp.Amount})
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf(
				"buildPrincipal: manual principal_payment задан (%d записей), "+
					"но все даты раньше StartDate=%s",
				len(manual), ctx.StartDate.Format("2006-01-02"),
			)
		}
		// Гарантируем maturity в конце.
		if !out[len(out)-1].Date.Equal(ctx.Maturity) {
			out = append(out, scheduleItem{Date: ctx.Maturity, Value: 0})
		}
		return out, nil
	}

	// 2. Правило.
	if period := deal.RegularPayPeriod; period != "" {
		if period == "ENDDATE" {
			return []scheduleItem{{Date: ctx.Maturity, Value: 0}}, nil
		}
		if anchor, ok := getAnchor(deal.RegularPayDayNum, deal.RegularPayStart); ok {
			start := ruleStart(ctx, deal.RegularPayStart)
			dates := buildSchedule(start, ctx.Maturity, period, anchor)
			out := make([]scheduleItem, len(dates))
			for i, d := range dates {
				out[i] = scheduleItem{Date: d, Value: 0}
			}
			return out, nil
		}
	}

	// 3. Fallback — даты процентов (уже с maturity).
	out := make([]scheduleItem, 0, len(interestDates)+1)
	for _, d := range interestDates {
		out = append(out, scheduleItem{Date: d, Value: 0})
	}
	if len(out) == 0 || !out[len(out)-1].Date.Equal(ctx.Maturity) {
		out = append(out, scheduleItem{Date: ctx.Maturity, Value: 0})
	}
	return out, nil
}

// buildInterestDates — даты эмиссии процентов.
// Приоритет:
//  1. interest_payment (ручной график дат).
//  2. interest_payment_rule_period_cd:
//     - ENDDATE -> [maturity];
//     - другое -> правило от interest_payment_rule_start_dttm.
//  3. fallback — [maturity].
func buildInterestDates(deal Deal, ctx flowContext) []time.Time {
	// 1. Ручной график.
	if manual := parseManualDates(deal.ManualInterestPayment); len(manual) > 0 {
		out := make([]time.Time, 0, len(manual)+1)
		for _, d := range manual {
			if !d.Before(ctx.StartDate) {
				out = append(out, d)
			}
		}
		if len(out) == 0 {
			return []time.Time{ctx.Maturity}
		}
		if !out[len(out)-1].Equal(ctx.Maturity) {
			out = append(out, ctx.Maturity)
		}
		return out
	}

	// 2. Правило.
	if period := deal.InterestPayPeriod; period != "" {
		if period == "ENDDATE" {
			return []time.Time{ctx.Maturity}
		}
		if anchor, ok := getAnchor(deal.InterestPayDayNum, deal.InterestPayStart); ok {
			start := ruleStart(ctx, deal.InterestPayStart)
			return buildSchedule(start, ctx.Maturity, period, anchor)
		}
	}

	// 3. Fallback.
	return []time.Time{ctx.Maturity}
}

// buildRepricing — даты и ставки переоценки. Только для FLOAT.
// Возвращает (nil, nil, nil), если RateType != FLOAT или переоценки нет.
// Если ставки заданы — их длина обязана совпасть с длиной дат.
func buildRepricing(deal Deal, ctx flowContext) ([]time.Time, []float64, error) {
	if deal.RateType != "FLOAT" {
		return nil, nil, nil
	}

	dates := buildRepricingDates(deal, ctx)
	rates := parseManualRates(deal.RepricingRateSchedule)

	if len(rates) > 0 && len(dates) > 0 && len(rates) != len(dates) {
		return nil, nil, fmt.Errorf(
			"buildRepricing: len(repricingDates)=%d != len(repricingRates)=%d",
			len(dates), len(rates),
		)
	}
	if len(rates) > 0 && len(dates) == 0 {
		return nil, nil, fmt.Errorf(
			"buildRepricing: repricing_dttm_rate_schedule задан (%d ставок), "+
				"но repricing_dttm_schedule пуст",
			len(rates),
		)
	}
	return dates, rates, nil
}

// buildRepricingDates — даты переоценки. Проверка RateType — в buildRepricing;
// эта функция вызывается только оттуда, поэтому проверка здесь не дублируется.
func buildRepricingDates(deal Deal, ctx flowContext) []time.Time {
	// 1. Явный список дат.
	if dates := parseManualDates(deal.RepricingDttmSchedule); len(dates) > 0 {
		out := make([]time.Time, 0, len(dates)+1)
		for _, d := range dates {
			if !d.Before(ctx.StartDate) {
				out = append(out, d)
			}
		}
		if len(out) > 0 && !out[len(out)-1].Equal(ctx.Maturity) {
			out = append(out, ctx.Maturity)
		}
		return out
	}

	// 2. Правило.
	if period := deal.RepricingRulePeriod; period != "" {
		if anchor, ok := getAnchor(deal.RepricingRuleDayNum, deal.RepricingRuleStart); ok {
			start := ruleStart(ctx, deal.RepricingRuleStart)
			return buildSchedule(start, ctx.Maturity, period, anchor)
		}
	}

	// 3. Переоценки нет.
	return nil
}

// buildFixedSteps — ступени FIXED из repricing_dttm_rate_schedule_fixed.
// Приоритетнее InterestRate. Возвращает nil, если не задано или RateType != FIXED.
//
// Если первая ступень позже StartDate — добавляется синтетическая ступень
// (StartDate, InterestRate), чтобы ставка для периода [StartDate, firstStep)
// была определена (а не «будущая ступень»).
func buildFixedSteps(deal Deal, ctx flowContext) []scheduleItem {
	if deal.RateType != "FIXED" {
		return nil
	}
	manual := parseManualSteps(deal.RepricingRateFixed)
	if len(manual) == 0 {
		return nil
	}
	sort.Slice(manual, func(i, j int) bool { return manual[i].Date.Before(manual[j].Date) })

	out := make([]scheduleItem, 0, len(manual)+1)
	if manual[0].Date.After(ctx.StartDate) {
		// До первой ступени действует базовая ставка.
		out = append(out, scheduleItem{Date: ctx.StartDate, Value: deal.InterestRate})
	}
	for _, ms := range manual {
		out = append(out, scheduleItem{Date: ms.Date, Value: ms.Rate})
	}
	return out
}