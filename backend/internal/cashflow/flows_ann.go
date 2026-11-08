package cashflow

import ()

// generateAnnuityFlows — аннуитет: фиксированный суммарный платёж,
// тело = PMT − interest, последний платёж добивает остаток.
func generateAnnuityFlows(deal Deal, ctx flowContext) []CashFlow {
	sched := ctx.schedules
	payDates := principalDatesWithFallback(sched.Principal, ctx.Maturity)
	if len(payDates) == 0 {
		return nil
	}

	n := len(payDates)
	period := pickPeriod(deal)
	pmt := calcAnnuityPMT(ctx.Remaining, effectiveRate(deal), n, period)

	var cfs []CashFlow
	remaining := ctx.Remaining
	from := ctx.AccrualStart

	for i, payDate := range payDates {
		yearFrac := YearFraction(from, payDate, ctx.DayConvention)
		rate := ctx.RateAt(from)
		interest := remaining * rate * yearFrac

		balanceBefore := remaining
		var principal, closeAmt float64

		if i != n-1 {
			principal = roundMoney(pmt - interest)
			if principal < 0 { principal = 0 }
			if principal > remaining { principal = remaining }
		} else {
			// Последний период: сначала аннуитетная часть, потом close
			principal = roundMoney(pmt - interest)
			if principal < 0 { principal = 0 }
			if principal > remaining { principal = remaining }
			closeAmt = roundMoney(remaining - principal)
		}

		remainingAfterPrincipal := roundMoney(remaining - principal)

		if interest != 0 {
			cfs = append(cfs, emitInterest(deal, ctx, from, payDate, roundMoney(interest), balanceBefore))
		}
		if principal != 0 {
			cfs = append(cfs, emitPrincipal(deal, ctx, from, payDate, principal, remainingAfterPrincipal))
		}
		if closeAmt != 0 {
			cfs = append(cfs, emitClose(deal, ctx, from, payDate, closeAmt, 0))
		}

		remaining = roundMoney(remaining - principal - closeAmt)
		from = payDate
		if remaining <= 0 { break }
	}
	return cfs
}