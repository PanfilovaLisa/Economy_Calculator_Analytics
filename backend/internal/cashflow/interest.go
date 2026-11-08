package cashflow

import (
	"math"
	"strings"
	"time"
	"strconv"
)

// daysBetween - целое число календарных дней от a до b
// Обе даты обрезаются до полуночи UTC, чтобы избежать влияния времени суток и DST
func daysBetween(a, b time.Time) int {
	aa := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	bb := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(bb.Sub(aa).Hours() / 24)
}

// roundMoney - округление до копеек (2 знака)
func roundMoney(x float64) float64 {
	return math.Round(x*100) / 100
}

// isLeapYear - високосный год.
func isLeapYear(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

// YearFraction - доля года между start и end по правилу convention
// ACT/365: days / 365
// ACT/ACT ISDA
// ACT/ACT (без ISDA) и неизвестное - как ACT/365
func YearFraction(start, end time.Time, convention string) float64 {
	if !end.After(start) {
		return 0
	}
	conv := strings.ToUpper(strings.TrimSpace(convention))

	switch conv {
	case "ACT/365":
		return float64(daysBetween(start, end)) / 365.0

	case "ACT/ACT ISDA":
		return actActISDA(start, end)

	default:
		return float64(daysBetween(start, end)) / 365.0
	}
}

// actActISDA — доля года по ACT/ACT ISDA
func actActISDA(start, end time.Time) float64 {
	if start.Year() == end.Year() {
		div := 365.0
		if isLeapYear(start.Year()) {
			div = 366.0
		}
		return float64(daysBetween(start, end)) / div
	}

	sum := 0.0

	// Первый год: от start до 31.12 включительно
	endOfY1 := time.Date(start.Year(), 12, 31, 0, 0, 0, 0, start.Location())
	div1 := 365.0
	if isLeapYear(start.Year()) {
		div1 = 366.0
	}
	sum += float64(daysBetween(start, endOfY1)+1) / div1

	// Полные годы между
	for y := start.Year() + 1; y < end.Year(); y++ {
		sum += 1.0
	}

	// Последний год: от 01.01 до end (невключительно)
	startOfY2 := time.Date(end.Year(), 1, 1, 0, 0, 0, 0, end.Location())
	div2 := 365.0
	if isLeapYear(end.Year()) {
		div2 = 366.0
	}
	sum += float64(daysBetween(startOfY2, end)) / div2

	return sum
}

// effectiveRate — годовая ставка сделки с учётом FLOAT и cap/floor
// Возвращает ставку в долях
func effectiveRate(deal Deal) float64 {
	r := deal.InterestRate
	if strings.EqualFold(deal.RateType, "FLOAT") && deal.BaseIndicatorRate != nil {
		r += *deal.BaseIndicatorRate
	}
	if deal.CapLimitInterestRate != nil && r > *deal.CapLimitInterestRate {
		r = *deal.CapLimitInterestRate
	}
	if deal.FloorLimitInterestRate != nil && r < *deal.FloorLimitInterestRate {
		r = *deal.FloorLimitInterestRate
	}
	return r
}

// periodsPerYear — сколько периодов данного типа в году.
// "1M" -> 12, "3M" -> 4, "1Y" -> 1, "45D" -> 365/45.
func periodsPerYear(period string) float64 {
	period = strings.ToUpper(strings.TrimSpace(period))
	switch {
	case strings.HasSuffix(period, "D"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "D"))
		if n <= 0 {
			return 365
		}
		return 365.0 / float64(n)
	case strings.HasSuffix(period, "M"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "M"))
		if n <= 0 {
			return 12
		}
		return 12.0 / float64(n)
	case strings.HasSuffix(period, "Y"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "Y"))
		if n <= 0 {
			return 1
		}
		return 1.0 / float64(n)
	}
	return 12
}

// calcAnnuityPMT — фиксированный платёж аннуитета
// period нужен, чтобы ставка за период считалась корректно:
// для "1M" r = annualRate/12, для "3M" r = annualRate/4, для "1Y" r = annualRate.
func calcAnnuityPMT(principal, annualRate float64, n int, period string) float64 {
	if n <= 0 || principal <= 0 {
		return 0
	}
	if annualRate == 0 {
		return principal / float64(n)
	}
	r := annualRate / periodsPerYear(period)
	denom := 1 - math.Pow(1+r, float64(-n))
	if denom == 0 {
		return 0
	}
	return principal * r / denom
}

// makeRateAt — замыкание «ставка на дату»
// Покрывает три сценария:
//   - FixedSteps задан (FIXED со ступенями): ставка по последней ступени <= t
//   - FLOAT с RepricingDates/Rates: ставка по последней переоценке <= t
//   - иначе: базовая effectiveRate(deal)
func makeRateAt(deal Deal, sched schedules, indices map[string]indexSeries) func(time.Time) float64 {
    // 1. FIXED со ступенями.
    if len(sched.FixedSteps) > 0 {
        steps := sched.FixedSteps
        return func(t time.Time) float64 {
            r := steps[0].Value
            for _, s := range steps {
                if s.Date.After(t) {
                    break
                }
                r = s.Value
            }
            return clampRate(deal, r)
        }
    }

    // 2. FLOAT с явными ставками переоценки.
    if len(sched.RepricingDates) > 0 && len(sched.RepricingRates) == len(sched.RepricingDates) {
        dates := sched.RepricingDates
        rates := sched.RepricingRates
        return func(t time.Time) float64 {
            r := rates[0]
            for i, d := range dates {
                if d.After(t) {
                    break
                }
                r = rates[i]
            }
            return clampRate(deal, r)
        }
    }

    // 3. FLOAT с индексом (CBKEY1M и т.п.).
    // Семантика: interest_rate — это маржа, индекс берётся из файла на дату t.
    if strings.EqualFold(deal.RateType, "FLOAT") && deal.BaseIndicator != "" {
        if series, ok := indices[deal.BaseIndicator]; ok {
            margin := deal.InterestRate
            return func(t time.Time) float64 {
                idx, ok := series.rateAt(t)
                if !ok {
                    return clampRate(deal, margin)
                }
                return clampRate(deal, margin+idx)
            }
        }
        // индекс не найден — падаем на базовую
    }

    // 4. Базовая.
    base := effectiveRate(deal)
    return func(time.Time) float64 { return base }
}

// clampRate — cap/floor на уже полученную ставку.
func clampRate(deal Deal, r float64) float64 {
	if deal.CapLimitInterestRate != nil && r > *deal.CapLimitInterestRate {
		r = *deal.CapLimitInterestRate
	}
	if deal.FloorLimitInterestRate != nil && r < *deal.FloorLimitInterestRate {
		r = *deal.FloorLimitInterestRate
	}
	return r
}