package cashflow

import (
	"math"
    "strings"
    "time"
)

type flowContext struct {
    StartDate     time.Time
    AccrualStart          time.Time // дата начала первого процентного периода
    AccrualStartPrincipal time.Time // дата начала первого периода тела (последнее погашение тела в прошлом)
    Maturity      time.Time
    Report        time.Time
    Now           time.Time
    Remaining     float64
    DayConvention string
    isDeposit     bool
    RateAt        func(time.Time) float64
    schedules     schedules
    Indices       map[string]indexSeries
}

func (ctx flowContext) IsDeposit() bool { return ctx.isDeposit }

func (ctx flowContext) Signed(x float64) float64 {
    if ctx.isDeposit {
        return -x
    }
    return x
}

// prepareContext — валидирует сделку и приводит поля к удобному виду
// Возвращает false, если сделку считать нельзя.
func prepareContext(deal Deal, indices map[string]indexSeries) (flowContext, bool) {
	if deal.CurrentPrincipalAmt == 0 {
		return flowContext{}, false
	}
	if deal.ReportDate.IsZero() || deal.MaturityDate.IsZero() {
		return flowContext{}, false
	}
	if !deal.ReportDate.Before(deal.MaturityDate) {
		return flowContext{}, false
	}

	isDeposit := deal.CurrentPrincipalAmt < 0
	remaining := math.Abs(deal.CurrentPrincipalAmt)

	start := deal.ReportDate
	if !deal.DealDate.IsZero() && deal.DealDate.After(start) {
		start = deal.DealDate
	}
	if !start.Before(deal.MaturityDate) {
		return flowContext{}, false
	}

	conv := normalizeDayConvention(deal.DayConvention)

	accrualStart := computeAccrualStart(deal, start)
	accrualStartPrincipal := computeAccrualStartPrincipal(deal, start)

	ctx := flowContext{
		StartDate:             start,
		AccrualStart:          accrualStart,
		AccrualStartPrincipal: accrualStartPrincipal,
		Maturity:              deal.MaturityDate,
		Report:                deal.ReportDate,
		Now:                   time.Now(),
		Remaining:             remaining,
		DayConvention:         conv,
		isDeposit:             isDeposit,
		Indices:               indices,
	}

	sched, err := buildSchedules(deal, ctx)
	if err != nil {
		return flowContext{}, false
	}
	ctx.RateAt = makeRateAt(deal, sched, ctx.Indices)
	ctx.schedules = sched
	return ctx, true
}

// normalizeDayConvention приводит значение к канону и подставляет fallback
func normalizeDayConvention(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "ACT/365", "ACT/ACT", "ACT/ACT ISDA":
		return s
	default:
		// Неизвестное значение -> ACT/365
		return "ACT/365"
	}
}

// emitPrincipal — строка уменьшения тела
func emitPrincipal(deal Deal, ctx flowContext, from, to time.Time, principal, balanceAfter float64) CashFlow {
	days := daysBetween(ctx.Report, to)
	return CashFlow{
		LoadDate:         ctx.Now,
		SourceCode:       "CSV_UPLOAD",
		Discriminator:    deal.Discriminator,
		DealID:           deal.DealID,
		Currency:         deal.Currency,
		ReportDate:       ctx.Report,
		StartDate:        from,
		PaymentDate:      to,
		DaysToPayment:    days,
		TimeBucket:       GetTimeBucket(days),
		PaymentType:      TypePrincipalPayment,
		PrincipalAmount:  roundMoney(ctx.Signed(principal)),
		InterestAmount:   0,
		RemainingBalance: roundMoney(ctx.Signed(balanceAfter)),
	}
}

// emitPrepayment - досрочное погашение (ручной график SIMPLE)
func emitPrepayment(deal Deal, ctx flowContext, from, to time.Time, principal, balanceAfter float64) CashFlow {
	cf := emitPrincipal(deal, ctx, from, to, principal, balanceAfter)
	cf.PaymentType = TypePrepayment
	return cf
}

// emitClose - техническая строка закрытия (тело в maturity)
func emitClose(deal Deal, ctx flowContext, from, to time.Time, principal, balanceAfter float64) CashFlow {
	cf := emitPrincipal(deal, ctx, from, to, principal, balanceAfter)
	cf.PaymentType = TypeClosePayment
	return cf
}

// emitInterest — строка процентов
// balanceBefore — остаток ДО тела в этот же день (по ТЗ: сначала проценты, потом тело)
func emitInterest(deal Deal, ctx flowContext, from, to time.Time, interest, balanceBefore float64) CashFlow {
	days := daysBetween(ctx.Report, to)
	return CashFlow{
		LoadDate:         ctx.Now,
		SourceCode:       "CSV_UPLOAD",
		Discriminator:    deal.Discriminator,
		DealID:           deal.DealID,
		Currency:         deal.Currency,
		ReportDate:       ctx.Report,
		StartDate:        from,
		PaymentDate:      to,
		DaysToPayment:    days,
		TimeBucket:       GetTimeBucket(days),
		PaymentType:      TypeInterestPayment,
		PrincipalAmount:  0,
		InterestAmount:   ctx.Signed(interest),
		RemainingBalance: roundMoney(ctx.Signed(balanceBefore)),
	}
}

// pickPeriod — периодичность тела; fallback на проценты; fallback на "1M".
func pickPeriod(deal Deal) string {
	if p := deal.RegularPayPeriod; p != "" {
		return p
	}
	if p := deal.InterestPayPeriod; p != "" {
		return p
	}
	return "1M"
}

// principalDatesWithFallback — даты тела; если пусто — [maturity].
func principalDatesWithFallback(items []scheduleItem, maturity time.Time) []time.Time {
	if len(items) == 0 {
		return []time.Time{maturity}
	}
	out := make([]time.Time, len(items))
	for i, it := range items {
		out[i] = it.Date
	}
	return out
}

// hasManualPrincipal — были ли в расписании непустые суммы
// (признак, что это ручной график, а не правило с Value=0).
func hasManualPrincipal(items []scheduleItem) bool {
	for _, it := range items {
		if it.Value > 0 {
			return true
		}
	}
	return false
}

// computeAccrualStart — дата начала первого процентного периода.
// Для ANN/RGM — предыдущая плановая дата тела (RegularPayPeriod),
// для SIMPLE без ручного графика — предыдущая плановая дата процентов.
// Если предыдущей даты нет — fallback на дату начала сделки.
func computeAccrualStart(deal Deal, start time.Time) time.Time {
    var prev time.Time
    switch deal.Discriminator {
    case DiscANN, DiscRGM:
        prev = previousFromRule(deal.RegularPayStart, deal.MaturityDate,
            deal.ReportDate, deal.RegularPayPeriod, deal.RegularPayDayNum)
        if prev.IsZero() {
            prev = previousFromRule(deal.InterestPayStart, deal.MaturityDate,
                deal.ReportDate, deal.InterestPayPeriod, deal.InterestPayDayNum)
        }
    case DiscSIMPLE:
    // Ручной график процентов имеет приоритет: предыдущая дата <= start
    if manual := parseManualDates(deal.ManualInterestPayment); len(manual) > 0 {
        for _, d := range manual {
            if !d.After(start) && d.After(prev) {
                prev = d
            }
        }
    }
    // Иначе правило процентов
    if prev.IsZero() {
        prev = previousFromRule(deal.InterestPayStart, deal.MaturityDate,
            deal.ReportDate, deal.InterestPayPeriod, deal.InterestPayDayNum)
    }
    }
    if prev.IsZero() || (!deal.DealDate.IsZero() && prev.Before(deal.DealDate)) {
        prev = deal.DealDate
    }
    if prev.IsZero() {
        prev = start
    }
    return prev
}

// computeAccrualStartPrincipal — дата начала первого периода тела:
// последняя дата погашения тела в прошлом (из ручного графика или правила),
// либо ctx.StartDate, если тела в прошлом не было.
// Для ANN/RGM — предыдущая плановая дата тела (RegularPayPeriod).
// Для SIMPLE — по ручному графику или по RegularPayPeriod, если задан.
func computeAccrualStartPrincipal(deal Deal, start time.Time) time.Time {
    var prev time.Time
    switch deal.Discriminator {
    case DiscANN, DiscRGM:
        // Для ANN/RGM тело идёт по RegularPayPeriod; дата начала периода тела
        // = предыдущая плановая дата тела.
        prev = previousFromRule(deal.RegularPayStart, deal.MaturityDate,
            start, deal.RegularPayPeriod, deal.RegularPayDayNum)
    case DiscSIMPLE:
        // Ручной график имеет приоритет
        if manual := parseManualSchedule(deal.ManualPrincipalPayment); len(manual) > 0 {
            for _, mp := range manual {
                if mp.Date.Before(start) && mp.Date.After(prev) {
                    prev = mp.Date
                }
            }
        } else if deal.RegularPayPeriod != "" && deal.RegularPayPeriod != "ENDDATE" {
            // Правило тела, если ручного графика нет
            prev = previousFromRule(deal.RegularPayStart, deal.MaturityDate,
                start, deal.RegularPayPeriod, deal.RegularPayDayNum)
        }
        // Если ни ручного, ни правила — prev остаётся zero (тело только в maturity)
    }

    if prev.IsZero() {
        prev = start
    }
    return prev
}

// previousFromRule — последняя плановая дата <= before, вычисленная по правилу.
// Возвращает zero time.Time, если правило пустое или предыдущих дат нет.
func previousFromRule(ruleStart *time.Time, maturity, before time.Time,
    period string, dayNum *int) time.Time {
    if ruleStart == nil || period == "" || period == "ENDDATE" {
        return time.Time{}
    }
    anchor, ok := getAnchor(dayNum, ruleStart)
    if !ok || !ruleStart.Before(before) {
        return time.Time{}
    }
    cur := *ruleStart
    result := cur
    for {
        next := nextDate(cur, period, anchor)
        if !next.After(cur) {
            break
        }
        if next.After(before) || next.After(maturity) {
            break
        }
        result = next
        cur = next
    }
    return result
}

