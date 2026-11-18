package cashflow

import (
	"time"
)

type EventType uint8

const (
	EvPrincipal EventType = iota
	EvInterest
	EvRepricing
)

type event struct {
	Date  time.Time
	Type  EventType
	Value float64
}

type eventIter struct {
	principal []scheduleItem
	interest  []scheduleItem
	repricing []scheduleItem
	i, j, k   int
}

func newEventIter(s schedules, rType string) *eventIter {
	var repricing []scheduleItem
	switch rType {
	case RateFixed:
		repricing = s.FixedSteps
	case RateFloat:
		repricing = s.Repricing
	}
	return &eventIter{
		principal: s.Principal,
		interest:  s.Interest,
		repricing: repricing,
		i:         0, //principal
		j:         0, //interest
		k:         0, //repricing
	}
}

func (it *eventIter) Next() (event, bool) {
	if it.i >= len(it.principal) &&
		it.j >= len(it.interest) &&
		it.k >= len(it.repricing) {
		return event{}, false
	}

	var res event
	typeRes := -1 //0 = principal, 1 = interest, 2 = repricing

	//если в одну дату происходит несколько событий, то сначала repricing -> interest -> principal
	if it.k < len(it.repricing) {
		res = event{Date: it.repricing[it.k].Date, Type: EvRepricing, Value: it.repricing[it.k].Value}
		typeRes = 2
	}

	if it.j < len(it.interest) {
		cand := event{Date: it.interest[it.j].Date, Type: EvInterest, Value: it.interest[it.j].Value}
		if typeRes == -1 || better(cand, res) {
			res, typeRes = cand, 1
		}
	}

	if it.i < len(it.principal) {
		cand := event{Date: it.principal[it.i].Date, Type: EvPrincipal, Value: it.principal[it.i].Value}
		if typeRes == -1 || better(cand, res) {
			res, typeRes = cand, 0
		}
	}

	switch typeRes {
	case 0:
		it.i++
	case 1:
		it.j++
	case 2:
		it.k++
	}

	return res, true
}

func better(a, b event) bool {
	if !a.Date.Equal(b.Date) {
		return a.Date.Before(b.Date)
	}
	return a.Type > b.Type
}