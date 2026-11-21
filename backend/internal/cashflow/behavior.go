package cashflow

import (
	"math"
	"strings"
	"time"
)

func getBehaviorCPR(modelCode string) float64 {
	switch strings.ToUpper(strings.TrimSpace(modelCode)) {
	case "PREPAYMENT_CONSUMER":
		return 0.15 //пока это просто заглушки
	case "PREPAYMENT_MORTGAGE":
		return 0.10
	case "PREPAYMENT_AUTO":
		return 0.12
	case "PREPAYMENT_MR8_CRED":
		return 0.08
	case "PREPAYMENT_MR8_DEP":
		return 0.05
	default:
		return 0
	}
}

//считает сумму досрочного погашения за интервал [from, to]
// от остатка тела ПОСЛЕ планового платежа тела
func calcPrepayment(
	remainingAfterScheduled float64,
	modelCode string,
	from, to time.Time,
	dayConvention string,
) float64 {
	cpr := getBehaviorCPR(modelCode)
	if cpr <= 0 || remainingAfterScheduled <= 0 {
		return 0
	}

	yearFrac := YearFraction(from, to, dayConvention)
	if yearFrac <= 0 {
		return 0
	}

	// Перевод годовой ставки CPR в ставку за период (SMM)
	smm := 1 - math.Pow(1-cpr, yearFrac)
	prep := roundMoney(remainingAfterScheduled * smm)

	if prep > remainingAfterScheduled {
		return roundMoney(remainingAfterScheduled)
	}
	return prep
}