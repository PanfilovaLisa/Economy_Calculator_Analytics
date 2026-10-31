package cashflow

import (
	"math"
	"strconv"
	"strings"
	"time"
	"analytics/configs"
)

type CashFlowService struct {
	Config *configs.Config
}

func NewCashFlowService(conf *configs.Config) *CashFlowService {
	return &CashFlowService{
		Config: conf,
	}
}

func GenerateCashFlows(deal Deal) []CashFlow {
	var cfs []CashFlow

	if deal.CurrentPrincipalAmt == 0 || !deal.ReportDate.Before(deal.MaturityDate) {
		return cfs
	}

	isDeposit := deal.CurrentPrincipalAmt < 0
	remainingBalance := math.Abs(deal.CurrentPrincipalAmt)

	// Период шага: процентный период; ENDDATE — это bullet, не шаг.
	payPeriod := deal.InterestPayPeriod
	if payPeriod == "" || strings.EqualFold(payPeriod, "ENDDATE") {
		payPeriod = deal.RegularPayPeriod
	}
	if payPeriod == "" || strings.EqualFold(payPeriod, "ENDDATE") {
		payPeriod = "1M"
	}

	// Признак bullet-погашения тела (тело только в Maturity)
	bulletPrincipal := strings.EqualFold(deal.RegularPayPeriod, "ENDDATE") || deal.RegularPayPeriod == ""

	anchorDay := getAnchorDay(deal)

	currentStart := findPreviousPaymentDate(deal, deal.ReportDate, payPeriod, anchorDay)
	if currentStart.Before(deal.DealDate) {
		currentStart = deal.DealDate
	}

	if deal.Discriminator == DiscSIMPLE && deal.ManualPrincipalPayment != "" {
		return generateManualSimpleFlows(deal, currentStart, isDeposit)
	}

	remainingPayments := countRemainingPeriods(currentStart, deal.MaturityDate, payPeriod, anchorDay)
	if remainingPayments == 0 {
		remainingPayments = 1
	}

	var fixedPMT float64
	if deal.Discriminator == DiscANN && !bulletPrincipal {
		fixedPMT = calcAnnuityPMT(remainingBalance, deal.InterestRate, remainingPayments)
	}

	for currentStart.Before(deal.MaturityDate) {
		paymentDate := GetNextPaymentDate(currentStart, payPeriod, anchorDay)
		if paymentDate.After(deal.MaturityDate) {
			paymentDate = deal.MaturityDate
		}

		isLast := paymentDate.Equal(deal.MaturityDate)

		yearFraction := YearFraction(currentStart, paymentDate, deal.DayConvention)
		interestAmt := roundMoney(remainingBalance * deal.InterestRate * yearFraction)

		var principalAmt float64
		if isLast {
			principalAmt = remainingBalance
		} else if bulletPrincipal {
			principalAmt = 0
		} else {
			switch deal.Discriminator {
			case DiscANN:
				principalAmt = fixedPMT - interestAmt
				if principalAmt > remainingBalance {
					principalAmt = remainingBalance
				}
				if principalAmt < 0 {
					principalAmt = 0
				}
			case DiscRGM:
				if remainingPayments > 0 {
					principalAmt = roundMoney(remainingBalance / float64(remainingPayments))
				} else {
					principalAmt = remainingBalance
				}
			default:
				principalAmt = 0.0
			}
		}

		principalAmt = roundMoney(principalAmt)
		balanceBeforePayment := remainingBalance
		remainingBalance = roundMoney(remainingBalance - principalAmt)

		pType := TypePrincipalPayment
		if isLast {
			pType = TypeClosePayment
		}

		daysToPayment := int(paymentDate.Sub(deal.ReportDate).Hours() / 24)
		bucket := GetTimeBucket(daysToPayment)

		sign := 1.0
		if isDeposit {
			sign = -1.0
		}

		if interestAmt != 0 {
			cfs = append(cfs, CashFlow{
				LoadDate:         time.Now(),
				SourceCode:       "CSV_UPLOAD",
				DealID:           deal.DealID,
				Currency:         deal.Currency,
				StartDate:        currentStart,
				PaymentDate:      paymentDate,
				DaysToPayment:    daysToPayment,
				TimeBucket:       bucket,
				PaymentType:      TypeInterestPayment,
				PrincipalAmount:  0.0,
				InterestAmount:   roundMoney(interestAmt * sign),
				RemainingBalance: roundMoney(balanceBeforePayment * sign),
			})
		}

		if principalAmt != 0 || isLast {
			cfs = append(cfs, CashFlow{
				LoadDate:         time.Now(),
				SourceCode:       "CSV_UPLOAD",
				DealID:           deal.DealID,
				Currency:         deal.Currency,
				StartDate:        currentStart,
				PaymentDate:      paymentDate,
				DaysToPayment:    daysToPayment,
				TimeBucket:       bucket,
				PaymentType:      pType,
				PrincipalAmount:  roundMoney(principalAmt * sign),
				InterestAmount:   0.0,
				RemainingBalance: roundMoney(math.Max(0, remainingBalance) * sign),
			})
		}

		currentStart = paymentDate
		if remainingPayments > 0 {
			remainingPayments--
		}
	}

	return cfs
}

func findPreviousPaymentDate(deal Deal, reportDate time.Time, payPeriod string, anchorDay int) time.Time {
	if payPeriod == "" || strings.EqualFold(payPeriod, "ENDDATE") {
		return deal.DealDate
	}

	base := time.Date(deal.DealDate.Year(), deal.DealDate.Month(), anchorDay, 0, 0, 0, 0, deal.DealDate.Location())
	if base.Before(deal.DealDate) {
		base = GetNextPaymentDate(base, payPeriod, anchorDay)
	}

	prev := base
	current := base
	for iter := 0; iter < 5000; iter++ {   // защита от зацикливания
		next := GetNextPaymentDate(current, payPeriod, anchorDay)
		if !next.After(current) {
			// шаг не двигает дату — выходим, чтобы не зациклиться
			return current
		}
		if next.After(reportDate) || next.Equal(reportDate) {
			return current
		}
		prev = current
		current = next
		if current.After(reportDate) {
			return prev
		}
	}
	return current
}

// Расчет для кастомных графиков SIMPLE (амортизация и досрочные погашения)
func generateManualSimpleFlows(deal Deal, startDate time.Time, isDeposit bool) []CashFlow {
	var cfs []CashFlow
	manualPayments := parseManualSchedule(deal.ManualPrincipalPayment)

	remainingBalance := math.Abs(deal.CurrentPrincipalAmt)
	currentStart := startDate
	sign := 1.0
	if isDeposit {
		sign = -1.0
	}

	for _, mp := range manualPayments {
		if !mp.Date.After(startDate) {
			continue
		}

		paymentDate := mp.Date
		if paymentDate.After(deal.MaturityDate) {
			paymentDate = deal.MaturityDate
		}

		yearFraction := YearFraction(currentStart, paymentDate, deal.DayConvention)
		interestAmt := roundMoney(remainingBalance * deal.InterestRate * yearFraction)

		principalAmt := math.Abs(mp.Amount)
		isLast := paymentDate.Equal(deal.MaturityDate)
		if isLast || principalAmt > remainingBalance {
			principalAmt = remainingBalance
		}

		balanceBeforePayment := remainingBalance
		remainingBalance = roundMoney(remainingBalance - principalAmt)
		daysToPayment := int(paymentDate.Sub(deal.ReportDate).Hours() / 24)
		bucket := GetTimeBucket(daysToPayment)

		if interestAmt != 0 {
			cfs = append(cfs, CashFlow{
				LoadDate:         time.Now(),
				SourceCode:       "CSV_UPLOAD",
				DealID:           deal.DealID,
				Currency:         deal.Currency,
				StartDate:        currentStart,
				PaymentDate:      paymentDate,
				DaysToPayment:    daysToPayment,
				TimeBucket:       bucket,
				PaymentType:      TypeInterestPayment,
				PrincipalAmount:  0.0,
				InterestAmount:   roundMoney(interestAmt * sign),
				RemainingBalance: roundMoney(balanceBeforePayment * sign),
			})
		}

		if principalAmt != 0 {
			pType := TypePrepayment
			if isLast || remainingBalance == 0 {
				pType = TypeClosePayment
			}
			cfs = append(cfs, CashFlow{
				LoadDate:         time.Now(),
				SourceCode:       "CSV_UPLOAD",
				DealID:           deal.DealID,
				Currency:         deal.Currency,
				StartDate:        currentStart,
				PaymentDate:      paymentDate,
				DaysToPayment:    daysToPayment,
				TimeBucket:       bucket,
				PaymentType:      pType,
				PrincipalAmount:  roundMoney(principalAmt * sign),
				InterestAmount:   0.0,
				RemainingBalance: roundMoney(remainingBalance * sign),
			})
		}

		currentStart = paymentDate
		if remainingBalance == 0 {
			break
		}
	}

	// Финальный Bullet платеж, если график закончился, а ОД остался
	if remainingBalance > 0 && currentStart.Before(deal.MaturityDate) {
		yearFraction := YearFraction(currentStart, deal.MaturityDate, deal.DayConvention)
		interestAmt := roundMoney(remainingBalance * deal.InterestRate * yearFraction)
		daysToPayment := int(deal.MaturityDate.Sub(deal.ReportDate).Hours() / 24)
		bucket := GetTimeBucket(daysToPayment)

		if interestAmt != 0 {
			cfs = append(cfs, CashFlow{
				LoadDate:         time.Now(),
				SourceCode:       "CSV_UPLOAD",
				DealID:           deal.DealID,
				Currency:         deal.Currency,
				StartDate:        currentStart,
				PaymentDate:      deal.MaturityDate,
				DaysToPayment:    daysToPayment,
				TimeBucket:       bucket,
				PaymentType:      TypeInterestPayment,
				PrincipalAmount:  0.0,
				InterestAmount:   roundMoney(interestAmt * sign),
				RemainingBalance: roundMoney(remainingBalance * sign),
			})
		}

		cfs = append(cfs, CashFlow{
			LoadDate:         time.Now(),
			SourceCode:       "CSV_UPLOAD",
			DealID:           deal.DealID,
			Currency:         deal.Currency,
			StartDate:        currentStart,
			PaymentDate:      deal.MaturityDate,
			DaysToPayment:    daysToPayment,
			TimeBucket:       bucket,
			PaymentType:      TypeClosePayment,
			PrincipalAmount:  roundMoney(remainingBalance * sign),
			InterestAmount:   0.0,
			RemainingBalance: 0.0,
		})
	}

	return cfs
}


// сдвигает дату вперед (используется только для дней; месяцы считаются в GetNextPaymentDate)
func AddPeriod(date time.Time, period string) time.Time {
	period = strings.ToUpper(strings.TrimSpace(period))
	if period == "ENDDATE" || period == "" {
		return date
	}

	if strings.HasSuffix(period, "D") {
		daysStr := strings.TrimSuffix(period, "D")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return date
		}
		return date.AddDate(0, 0, days)
	}

	return date
}

// вычисляет "якорный" день платежа из правил сделки (например, 22-е число)
func getAnchorDay(deal Deal) int {
	if deal.RegularPayDayNum != nil {
		return *deal.RegularPayDayNum
	}
	if deal.RegularPayStart != nil {
		return deal.RegularPayStart.Day()
	}
	if deal.InterestPayDayNum != nil {
		return *deal.InterestPayDayNum
	}
	if deal.InterestPayStart != nil {
		return deal.InterestPayStart.Day()
	}
	return deal.DealDate.Day()
}

// рассчитывает следующую дату платежа с жесткой привязкой к дню
func GetNextPaymentDate(currentStart time.Time, period string, anchorDay int) time.Time {
	period = strings.ToUpper(strings.TrimSpace(period))

	if strings.HasSuffix(period, "M") {
		monthsStr := strings.TrimSuffix(period, "M")
		months, err := strconv.Atoi(monthsStr)
		if err != nil || months == 0 {
			months = 1
		}

		y, m, _ := currentStart.Date()
		targetMonth := time.Month(int(m) + months)
		targetYear := y + (int(targetMonth)-1)/12
		targetMonth = time.Month((int(targetMonth)-1)%12 + 1)

		lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, currentStart.Location()).Day()

		d := anchorDay
		if anchorDay == -1 || d > lastDay {
			d = lastDay
		}
		if d <= 0 {
			d = lastDay
		}

		return time.Date(targetYear, targetMonth, d, 0, 0, 0, 0, currentStart.Location())
	}

	// Для "D" — просто добавляем дни
	return AddPeriod(currentStart, period)
}

// вычисляет долю года с поддержкой стандартов ACT/365 и ACT/ACT ISDA
func YearFraction(start, end time.Time, convention string) float64 {
	if start.After(end) || start.Equal(end) {
		return 0.0
	}
	days := end.Sub(start).Hours() / 24

	switch strings.ToUpper(convention) {
	case "ACT/365":
		return days / 365.0
	case "ACT/ACT", "ACT/ACT ISDA":
		yStart := start.Year()
		yEnd := end.Year()
		if yStart == yEnd {
			div := 365.0
			if isLeapYear(yStart) {
				div = 366.0
			}
			return days / div
		}

		sum := 0.0
		// Первый неполный год: от start до 01.01.(yStart+1)
		endOfYear1 := time.Date(yStart+1, 1, 1, 0, 0, 0, 0, time.UTC)
		days1 := endOfYear1.Sub(start).Hours() / 24
		div1 := 365.0
		if isLeapYear(yStart) {
			div1 = 366.0
		}
		sum += days1 / div1

		// Промежуточные полные годы
		for y := yStart + 1; y < yEnd; y++ {
			sum += 1.0
		}

		// Последний неполный год: от 01.01.yEnd до end
		startOfYear2 := time.Date(yEnd, 1, 1, 0, 0, 0, 0, time.UTC)
		days2 := end.Sub(startOfYear2).Hours() / 24
		div2 := 365.0
		if isLeapYear(yEnd) {
			div2 = 366.0
		}
		sum += days2 / div2
		return sum
	default:
		return days / 365.0
	}
}

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

type manualPaymentTuple struct {
	Date   time.Time
	Amount float64
}

func parseManualSchedule(scheduleStr string) []manualPaymentTuple {
	var list []manualPaymentTuple
	tokens := strings.Fields(scheduleStr)
	for _, t := range tokens {
		parts := strings.Split(t, ",")
		if len(parts) != 2 {
			continue
		}
		dt, err := time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		amtStr := strings.Replace(strings.TrimSpace(parts[1]), ",", ".", -1)
		amt, err := strconv.ParseFloat(amtStr, 64)
		if err != nil {
			continue
		}
		list = append(list, manualPaymentTuple{Date: dt, Amount: amt})
	}
	return list
}

func calcAnnuityPMT(principal, annualRate float64, periods int) float64 {
	if periods <= 0 || principal <= 0 {
		return 0.0
	}
	if annualRate == 0 {
		return principal / float64(periods)
	}
	monthlyRate := annualRate / 12.0
	denom := 1.0 - math.Pow(1.0+monthlyRate, float64(-periods))
	if denom == 0 {
		return 0.0
	}
	return principal * (monthlyRate / denom)
}

func countRemainingPeriods(start, end time.Time, period string, anchorDay int) int {
	if start.After(end) || start.Equal(end) {
		return 0
	}
	if period == "ENDDATE" || period == "" {
		return 1
	}

	count := 0
	current := start
	for current.Before(end) {
		next := GetNextPaymentDate(current, period, anchorDay)
		count++

		if next.After(end) || next.Equal(end) {
			break
		}
		current = next
	}
	return count
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

func roundMoney(x float64) float64 {
	return math.Round(x*100) / 100
}