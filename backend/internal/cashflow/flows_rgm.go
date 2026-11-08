package cashflow

import ()

// generateDifferentiatedFlows — дифференцированный: тело равными долями,
// проценты на остаток, последний платёж добивает остаток.
func generateDifferentiatedFlows(deal Deal, ctx flowContext) []CashFlow {
	sched := ctx.schedules
	payDates := principalDatesWithFallback(sched.Principal, ctx.Maturity)
	if len(payDates) == 0 {
		return nil
	}

	n := len(payDates)
	fixedPrincipal := roundMoney(ctx.Remaining / float64(n))

	var cfs []CashFlow
	remaining := ctx.Remaining
	from := ctx.AccrualStart

	for i, payDate := range payDates {
		yearFrac := YearFraction(from, payDate, ctx.DayConvention)
		rate := ctx.RateAt(from)
		interest := remaining * rate * yearFrac

		balanceBefore := remaining
		isLast := i == n-1

		var principal, closeAmt float64

		if isLast {
			// Последний период: регулярная часть — fixedPrincipal,
			// остаток после неё — close_payment.
			principal = fixedPrincipal
			if principal > remaining { principal = remaining }
			principal = roundMoney(principal)
			closeAmt = roundMoney(remaining - principal)
		} else {
			principal = fixedPrincipal
			if principal > remaining { principal = remaining }
			principal = roundMoney(principal)
		}

		remainingAfterPrincipal := remaining - principal

		if interest != 0 {
			cfs = append(cfs, emitInterest(deal, ctx, from, payDate, roundMoney(interest), balanceBefore))
		}
		if principal != 0 {
			cfs = append(cfs, emitPrincipal(deal, ctx, from, payDate, principal, remainingAfterPrincipal))
		}
		if closeAmt != 0 {
			cfs = append(cfs, emitClose(deal, ctx, from, payDate, closeAmt, 0))
		}

		remaining = remaining - principal - closeAmt
		from = payDate
		if remaining <= 0 { break }
	}
	return cfs
}