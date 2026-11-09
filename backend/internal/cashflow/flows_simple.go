package cashflow

import (
	"time"
	"sort"
)

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
				cfs = append(cfs, emitClose(deal, ctx, ctx.AccrualStartPrincipal, payDate, principal, remaining))
			} else {
				cfs = append(cfs, emitPrincipal(deal, ctx, from, payDate, principal, remaining))
			}
		}
		from = payDate
		if remaining <= 0 { break }
	}
	return cfs
}

// generateManualFlows — ручной график тела (SIMPLE).
// Проценты идут по своему расписанию (ctx.schedules.InterestDates),
// тело — по ручному графику (principalSched).
// Оба списка сливаются в один хронологический проход:
//   - на процентной дате эмитим interest_payment;
//   - на дате тела эмитим prepayment (или close_payment на последней);
//   - если дата есть в обоих списках — обе строки в один день;
//   - проценты всегда начисляются за фактическое число дней от предыдущего события.
func generateManualFlows(deal Deal, ctx flowContext, principalSched []scheduleItem) []CashFlow {
	interestDates := ctx.schedules.InterestDates
	principalDates := principalSched

	sort.Slice(principalDates, func(a, b int) bool {
		return principalDates[a].Date.Before(principalDates[b].Date)
	})

	var cfs []CashFlow
	remaining := ctx.Remaining
	from := ctx.AccrualStart                    // для процентов
	fromPrincipal := ctx.AccrualStartPrincipal  // для тела

	i, j := 0, 0
	for i < len(interestDates) || j < len(principalDates) {
		var d time.Time
		switch {
		case i >= len(interestDates):
			d = principalDates[j].Date
		case j >= len(principalDates):
			d = interestDates[i]
		case interestDates[i].Before(principalDates[j].Date):
			d = interestDates[i]
		case principalDates[j].Date.Before(interestDates[i]):
			d = principalDates[j].Date
		default:
			d = interestDates[i]
		}

		if d.Before(ctx.StartDate) {
			if i < len(interestDates) && interestDates[i].Equal(d) {
				i++
			}
			if j < len(principalDates) && principalDates[j].Date.Equal(d) {
				j++
			}
			continue
		}

		hasInterest := i < len(interestDates) && interestDates[i].Equal(d)
		hasPrincipal := j < len(principalDates) && principalDates[j].Date.Equal(d)

		var principalAmt float64
		if hasPrincipal {
			principalAmt = principalDates[j].Value
		}

		// Проценты — от предыдущего события (разрезаются датой тела).
		yearFrac := YearFraction(from, d, ctx.DayConvention)
		rate := ctx.RateAt(from)
		interest := remaining * rate * yearFrac
		balanceBefore := remaining

		nextI, nextJ := i, j
		if hasInterest {
			nextI++
		}
		if hasPrincipal {
			nextJ++
		}
		isLast := nextI >= len(interestDates) && nextJ >= len(principalDates)

		var principal, closeAmt float64
		if hasPrincipal {
			principal = principalAmt
			if principal <= 0 || principal > remaining {
				principal = remaining
			}
			principal = roundMoney(principal)
			if isLast {
				closeAmt = roundMoney(remaining - principal)
			}
		}

		remainingAfterPrincipal := remaining - principal

		// 1. Проценты — от предыдущего события.
		if interest != 0 {
			cfs = append(cfs, emitInterest(deal, ctx, from, d, roundMoney(interest), balanceBefore))
		}

		// 2. Тело — от предыдущего погашения тела (fromPrincipal).
		if principal != 0 {
			if isLast {
				cfs = append(cfs, emitClose(deal, ctx, fromPrincipal, d, principal, remainingAfterPrincipal))
			} else {
				cfs = append(cfs, emitPrepayment(deal, ctx, fromPrincipal, d, principal, remainingAfterPrincipal))
			}
		}

		if closeAmt != 0 {
			cfs = append(cfs, emitClose(deal, ctx, fromPrincipal, d, closeAmt, 0))
		}

		remaining = remaining - principal - closeAmt
		from = d

		if hasPrincipal {
			fromPrincipal = d
		}

		if hasInterest {
			i++
		}
		if hasPrincipal {
			j++
		}

		if remaining <= 0 {
			break
		}
	}

	return cfs
}