package cashflow

import (
	"time"
)

type EventType uint8

const (
	EvPrincipal EventType = iota
	SubEvInterest
	SubEvRepricing
	EvInterest
	EvRepricing
)

type event struct {
	Date  time.Time
	Type  EventType
	Value float64
}

type eventIter struct {
	principal    []scheduleItem
	subInterest  []scheduleItem
	subRepricing []scheduleItem
	interest     []scheduleItem
	repricing    []scheduleItem
	i, j, k, m, n  int
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
		subInterest: s.SubInterest,
		subRepricing: s.SubRepricing,
		interest:  s.Interest,
		repricing: repricing,
		i:         0, //principal
		m:         0, //sub interest
		n:         0, //sub repricing
		j:         0, //interest
		k:         0, //repricing
	}
}

func (it *eventIter) Next() (event, bool) {
	if it.i >= len(it.principal) &&
		it.j >= len(it.interest) &&
		it.k >= len(it.repricing) &&
		it.m >= len(it.subInterest) &&
		it.n >= len(it.subRepricing) {
		return event{}, false
	}

	var res event
	typeRes := -1 //0 = principal, 1 = SubEvInterest, 2 = SubEvRepricing, 3 = interest, 4 = repricing

	//если в одну дату происходит несколько событий, то сначала repricing -> interest -> SubRepricing -> SubInterest -> principal
	if it.k < len(it.repricing) {
		res = event{Date: it.repricing[it.k].Date, Type: EvRepricing, Value: it.repricing[it.k].Value}
		typeRes = int(EvRepricing)
	}

	if it.j < len(it.interest) {
		cand := event{Date: it.interest[it.j].Date, Type: EvInterest, Value: it.interest[it.j].Value}
		if typeRes == -1 || better(cand, res) {
			res, typeRes = cand, int(EvInterest)
		}
	}

	if it.n < len(it.subRepricing) {
		cand := event{Date: it.subRepricing[it.n].Date, Type: SubEvRepricing, Value: it.subRepricing[it.n].Value}
		if typeRes == -1 || better(cand, res) {
			res, typeRes = cand, int(SubEvRepricing)
		}
	}

	if it.m < len(it.subInterest) {
		cand := event{Date: it.subInterest[it.m].Date, Type: SubEvInterest, Value: it.subInterest[it.m].Value}
		if typeRes == -1 || better(cand, res) {
			res, typeRes = cand, int(SubEvInterest)
		}
	}


	if it.i < len(it.principal) {
		cand := event{Date: it.principal[it.i].Date, Type: EvPrincipal, Value: it.principal[it.i].Value}
		if typeRes == -1 || better(cand, res) {
			res, typeRes = cand, int(EvPrincipal)
		}
	}

	switch typeRes {
	case 0:
		it.i++
	case 1:
		it.m++
	case 2:
		it.n++
	case 3:
		it.j++
	case 4:
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