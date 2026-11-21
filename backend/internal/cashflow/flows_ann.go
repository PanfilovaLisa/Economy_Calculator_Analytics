package cashflow

import (
	"fmt"
	"math"
	"time"
)

// аннуитет: фиксированный суммарный платёж,
// тело = PMT − interest, последний платёж добивает остаток
// close payment добавляется здесь
func generateAnnFlows(deal Deal, ctx flowContext) ([]CashFlow, error) {
	sched := ctx.schedules

	n := len(sched.Principal) - 1

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

	// PMT от текущего остатка, числа платежей и ставки
	remaining := ctx.Remaining
	pmt, err := calcAnnuityPMT(remaining, initialRate, n, sched.PrincipalPeriod)
	if err != nil {
		return nil, fmt.Errorf("generateAnnFlows: %w", err)
	}
	pmt = roundMoney(pmt)

	prevRate := 0.0
	currentRate := initialRate
	fromRepricing := time.Time{}
	fromInterest := time.Time{} // начало текущего процентного интервала, может меняться во время репрайсинга и тела
	accruedInterestForInterest := 0.0
	fromPrincipal := time.Time{} // начало текущего интервала тела, может меняться во время репрайсинга
	accruedInterestOnPrincipal := 0.0 // проценты для формулы тела (с прошлой даты тела)

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
			if !fromPrincipal.IsZero() {
				//копим проценты для тела со старой ставкой
				yearFrac := YearFraction(fromPrincipal, ev.Date, ctx.DayConvention)
				rate := currentRate
				accruedInterestOnPrincipal += remaining * rate * yearFrac
				fromPrincipal = ev.Date
			}

			prevRate = currentRate

			// Recast: пересчитать PMT от остатка и оставшихся платежей по новой ставке
			// Оставшиеся платежи - все даты тела после текущей даты события
			currentRate = roundRate(ev.Value)
			remainingPayments := countPrincipalAfter(sched.Principal, ev.Date)
			if remainingPayments > 0 {
				pmt, err = calcAnnuityPMT(remaining, currentRate, remainingPayments, sched.PrincipalPeriod)
				if err != nil { return nil, fmt.Errorf("generateAnnFlows: %w", err) }
				pmt = roundMoney(pmt)
			}

			if !fromRepricing.IsZero() && !ev.Date.Before(ctx.StartDate) {
				cfs = append(cfs, emitRepricing(deal, ctx, fromRepricing, ev.Date, remaining, prevRate, currentRate))
			}

			fromRepricing = ev.Date

		case EvInterest:
			// Проценты за интервал от prev до ev.Date с подынтервалами ставок
			// проценты зависят от смены тела и репрайсинга
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
			// Накопленные проценты за интервал тела (от prevPrincipal до ev.Date)
			// тело зависит от накопленный процентов за период тела, а они зависят от репрайсинга
			if fromPrincipal.IsZero() && it.i == 1 {
				fromPrincipal = ev.Date
				accruedInterestOnPrincipal = 0
				break
			}

			yearFrac := YearFraction(fromPrincipal, ev.Date, ctx.DayConvention)
			rate := currentRate
			accruedInterestOnPrincipal += remaining * rate * yearFrac

			// Тело = PMT − проценты за период тела
			principal := roundMoney(pmt - roundMoney(accruedInterestOnPrincipal))
			if principal < 0 {
				principal = 0
			}
			if principal > remaining {
				principal = roundMoney(remaining)
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

			remaining -= principal

			cfs = append(cfs, emitPrincipal(deal, ctx, sched.Principal[it.i - 2].Date, ev.Date, principal, roundMoney(remaining)))
			

			fromPrincipal = ev.Date
			accruedInterestOnPrincipal = 0

			if remaining <= 0 {
				return cfs, nil
			}
		}
	}

	principal := roundMoney(remaining)
	remaining = 0

	cfs = append(cfs, emitClose(deal, ctx, fromPrincipal, deal.MaturityDate, principal, remaining))

	return cfs, nil
}

// фиксированный платёж аннуитета
// period нужен, чтобы ставка за период считалась корректно:
// для "1M" r = annualRate/12, для "3M" r = annualRate/4, для "1Y" r = annualRate
func calcAnnuityPMT(principal, annualRate float64, n int, period string) (float64, error) {
	if principal <= 0 {
		return 0, fmt.Errorf("calcAnnuityPMT: principal must be > 0, got %v", principal)
	}
	if annualRate < 0 {
		return 0, fmt.Errorf("calcAnnuityPMT: negative annual rate %v", annualRate)
	}
	if n <= 0 {
		return 0, fmt.Errorf("calcAnnuityPMT: n must be > 0, got %d", n)
	}

	ppy, err := periodsPerYear(period)
	if err != nil {
		return 0, fmt.Errorf("calcAnnuityPMT: %w", err)
	}

	if annualRate == 0 {
		return principal / float64(n), nil
	}

	r := annualRate / ppy

	denom := 1 - math.Pow(1+r, float64(-n))
	if denom == 0 {
		return 0, fmt.Errorf("calcAnnuityPMT: zero denominator (r=%v, n=%d)", r, n)
	}

	pmt := principal * r / denom
	if math.IsInf(pmt, 0) || math.IsNaN(pmt) {
		return 0, fmt.Errorf("calcAnnuityPMT: non-finite PMT %v (principal=%v, r=%v, n=%d)", pmt, principal, r, n)
	}
	return pmt, nil
}