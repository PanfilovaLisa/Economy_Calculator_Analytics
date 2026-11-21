package cashflow

import "time"

// распределяет интервал в днях по регуляторным корзинам IRRBB
func GetTimeBucket(days int) string {
	switch {
	case days <= 1:
		return "1d"
	case days <= 30:
		return "1m"
	case days <= 90:
		return "3m"
	case days <= 180:
		return "6m"
	case days <= 270:
		return "9m"
	case days <= 365:
		return "1y"
	case days <= 547:
		return "1.5y"
	case days <= 730:
		return "2y"
	case days <= 1095:
		return "3y"
	case days <= 1460:
		return "4y"
	case days <= 1825:
		return "5y"
	case days <= 2190:
		return "6y"
	case days <= 2555:
		return "7y"
	case days <= 2920:
		return "8y"
	case days <= 3285:
		return "9y"
	case days <= 3650:
		return "10y"
	case days <= 5475:
		return "15y"
	case days <= 7300:
		return "20y"
	default:
		return "30y"
	}
}

// строка уменьшения тела
func emitPrincipal(
	deal Deal, ctx flowContext, from, to time.Time, principal, balanceAfter float64,
	) CashFlow {
	days := daysBetween(deal.ReportDate, to)
	return CashFlow{
		DealID: deal.DealID,
		Currency: deal.Currency,
		Discriminator: deal.Discriminator,
		RateType: deal.RateType,
		ReportDate: deal.ReportDate,
		PaymentType: TypePrincipalPayment,
		DaysToPayment: days,
		TimeBucket: GetTimeBucket(days),
	
		StartDate: from,
		PaymentDate: to,
		PrincipalAmount: ctx.Signed(principal),
		InterestAmount: 0,
		RemainingBalance:  ctx.Signed(balanceAfter),

		PrevRate: 0,
		CurrentRate: 0,
	}
}

// досрочное погашение (ручной график SIMPLE)
func emitPrepayment(
	deal Deal, ctx flowContext, from, to time.Time, principal, balanceAfter float64,
	) CashFlow {
	cf := emitPrincipal(deal, ctx, from, to, principal, balanceAfter)
	cf.PaymentType = TypePrepayment
	return cf
}

// техническая строка закрытия (тело в maturity)
func emitClose(
	deal Deal, ctx flowContext, from, to time.Time, principal, balanceAfter float64,
	) CashFlow {
	cf := emitPrincipal(deal, ctx, from, to, principal, balanceAfter)
	cf.PaymentType = TypeClosePayment
	return cf
}

// строка процентов
func emitInterest(
	deal Deal, ctx flowContext, from, to time.Time, interest, balanceAfter float64,
	) CashFlow {
	days := daysBetween(deal.ReportDate, to)
	return CashFlow{
		DealID: deal.DealID,
		Currency: deal.Currency,
		Discriminator: deal.Discriminator,
		RateType: deal.RateType,
		ReportDate: deal.ReportDate,
		PaymentType: TypeInterestPayment,
		DaysToPayment: days,
		TimeBucket: GetTimeBucket(days),
	
		StartDate: from,
		PaymentDate: to,
		PrincipalAmount: 0,
		InterestAmount:  ctx.Signed(interest),
		RemainingBalance:  ctx.Signed(balanceAfter),

		PrevRate: 0,
		CurrentRate: 0,
	}
}

// строка repricing
func emitRepricing(
	deal Deal, ctx flowContext, from, to time.Time, balance, prevRate, curRate float64,
	) CashFlow {
	days := daysBetween(deal.ReportDate, to)
	return CashFlow{
		DealID: deal.DealID,
		Currency: deal.Currency,
		Discriminator: deal.Discriminator,
		RateType: deal.RateType,
		ReportDate: deal.ReportDate,
		PaymentType: TypeRepricing,
		DaysToPayment: days,
		TimeBucket: GetTimeBucket(days),
	
		StartDate: from,
		PaymentDate: to,
		PrincipalAmount: 0,
		InterestAmount: 0,
		RemainingBalance:  ctx.Signed(balance),

		PrevRate: prevRate,
		CurrentRate: curRate,
	}
}