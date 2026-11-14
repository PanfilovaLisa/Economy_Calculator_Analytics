package cashflow

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

//есть два способа что делать если period > 1М и startPayRule.Day() < anchorDay: 
// 1) первый платеж anchorDay в том же месяце (первый период короткий)
// 2) первый платеж startPayRule + period в этом месяце в anchorDay (первый период длинный) <- делаем это
func buildSchedule(startPayRule, maturity, start time.Time, period string, anchorDay int, isRepricing bool) ([]scheduleItem, error) {
	var dates []scheduleItem

	cur := startPayRule
	for {
		next, err := nextDate(cur, period, anchorDay)
		if err != nil {
			return nil, fmt.Errorf("buildShedule: %w", err)
		} else if (next == cur) {
			dates = append(dates, scheduleItem{Date: cur, Value: -1})
			break
		}

		if !next.Before(maturity) {
			if (len(dates) == 0) {
				dates = append(dates, scheduleItem{Date: cur, Value: -1})
			}
			break
		}
		if (!next.Before(start)) {
			if len(dates) == 0 {
				dates = append(dates, scheduleItem{Date: cur, Value: -1})
			}
			dates = append(dates, scheduleItem{Date: next, Value: 0}) //добавляем только будущие даты
		}
		cur = next
	}
	if !isRepricing {
		//maturity не нужно добавлять для repricing
		dates = append(dates, scheduleItem{Date: maturity, Value: 0}) //maturity должен быть всегда, но один, без другого close payment
	}
	return dates, nil
}

//следующая дата после cur по period: "1M", "45D", "1Y"
//ENDDATE возвращаем cur
//anchorDay: >0 - фиксированный день месяца; -1 - последний день месяца; 0 - без фиксации
func nextDate(cur time.Time, period string, anchorDay int) (time.Time, error) {
	if period == "ENDDATE" {
		return cur, nil
	}

	var next time.Time
	switch {
	case strings.HasSuffix(period, "D"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "D")) // strconv.Atoi("") => 0, err
		if n <= 0 {
			return time.Time{}, fmt.Errorf("unknown period %q", period)
		}
		next = cur.AddDate(0, 0, n)
		return next, nil //anchorDay игнорируется, потому что день - это день, а не день месяца
	case strings.HasSuffix(period, "M"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "M"))
		if n <= 0 {
			return time.Time{}, fmt.Errorf("unknown period %q", period)
		}
		next = addMonths(cur, n)
	case strings.HasSuffix(period, "Y"):
		n, _ := strconv.Atoi(strings.TrimSuffix(period, "Y"))
		if n <= 0 {
			return time.Time{}, fmt.Errorf("unknown period %q", period)
		}
		next = addYears(cur, n)
	default:
		return time.Time{}, fmt.Errorf("unknown period %q", period)
	}

	if anchorDay == -1 {
		//последний день, делаем +1,тк 0 день месяца это день перед этим месяцем
		return time.Date(next.Year(), next.Month()+1, 0, 0, 0, 0, 0, next.Location()), nil
	}
	if anchorDay > 0 {
		lastDay := time.Date(next.Year(), next.Month()+1, 0, 0, 0, 0, 0, next.Location()).Day()
		d := anchorDay
		if d > lastDay {
			d = lastDay
		}
		return time.Date(next.Year(), next.Month(), d, 0, 0, 0, 0, next.Location()), nil
	}

	return next, nil
}

//прибавление месяцев с клампингом дня (31 в феврале => 28/29)
//months >= 0
func addMonths(t time.Time, months int) time.Time {
	year, month, day := t.Date()
	newMonth := month + time.Month(months) //type Month int - от 1 до 12
	newYear := year + (int(newMonth)-1)/12
	nm := (int(newMonth)-1)%12 + 1
	lastDay := time.Date(newYear, time.Month(nm)+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(newYear, time.Month(nm), day, 0, 0, 0, 0, t.Location())
}

//прибавление лет с клампингом дня (29.02 в невисокосный -> 28.02)
func addYears(t time.Time, years int) time.Time {
	return addMonths(t, years*12)
}