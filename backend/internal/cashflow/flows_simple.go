package cashflow

import (
	"time"
)

// simple: ручной график (principal_payment) или всё в maturity
func generateSimpleFlows(deal Deal, ctx flowContext) ([]CashFlow, error) {
	sched := ctx.schedules

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
	initialRate =  roundRate(initialRate)

	//начальная ставка для sub
	var SubInitialRate float64 = 0
	if ctx.hasSub {
		SubInitialRate = clampRate(*deal.SubInterestRate, deal.SubCapLimitInterestRate, deal.SubFloorLimitInterestRate)
		if deal.SubRateTypeCd == RateFloat{
			if len(sched.SubRepricing) > 0 && sched.SubRepricing[0].Value != -1 {
				SubInitialRate = sched.SubRepricing[0].Value
			}
		}
		SubInitialRate =  roundRate(SubInitialRate)
	}

	remaining := ctx.Remaining

	prevRate := 0.0
	currentRate := initialRate
	fromRepricing := time.Time{}
	fromInterest := time.Time{} // начало текущего процентного интервала, может меняться во время репрайсинга и тела
	accruedInterestForInterest := 0.0
	fromPrincipal := time.Time{} // начало текущего интервала тела

	SubPrevRate := 0.0
	SubCurrentRate := SubInitialRate
	fromSubRepricing := time.Time{}
	fromSubInterest := time.Time{} //может менятся от sub репрайсинга и тела
	accruedSubInterestForInterest := 0.0 

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
			currentRate =  roundRate(ev.Value)

			if !fromRepricing.IsZero() && !ev.Date.Before(ctx.StartDate) {
				cfs = append(cfs, emitRepricing(deal, ctx, fromRepricing, ev.Date, remaining, prevRate, currentRate))
			}

			fromRepricing = ev.Date

		case EvInterest:
			if fromInterest.IsZero() && it.j == 1 {
				fromInterest = ev.Date
				accruedInterestForInterest = 0
				break
			}

			yearFrac := YearFraction(fromInterest, ev.Date, ctx.DayConvention)
			rate := currentRate
			accruedInterestForInterest += remaining * rate * yearFrac
			
			cfs = append(cfs, emitInterest(deal, ctx, sched.Interest[it.j - 2].Date, ev.Date, roundMoney(accruedInterestForInterest), remaining))
			
			fromInterest = ev.Date
			accruedInterestForInterest = 0

		case SubEvRepricing:
			if ev.Value == -1 {
				fromSubRepricing = ev.Date
				break 
			}
			if !fromSubInterest.IsZero() {
				yearFrac := YearFraction(fromSubInterest, ev.Date, ctx.DayConvention)
				rate := SubCurrentRate
				accruedSubInterestForInterest += remaining * rate * yearFrac
				fromSubInterest = ev.Date
			}

			SubPrevRate = SubCurrentRate
			SubCurrentRate = roundRate(ev.Value)

			if !fromSubRepricing.IsZero() && !ev.Date.Before(ctx.StartDate) {
				cfs = append(cfs, emitSubRepricing(deal, ctx, fromSubRepricing, ev.Date, remaining, SubPrevRate, SubCurrentRate))
			}

			fromSubRepricing = ev.Date

		case SubEvInterest:
			if fromSubInterest.IsZero() && it.m == 1 {
				fromSubInterest = ev.Date
				accruedSubInterestForInterest = 0
				break
			}

			yearFrac := YearFraction(fromSubInterest, ev.Date, ctx.DayConvention)
			rate := SubCurrentRate
			accruedSubInterestForInterest += remaining * rate * yearFrac
			
			cfs = append(cfs, emitSubInterest(deal, ctx, sched.SubInterest[it.m - 2].Date, ev.Date, roundMoney(accruedSubInterestForInterest), remaining))
			fromSubInterest = ev.Date
			accruedSubInterestForInterest = 0

		case EvPrincipal:
			if fromPrincipal.IsZero() && it.i == 1 {
				fromPrincipal = ev.Date
				break
			}

			if !fromInterest.IsZero() && !fromInterest.Equal(ev.Date) {
				//копим проценты для процентов по старому телу
				yearFrac := YearFraction(fromInterest, ev.Date, ctx.DayConvention)
				rate := currentRate
				accruedInterestForInterest += remaining * rate * yearFrac
				fromInterest = ev.Date
			}
			if !fromSubInterest.IsZero() && !fromSubInterest.Equal(ev.Date) {
				yearFrac := YearFraction(fromSubInterest, ev.Date, ctx.DayConvention)
				rate := SubCurrentRate
				accruedSubInterestForInterest += remaining * rate * yearFrac
				fromSubInterest = ev.Date
			}

			var principal float64
			if deal.MaturityDate.Equal(ev.Date) {
				principal = remaining
			} else  {
				principal = ev.Value
				if principal > remaining {
					principal = remaining
				}
			}

			remaining -= principal

			cfs = append(cfs, emitPrepayment(deal, ctx, sched.Principal[it.i - 2].Date, ev.Date, roundMoney(principal), roundMoney(remaining)))

			fromPrincipal = ev.Date
		}
	}

	principal := roundMoney(remaining)
	remaining = 0

	cfs = append(cfs, emitClose(deal, ctx, fromPrincipal, deal.MaturityDate, principal, remaining))

	return cfs, nil
}
