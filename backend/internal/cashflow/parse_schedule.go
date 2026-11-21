package cashflow

import (
	"strings"
	"sort"
	"strconv"
	"time"
	"fmt"
)

// из расписания тела делает расписание для процентов
func principalToInterest(principal []scheduleItem) []scheduleItem {
	if principal == nil {
		return nil
	}

	var interest []scheduleItem
	prev := time.Time{}
	for _, item := range principal {
		//повторяющиеся у тела могут быть только в maturity, у процентов не должно быть повторяющихся дат
		if prev != item.Date {
			interest = append(interest, scheduleItem{Date: item.Date, Value: 0})
			prev = item.Date
		}
	}
	return interest
}

// парсит principal_payment: "2022-12-31,250000 2023-01-31,250000"
// и парсит repricing_dttm_rate_schedule_fixed: "2022-08-31,0.09"
// и сортирует
func parseManualSchedule(s string) ([]scheduleItem, error) {
	if s == "" {
		return nil, nil
	}
	var list []scheduleItem
	for _, tok := range strings.Fields(s) {
		parts := strings.SplitN(tok, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("token %q: expected 'date,amount'", tok)
		}
		dt, err := parseDate(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("token %q: date: %w", tok, err)
		}
		amt, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("token %q: amount: %w", tok, err)
		}
		list = append(list, scheduleItem{Date: dt, Value: amt})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Date.Before(list[j].Date)
	})
	return list, nil
}

// парсит interest_payment / repricing_dttm_schedule и сортирует
// "2022-09-30 2022-10-31 2022-11-30".
func parseManualDates(s string) ([]scheduleItem, error) {
	if s == "" {
		return nil, nil
	}
	var list []scheduleItem
	for _, tok := range strings.Fields(s) {
		dt, err := parseDate(tok)
		if err != nil {
			return nil, fmt.Errorf("token %q: date: %w", tok, err)
		}
		list = append(list, scheduleItem{Date: dt, Value: 0})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Date.Before(list[j].Date)
	})
	return list, nil
}

// парсит дату в одном из поддерживаемых форматов:
// 2006-01-02 или 02.01.2006. Если не подошёл ни один — ошибка
func parseDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Parse("02.01.2006", s)
}

func getAnchor(dayNum *int, start *time.Time) (int, bool) {
	if dayNum != nil {
		if *dayNum > 0 {
			return *dayNum, true
		}
		if *dayNum == -1 {
			return -1, true
		}
	}
	if start != nil {
		return start.Day(), true
	}
	return 0, false
}
