package cashflow

import (
	"math"
    "strings"
    "time"
)

type flowContext struct {
	StartDate     time.Time //дата, когда начали платить
	Maturity      time.Time //дата, когда закончим платить (реализация поведенческой модели не MVP)
	Report        time.Time //дата отчета
	Now           time.Time
	Remaining     float64 // положительная оставшаяся сумма
	DayConvention string

	isDeposit     bool //приватное

	// RateAt заполняется позже в GenerateCashFlows после buildSchedules
	// Для MVP — замыкание; см. makeRateAt
	RateAt func(time.Time) float64
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
func prepareContext(deal Deal) (flowContext, bool) {
	//проверки остаток не ноль, отчёт раньше maturity
	if deal.CurrentPrincipalAmt == 0 {
		return flowContext{}, false
	}
	if deal.ReportDate.IsZero() || deal.MaturityDate.IsZero() {
		return flowContext{}, false
	}
	if !deal.ReportDate.Before(deal.MaturityDate) {
		return flowContext{}, false
	}

	// Знак и депозит.
	isDeposit := deal.CurrentPrincipalAmt < 0

	// Остаток по модулю
	remaining := math.Abs(deal.CurrentPrincipalAmt)

	// Старт: позже из report_dttm и deal_dttm.
	start := deal.ReportDate
	if !deal.DealDate.IsZero() && deal.DealDate.After(start) {
		start = deal.DealDate
	}

	// Старт должен быть строго раньше maturity.
	if !start.Before(deal.MaturityDate) {
		return flowContext{}, false
	}

	// DayConvention с fallback
	conv := normalizeDayConvention(deal.DayConvention)

	return flowContext{
        StartDate:     start,
        Maturity:      deal.MaturityDate,
        Report:        deal.ReportDate,
        Now:           time.Now(),
        Remaining:     remaining,
        DayConvention: conv,
        isDeposit:     isDeposit,
    }, true
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
		InterestAmount:   roundMoney(ctx.Signed(interest)),
		RemainingBalance: roundMoney(ctx.Signed(balanceBefore)),
	}
}