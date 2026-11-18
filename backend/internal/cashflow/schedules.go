package cashflow

import (
	"fmt"
	"time"
)

type scheduleItem struct {
	Date  time.Time
	Value float64
}

func (si scheduleItem) IsZero() bool {
	return si.Date.IsZero()
}

// всегда shedule[0] - accrualStart - !дата начала первого платежа!, далее только даты будущих платежей
type schedules struct {
	//Principal[0] - accrualStartPrincipal - !дата начала последнего прошедшего платежа!, далее только даты будущих платежей
	Principal  []scheduleItem
	PrincipalPeriod string
	//Interest[0] - accrualStartInterest - !дата начала последнего прошедшего платежа!, далее только даты будущих платежей
	Interest   []scheduleItem //у процентов нету value всегда равно 0, кроме InterestDates[0].value = -1
	InterestPeriod string
	//!в Repricing, FixedSteps может быть несколько платежей в прошлом!
	Repricing  []scheduleItem //в Repricing и FixedSteps Value - это ставка на которую меняет в соответствующую дату
	RepricingPeriod string
	FixedSteps []scheduleItem
}

// собирает все расписания вместе
// Тело: ManualPrincipalPayment -> RegularPayPeriod -> даты процентов -> [maturity]
// Проценты: ManualInterestPayment ->  InterestPayPeriod -> даты тела -> [maturity]
// Если оба пусты -> [maturity] для обоих (не ошибка, просто вырожденный случай)
// если enddate, то расписание тела = []scheduleItem{{ctx.StartDate, -1}} состоит только из этого так как {deal.MaturityDate, 0} - close payment
// для процентов и репрайсинга []scheduleItem{{ctx.StartDate, -1}, {deal.MaturityDate, 0}}
func buildSchedules(deal Deal, ctx flowContext) (schedules, error) {
	principal, pPeriod, err := buildPrincipal(deal, ctx)
	if err != nil {
		return schedules{}, err
	}

	interest, iPeriod, err := buildInterest(deal, ctx)
	if err != nil {
		return schedules{}, err
	}

	if principal == nil {
		if deal.Discriminator == DiscSIMPLE {
			principal = []scheduleItem{{ctx.StartDate, -1}} //enddate
			pPeriod = "ENDDATE"
		} else {
			if interest != nil {
				principal = interest
				pPeriod = iPeriod
			} else {
				principal = []scheduleItem{{ctx.StartDate, -1}}
				pPeriod = "ENDDATE"
			}
		}
	}
	if interest == nil {
		if principal != nil {
			interest = principalToInterest(principal)
			iPeriod = pPeriod
		} else {
			interest = []scheduleItem{{ctx.StartDate, -1}, {deal.MaturityDate, 0}}
			iPeriod = "ENDDATE"
		}
	}

	minAccrualStart := principal[0].Date
	if interest[0].Date.Before(minAccrualStart) {
		//минимальная дата из дат: последнего прошлого платежа тела и последнего прошлого платежа процентов  
	    // нужно потому, что в репрайсинге важны все даты с начала minAccrualStart, для него не работает контракт sched[0] прошедший в нем прошедших может быть много
		minAccrualStart = interest[0].Date
	}

	var fixedRepricing []scheduleItem
	var repricing []scheduleItem
	var rPeriod string = ""
	if deal.RateType == RateFloat {
		repricing, rPeriod, err = buildFloatRepricing(deal, ctx, minAccrualStart)
    	if err != nil { return schedules{}, err }
	} else { 
		fixedRepricing, err = buildFixedRepricing(deal.MaturityDate, minAccrualStart, deal.RepricingRateFixed)
		if err != nil { return schedules{}, err }
	}

	return schedules{
		Principal:  principal,
		PrincipalPeriod: pPeriod,
		Interest:   interest,
		InterestPeriod: iPeriod,
		Repricing:  repricing,
		RepricingPeriod: rPeriod,
		FixedSteps: fixedRepricing,
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
		if !item.Date.Before(start) {
			if len(dates) == 0 {
				if isRepricing {
					if prev.IsZero() {
						dates = append(dates, scheduleItem{Date: start, Value: -1})
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

// строит расписание для тела
// только для simple расматриваем ручные графики, все остальные только по правилу
func buildPrincipal(deal Deal, ctx flowContext) ([]scheduleItem, string, error) {
	if deal.Discriminator == DiscSIMPLE {
		manual, err := parseManualSchedule(deal.ManualPrincipalPayment)
		if err != nil {
			return nil, "", fmt.Errorf("principal_payment: %w", err)
		}

		if len(manual) > 0 {
			dates, err := filterManual(manual, deal.MaturityDate, ctx.StartDate, false)
			if err != nil {
				return nil, "", fmt.Errorf("principal_payment: %w", err)
			}
			if !dates[len(dates)-1].Date.Equal(deal.MaturityDate) {
				dates = append(dates, scheduleItem{Date: deal.MaturityDate, Value: 0})
			}
			return dates, "", nil
		}
	}

	anchorDay, hasAnchor := getAnchor(deal.RegularPayDayNum, deal.RegularPayStart)
	//RegularPayStart если этого нет, то просто считаю, что правило не задано
	if deal.RegularPayPeriod != "" && hasAnchor && deal.RegularPayStart != nil {
		if deal.RegularPayPeriod == "ENDDATE" {
			return []scheduleItem{{ctx.StartDate, -1}}, "ENDDATE", nil
		}
		dates, err := buildSchedule(*deal.RegularPayStart, deal.MaturityDate, ctx.StartDate, deal.RegularPayPeriod, anchorDay)
		if err != nil {
			return nil, "", fmt.Errorf("principal_payment: %w", err)
		}
		if !dates[len(dates)-1].Date.Equal(deal.MaturityDate) {
			dates = append(dates, scheduleItem{Date: deal.MaturityDate, Value: 0})
		}
		return dates, deal.RegularPayPeriod, nil
	}
	return nil, "", nil // ничего не задано
}

// строит расписание для процентов
// только для simple расматриваем ручные графики, все остальные только по правилу
func buildInterest(deal Deal, ctx flowContext) ([]scheduleItem, string, error) {
	if deal.Discriminator == DiscSIMPLE {
		manual, err := parseManualDates(deal.ManualInterestPayment)
		if err != nil {
			return nil, "", fmt.Errorf("interest_payment: %w", err)
		}

		if len(manual) > 0 {
			dates, err := filterManual(manual, deal.MaturityDate, ctx.StartDate, false)
			if err != nil {
				return nil, "", fmt.Errorf("interest_payment: %w", err)
			}
			if !dates[len(dates)-1].Date.Equal(deal.MaturityDate) {
				dates = append(dates, scheduleItem{Date: deal.MaturityDate, Value: 0})
			}
			return dates, "", nil
		}
	}

	anchorDay, hasAnchor := getAnchor(deal.InterestPayDayNum, deal.InterestPayStart)
	//InterestPayStart если этого нет, то просто считаю, что правило не задано
	if deal.InterestPayPeriod != "" && hasAnchor && deal.InterestPayStart != nil {
		if deal.InterestPayPeriod == "ENDDATE" {
			return []scheduleItem{{ctx.StartDate, -1}}, "ENDDATE", nil
		}
		dates, err := buildSchedule(*deal.InterestPayStart, deal.MaturityDate, ctx.StartDate, deal.InterestPayPeriod, anchorDay)
		if err != nil {
			return nil, "", fmt.Errorf("interest_payment: %w", err)
		}
		if !dates[len(dates)-1].Date.Equal(deal.MaturityDate) {
			dates = append(dates, scheduleItem{Date: deal.MaturityDate, Value: 0})
		}
		return dates, deal.InterestPayPeriod, nil
	}
	return nil, "", nil // ничего не задано
}

// для поля repricing_dttm_rate_schedule_fixed
// если ставка FIXED, то на поля repricing_dttm_schedule и repricing_dttm_rate_schedule не обращаем внимания
// и при fixed итоговая ставка равна ставке, либо interest rate либо полученной ставке из этого расписания
// sched[0].Value = -1 если этой даты не было в расписании и она была добавлена программой
func buildFixedRepricing(maturity, start time.Time, repricingRateFixed string) ([]scheduleItem, error) {
	manual, err := parseManualSchedule(repricingRateFixed)
	if err != nil {
		return nil, fmt.Errorf("fixed_repricing: %w", err)
	}

	if len(manual) > 0 {
		dates, err := filterManual(manual, maturity, start, true)
		if err != nil {
			return nil, fmt.Errorf("fixed_repricing: %w", err)
		}
		return dates, nil
	}

	return nil, nil
}

// даты либо по правилу, либо по repricing_dttm_schedule, тогда ставки из cbkey
// либо если есть repricing_dttm_rate_schedule и даты, и ставки отсюда
func buildFloatRepricing(deal Deal, ctx flowContext, start time.Time) ([]scheduleItem, string, error) {
	manual, err := parseManualSchedule(deal.RepricingRateFloat)
	if err != nil {
		return nil, "", fmt.Errorf("float_repricing: %w", err)
	}
	if len(manual) > 0 {
		dates, err := filterManual(manual, deal.MaturityDate, start, true)
		if err != nil {
			return nil, "", fmt.Errorf("float_repricing: %w", err)
		}

		for i := range dates {
			if i == 0 && dates[i].Value == -1 { continue } //нужно потому что иногда программа добавляет в sched[0] несуществующую дату
			raw := deal.InterestRate + dates[i].Value // маржа + индекс
            dates[i].Value = clampRate(raw, deal.CapLimitInterestRate, deal.FloorLimitInterestRate)
		}

		return dates, "", nil
	}

	manual, err = parseManualDates(deal.RepricingDttmFloat)
	if err != nil {
		return nil, "", fmt.Errorf("float_repricing: %w", err)
	}
	if len(manual) > 0 {
		dates, err := filterManual(manual, deal.MaturityDate, start, true)
		if err != nil {
			return nil, "", fmt.Errorf("float_repricing: %w", err)
		}

		//дальше ищем в cbkey ставки которые актуальны на дату и заполняем value
		for i := range dates {
			if i == 0 && dates[i].Value == -1 { continue }
			idx, ok := ctx.Indices.rateAt(dates[i].Date)
			if !ok {
				break
			}
			raw := deal.InterestRate + idx   // маржа + индекс
            dates[i].Value = clampRate(raw, deal.CapLimitInterestRate, deal.FloorLimitInterestRate)
		}

		return dates, "", nil
	}

	anchorDay, hasAnchor := getAnchor(deal.RepricingRuleDayNum, deal.RepricingRuleStart)
	//RepricingRuleStart если этого нет, то просто считаю, что правило не задано
	if deal.RepricingRulePeriod != "" && hasAnchor && deal.RepricingRuleStart != nil {
		var dates []scheduleItem
		if deal.RepricingRulePeriod == "ENDDATE" {
			dates = []scheduleItem{{start, -1}, {deal.MaturityDate, 0}}
		} else {
			dates, err = buildSchedule(*deal.RepricingRuleStart, deal.MaturityDate, start, deal.RepricingRulePeriod, anchorDay)
			if err != nil {
				return nil, "", fmt.Errorf("float_repricing: %w", err)
			}
		}

		//дальше ищем в cbkey ставки которые актуальны на дату и заполняем value
		for i := range dates {
			if i == 0 && dates[i].Value == -1 { continue }
			idx, ok := ctx.Indices.rateAt(dates[i].Date)
			if !ok {
				break
			}
			raw := deal.InterestRate + idx   // маржа + индекс
            dates[i].Value = clampRate(raw, deal.CapLimitInterestRate, deal.FloorLimitInterestRate)
		}

		return dates, deal.RepricingRulePeriod, nil
	}

	return nil, "", nil //ничего не задано
}