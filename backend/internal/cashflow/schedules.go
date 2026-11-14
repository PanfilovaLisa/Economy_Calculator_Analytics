package cashflow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

//Value зависит от поля schedules:
//- Principal: сумма платежа тела (или 0 остаток, посчитать формулой), -1 - не платеж
//- FixedSteps: годовая ставка в долях (0.05)
type scheduleItem struct {
	Date  time.Time
	Value float64
}

func (si scheduleItem) IsZero() bool {
	return si.Date.IsZero()
}

//всегда shedule[0] - accrualStart - !дата начала первого платежа!, далее только даты будущих платежей
type schedules struct {
	Principal      []scheduleItem
	InterestDates  []scheduleItem //у процентов нету value всегда равно 0, кроме InterestDates[0].value = -1
	Repricing      []scheduleItem //в Repricing и FixedSteps Value - это ставка на которую меняет в соответствующую дату
	FixedSteps     []scheduleItem
}

// собирает все расписания вместе
//Тело: ManualPrincipalPayment -> RegularPayPeriod -> даты процентов -> [maturity]
//Проценты: ManualInterestPayment ->  InterestPayPeriod -> даты тела -> [maturity]
//Если оба пусты -> [maturity] для обоих (не ошибка, просто вырожденный случай)
func buildSchedules(deal Deal, ctx flowContext) (schedules, error) {
	principal, err := buildPrincipal(deal, ctx)
    if err != nil { return schedules{}, err }

    interest, err := buildInterest(deal, ctx)
    if err != nil { return schedules{}, err }

    if principal == nil {
		if deal.Discriminator == DiscSIMPLE { 
			principal = []scheduleItem{{deal.MaturityDate, 0}} 
	    } else {
			if interest != nil {
				principal = interest
			} else {
				principal = []scheduleItem{{deal.MaturityDate, 0}}
			}
		}
    }
    if interest == nil {
        if principal != nil {
            interest = principalToInterest(principal)
        } else {
            interest = []scheduleItem{{deal.MaturityDate, 0}}
        }
    }
	
	var fixedRepricing []scheduleItem
	var repricing []scheduleItem
	if deal.RateType == RateFloat {
		repricing, err = buildFloatRepricing(deal, ctx)
    	if err != nil { return schedules{}, err }
	} else { 
		fixedRepricing, err = buildFixedRepricing(deal.MaturityDate, ctx.StartDate, deal.RepricingRateFixed)
		if err != nil { return schedules{}, err }
	}

    return schedules{
        Principal:     principal,
        InterestDates: interest,
        Repricing:     repricing,
        FixedSteps:    fixedRepricing,
    }, nil
}

func filterManual(manual []scheduleItem, maturity, start time.Time, isRepricing bool) ([]scheduleItem, error) {
	var dates []scheduleItem
	prev := scheduleItem{}
	for _, item := range manual {
		if item.Date.After(maturity) {
			if len(dates) == 0 && !isRepricing {
				return nil, fmt.Errorf("all payments > maturity")
			}
			break
		}
		if (!item.Date.Before(start)) {
			if len(dates) == 0 {
				if isRepricing {
					if prev.IsZero() {
						dates = append(dates, scheduleItem{Date: start, Value: 0})
					} else {
						dates = append(dates, prev)
					}
				} else {
					if prev.IsZero() {
						dates = append(dates, scheduleItem{Date: start, Value: -1})
					} else {
						dates = append(dates, scheduleItem{Date: prev.Date, Value: -1})
					}
				}
			}
			dates = append(dates, item)
		}
		prev = item
	}
	return dates, nil
}

//строит расписание для тела
//только для simple расматриваем ручные графики, все остальные только по правилу
func buildPrincipal(deal Deal, ctx flowContext) ([]scheduleItem, error) {
    if deal.Discriminator == DiscSIMPLE {
		manual, err := parseManualSchedule(deal.ManualPrincipalPayment)
		if err != nil { return nil, fmt.Errorf("principal_payment: %w", err)}
        
		if len(manual) > 0 {
			dates, err := filterManual(manual, deal.MaturityDate, ctx.StartDate, false)
			if err != nil { return nil, fmt.Errorf("principal_payment: %w", err)}
			dates = append(dates, scheduleItem{Date: deal.MaturityDate, Value: 0}) //всегда добавляем close payment (могут повторятся)
            return dates, nil
        }
    }

    anchorDay, hasAnchor := getAnchor(deal.RegularPayDayNum, deal.RegularPayStart)
	//RegularPayStart если этого нет, то просто считаю, что правило не задано
	if deal.RegularPayPeriod != "" && hasAnchor && deal.RegularPayStart != nil {
        if deal.RegularPayPeriod == "ENDDATE" {
            return []scheduleItem{{deal.MaturityDate, 0}}, nil
        }
        dates, err := buildSchedule(*deal.RegularPayStart, deal.MaturityDate, ctx.StartDate, deal.RegularPayPeriod, anchorDay, false)
        if err != nil {
            return nil, fmt.Errorf("principal_payment: %w", err)
        }
        return dates, nil
    }
    return nil, nil // ничего не задано
}

//строит расписание для процентов
//только для simple расматриваем ручные графики, все остальные только по правилу
func buildInterest(deal Deal, ctx flowContext) ([]scheduleItem, error) {
	if deal.Discriminator == DiscSIMPLE {
		manual, err := parseManualDates(deal.ManualInterestPayment)
		if err != nil { return nil, fmt.Errorf("interest_payment: %w", err)}
        
		if len(manual) > 0 {
			dates, err := filterManual(manual, deal.MaturityDate, ctx.StartDate, false)
			if err != nil { return nil, fmt.Errorf("interest_payment: %w", err)}
			if !dates[len(dates)-1].Date.Equal(deal.MaturityDate) {
				dates = append(dates, scheduleItem{Date: deal.MaturityDate, Value: 0}) //не всегда добавляем MaturityDate (только если нету)
			}
            return dates, nil
        }
    }

    anchorDay, hasAnchor := getAnchor(deal.InterestPayDayNum, deal.InterestPayStart)
	//InterestPayStart если этого нет, то просто считаю, что правило не задано
	if deal.InterestPayPeriod != "" && hasAnchor && deal.InterestPayStart != nil {
        if deal.InterestPayPeriod == "ENDDATE" {
            return []scheduleItem{{deal.MaturityDate, 0}}, nil
        }
        dates, err := buildSchedule(*deal.InterestPayStart, deal.MaturityDate, ctx.StartDate, deal.InterestPayPeriod, anchorDay, false)
        if err != nil {
            return nil, fmt.Errorf("interest_payment: %w", err)
        }
        return dates, nil
    }
    return nil, nil // ничего не задано
}

//для поля repricing_dttm_rate_schedule_fixed
//если ставка FIXED, то на поля repricing_dttm_schedule и repricing_dttm_rate_schedule не обращаем внимания
func buildFixedRepricing(maturity, start time.Time, repricingRateFixed string) ([]scheduleItem, error) {
	manual, err := parseManualSchedule(repricingRateFixed)
	if err != nil { return nil, fmt.Errorf("fixed_repricing: %w", err) }
	
	if len(manual) > 0 {
		dates, err := filterManual(manual, maturity, start, true)
		if err != nil { return nil, fmt.Errorf("fixed_repricing: %w", err)}
		return dates, nil
	}

	return nil, nil
}

//даты либо по правилу, либо по repricing_dttm_schedule, тогда ставки из cbkey
//либо если есть repricing_dttm_rate_schedule и даты, и ставки отсюда
func buildFloatRepricing(deal Deal, ctx flowContext) ([]scheduleItem, error) {
	manual, err := parseManualSchedule(deal.RepricingRateFloat);
	if err != nil { return nil, fmt.Errorf("float_repricing: %w", err) }
	if len(manual) > 0 {
		dates, err := filterManual(manual, deal.MaturityDate, ctx.StartDate, true)
		if err != nil { return nil, fmt.Errorf("float_repricing: %w", err) }
		return dates, nil
	}

    anchorDay, hasAnchor := getAnchor(deal.RepricingRuleDayNum, deal.RepricingRuleStart)
	//RepricingRuleStart если этого нет, то просто считаю, что правило не задано
	if deal.RepricingRulePeriod != "" && hasAnchor && deal.RepricingRuleStart != nil {
        if deal.RepricingRulePeriod == "ENDDATE" {
			return []scheduleItem{{Date: deal.MaturityDate, Value: 0}}, nil
		}
        dates, err := buildSchedule(*deal.RepricingRuleStart, deal.MaturityDate, ctx.StartDate, deal.RepricingRulePeriod, anchorDay, true)
        if err != nil { return nil, fmt.Errorf("float_repricing: %w", err) }

		//дальше ищем в cbkey ставки которые актуальны на дату и заполняем value
		for i := range dates {
			rate, ok := ctx.Indices.rateAt(dates[i].Date)
			if !ok {
				break
			}
			dates[i].Value = rate
		}

		return dates, nil
    }

    return nil, nil //ничего не задано
}

//из расписания тела делает расписание для процентов
func principalToInterest(principal []scheduleItem) ([]scheduleItem) {
	if principal == nil { return nil}

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

//парсит principal_payment: "2022-12-31,250000 2023-01-31,250000"
//и парсит repricing_dttm_rate_schedule_fixed: "2022-08-31,0.09"
//и сортирует
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

//парсит interest_payment / repricing_dttm_schedule и сортирует
//"2022-09-30 2022-10-31 2022-11-30".
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

//парсит дату в одном из поддерживаемых форматов:
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