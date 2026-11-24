package cashflow

import (
	"strings"
	"strconv"
	"fmt"
	"time"
	"math"
)

func roundMoney(x float64) float64 {
    return math.Round(x*100) / 100
}

func roundRate(x float64) float64 {
    return math.Round(x*10000) / 10000
}

//полная ставка до cap/floor: маржа + индекс (только для FLOAT с индексом)
// Для FIXED просто interest_rate
func fullRate(interestRate float64, indicatorRate *float64, rateType string) float64 {
    if rateType == RateFloat && indicatorRate != nil {
        return interestRate + *indicatorRate
    }
    return interestRate
}

//применяет cap/floor к ставке
func clampRate(rate float64, capLimit, floorLimit *float64) float64 {
    if capLimit != nil && rate > *capLimit {
        rate = *capLimit
    }
    if floorLimit != nil && rate < *floorLimit {
        rate = *floorLimit
    }
    return rate
}

func periodsPerYear(period string) (float64, error) {
	if period == "ENDDATE" {
		return 0, nil
	}

	switch {
	case strings.HasSuffix(period, "D"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "D"))
		if n <= 0 {
			return 0, fmt.Errorf("unknown period %q", period)
		}
		return 365.0 / float64(n), nil
	case strings.HasSuffix(period, "M"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "M"))
		if n <= 0 {
			return 0, fmt.Errorf("unknown period %q", period)
		}
		return 12.0 / float64(n), nil
	case strings.HasSuffix(period, "Y"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "Y"))
		if n <= 0 {
			return 0, fmt.Errorf("unknown period %q", period)
		}
		return 1.0 / float64(n), nil
	default:
		if period == "" {
			return 0, nil
		}
		return 0, fmt.Errorf("unknown period %q", period)
	}
}

//число дат тела (без accrualStart) >= t
// Используется для recast PMT на дате репрайсинга: сколько платежей тела осталось
func countPrincipalAfter(principal []scheduleItem, t time.Time) int {
    cnt := 0
    for idx := 1; idx < len(principal); idx++ {   // 0-й — accrualStart, пропускаем
        if !principal[idx].Date.Before(t) {
            cnt++
        }
    }
    return cnt
}

//целое число календарных дней от a до b
// Обе даты обрезаются до полуночи UTC, чтобы избежать влияния времени суток и DST
func daysBetween(a, b time.Time) int {
	aa := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	bb := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(bb.Sub(aa).Hours() / 24)
}

//високосный год
func isLeapYear(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

//доля года по ACT/ACT ISDA
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

//доля года между start и end по правилу convention
// ACT/365: days / 365
// ACT/ACT ISDA
// ACT/ACT (без ISDA) и неизвестное - как ACT/365
func YearFraction(start, end time.Time, convention string) float64 {
	if !end.After(start) {
		return 0
	}

	switch convention {
	case "ACT/365":
		return float64(daysBetween(start, end)) / 365.0

	case "ACT/ACT ISDA":
		return actActISDA(start, end)

	default:
		return float64(daysBetween(start, end)) / 365.0
	}
}