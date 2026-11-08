package cashflow

import "time"

// generateSimpleFlows — простой:
//   - если principal_payment задан — ручной график (prepayment/close);
//   - иначе тело в maturity, промежуточные даты — только проценты.
func generateSimpleFlows(deal Deal, ctx flowContext) []CashFlow {
	sched := ctx.schedules

	if len(sched.Principal) > 0 && hasManualPrincipal(sched.Principal) {
		return generateManualFlows(deal, ctx, sched.Principal)
	}

	// Тело в конце: все даты процентов, тело только на последней.
	var cfs []CashFlow
	remaining := ctx.Remaining
	from := ctx.AccrualStart
	dates := sched.InterestDates
	if len(dates) == 0 {
		dates = []time.Time{ctx.Maturity}
	}

	for i, payDate := range dates {
		isLast := i == len(dates)-1
		yearFrac := YearFraction(from, payDate, ctx.DayConvention)
		rate := ctx.RateAt(from)
		interest := remaining * rate * yearFrac

		var principal float64
		if isLast {
			principal = remaining
		}
		balanceBefore := remaining
		remaining = remaining - principal

		if interest != 0 {
			cfs = append(cfs, emitInterest(deal, ctx, from, payDate, roundMoney(interest), balanceBefore))
		}
		if principal != 0 {
			if isLast {
				cfs = append(cfs, emitClose(deal, ctx, from, payDate, principal, remaining))
			} else {
				cfs = append(cfs, emitPrincipal(deal, ctx, from, payDate, principal, remaining))
			}
		}
		from = payDate
		if remaining <= 0 { break }
	}
	return cfs
}

// generateManualFlows — ручной график тела.
// Первая запись (или любая ненулевая до последней) — prepayment,
// последняя (дата == maturity) — close_payment.
func generateManualFlows(deal Deal, ctx flowContext, principalSched []scheduleItem) []CashFlow {
	var cfs []CashFlow
	remaining := ctx.Remaining
	from := ctx.AccrualStart

	for i, item := range principalSched {
		payDate := item.Date
		if !payDate.After(from) {
			continue
		}
		isLast := i == len(principalSched)-1

		yearFrac := YearFraction(from, payDate, ctx.DayConvention)
		rate := ctx.RateAt(from)
		interest := roundMoney(remaining * rate * yearFrac)

		principal := item.Value
		if principal <= 0 || isLast || principal > remaining {
			principal = remaining
		}
		principal = roundMoney(principal)

		balanceBefore := remaining
		remaining = roundMoney(remaining - principal)

		if interest != 0 {
			cfs = append(cfs, emitInterest(deal, ctx, from, payDate, interest, balanceBefore))
		}
		if principal != 0 {
			if isLast {
				cfs = append(cfs, emitClose(deal, ctx, from, payDate, principal, remaining))
			} else {
				cfs = append(cfs, emitPrepayment(deal, ctx, from, payDate, principal, remaining))
			}
		}

		from = payDate
		if remaining <= 0 {
			break
		}
	}
	return cfs
}