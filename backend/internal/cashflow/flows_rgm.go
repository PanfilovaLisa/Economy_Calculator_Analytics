package cashflow

import (
	"time"
)

// аннуитет: фиксированный суммарный платёж,
// тело = PMT − interest, последний платёж добивает остаток
// close payment добавляется здесь
func generateRGMFlows(deal Deal, ctx flowContext) ([]CashFlow, error) {
	sched := ctx.schedules

	n := len(sched.Principal) - 1 //-1 без shed[0], тк это прошлая дата

	//начальная ставка
	initialRate := fullRate(deal.InterestRate, deal.BaseIndicatorRate, deal.RateType)
	initialRate = clampRate(initialRate, deal.CapLimitInterestRate, deal.FloorLimitInterestRate)
	switch deal.RateType {
	case RateFloat:
		if len(sched.Repricing) > 0 && sched.Repricing[0].Value != -1 {
			initialRate = sched.Repricing[0].Value
		}
	case RateFixed:
		if len(sched.FixedSteps) > 0 && sched.FixedSteps[0].Value != -1{
			initialRate = sched.FixedSteps[0].Value
		}
	}

	remaining := ctx.Remaining
	fixedPrincipal := roundMoney(remaining / float64(n))

	prevRate := 0.0
	currentRate := initialRate
	fromRepricing := time.Time{}
	fromInterest := time.Time{} // начало текущего процентного интервала, может меняться во время репрайсинга и тела
	accruedInterestForInterest := 0.0
	fromPrincipal := time.Time{} // начало текущего интервала тела

	var cfs []CashFlow

	it := newEventIter(sched, deal.RateType)
	for ev, ok := it.Next(); ok; ev, ok = it.Next() {
		switch ev.Type {
		case EvRepricing:
			if ev.Value == -1 {
				fromRepricing = ev.Date
				break 
			}
			if !fromInterest.IsZero() {
				//копим проценты для процентов со старой ставкой
				yearFrac := YearFraction(fromInterest, ev.Date, ctx.DayConvention)
				rate := currentRate
				accruedInterestForInterest += remaining * rate * yearFrac
				fromInterest = ev.Date
			}

			prevRate = currentRate

			// Recast: пересчитать PMT от остатка и оставшихся платежей по новой ставке
			// Оставшиеся платежи - все даты тела после текущей даты события
			currentRate = ev.Value

			if !fromRepricing.IsZero() && !ev.Date.Before(ctx.StartDate) {
				cfs = append(cfs, emitRepricing(deal, ctx, fromRepricing, ev.Date, remaining, prevRate, currentRate))
			}

			fromRepricing = ev.Date

		case EvInterest:
			if fromInterest.IsZero() {
				fromInterest = ev.Date
				accruedInterestForInterest = 0
				break
			}

			yearFrac := YearFraction(fromInterest, ev.Date, ctx.DayConvention)
			if yearFrac != 0 {
				rate := currentRate
				accruedInterestForInterest += remaining * rate * yearFrac
				
				cfs = append(cfs, emitInterest(deal, ctx, fromInterest, ev.Date, roundMoney(accruedInterestForInterest), remaining))
			}
			fromInterest = ev.Date
			accruedInterestForInterest = 0

		case EvPrincipal:
			if fromPrincipal.IsZero() {
				fromPrincipal = ev.Date
				break
			}

			// Тело = remaining / n = fixedPrincipal
			principal := fixedPrincipal
			if remaining - principal < 0 { principal = roundMoney(remaining) }

			if !fromInterest.IsZero() {
				//копим проценты для процентов по старому телу
				yearFrac := YearFraction(fromInterest, ev.Date, ctx.DayConvention)
				rate := currentRate
				accruedInterestForInterest += remaining * rate * yearFrac
				fromInterest = ev.Date
			}

			remaining -= principal

			cfs = append(cfs, emitPrincipal(deal, ctx, fromPrincipal, ev.Date, principal, roundMoney(remaining)))
			

			fromPrincipal = ev.Date
		}
	}

	principal := roundMoney(remaining)
	remaining = 0

	cfs = append(cfs, emitClose(deal, ctx, fromPrincipal, deal.MaturityDate, principal, remaining))

	return cfs, nil
}
