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
	from := ctx.StartDate

	for i, payDate := range payDates {
		yearFrac := YearFraction(from, payDate, ctx.DayConvention)
		rate := ctx.RateAt(from)
		interest := roundMoney(remaining * rate * yearFrac)

		principal := fixedPrincipal
		if i == n-1 || principal > remaining {
			principal = remaining
		}
		principal = roundMoney(principal)

		balanceBefore := remaining
		remaining = roundMoney(remaining - principal)

		if principal != 0 {
			cfs = append(cfs, emitPrincipal(deal, ctx, from, payDate, principal, remaining))
		}
		if interest != 0 {
			cfs = append(cfs, emitInterest(deal, ctx, from, payDate, interest, balanceBefore))
		}
		from = payDate
		if remaining <= 0 {
			break
		}
	}
	return cfs
}