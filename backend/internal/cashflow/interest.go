package cashflow

import (
	"math"
	"strings"
	"time"
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