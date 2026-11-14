package cashflow

import (
	"math"
	"fmt"
	"time"
)

type flowContext struct {
	StartDate             time.Time
	Remaining             float64
	DayConvention         string
	IsDeposit             bool
	RateAt                func(time.Time) float64
	schedules             schedules
	Indices               map[string]indexSeries
}

func (ctx flowContext) Signed(x float64) float64 {
	if ctx.IsDeposit {
		return -x
	}
	return x
}

func prepareContext(deal Deal, indices map[string]indexSeries) (flowContext, error) {
	if deal.CurrentPrincipalAmt == 0 {
		return flowContext{}, fmt.Errorf("zero current principal")
	}
	if deal.ReportDate.IsZero() {
		return flowContext{}, fmt.Errorf("empty report date")
	}
	if deal.MaturityDate.IsZero() {
		return flowContext{}, fmt.Errorf("empty maturity date")
	}
	if deal.DealDate.IsZero() {
		return flowContext{}, fmt.Errorf("empty deal date")
	}
	if !deal.ReportDate.Before(deal.MaturityDate) {
		return flowContext{}, fmt.Errorf("report date %s >= maturity %s",
			deal.ReportDate.Format("2006-01-02"),
			deal.MaturityDate.Format("2006-01-02"))
	}

	isDeposit := deal.CurrentPrincipalAmt < 0
	remaining := math.Abs(deal.CurrentPrincipalAmt)

	start := deal.ReportDate
	if deal.DealDate.After(start) {
		start = deal.DealDate
	}
	if !start.Before(deal.MaturityDate) {
		return flowContext{}, fmt.Errorf("start date %s >= maturity %s",
			start.Format("2006-01-02"),
			deal.MaturityDate.Format("2006-01-02"))
	}
	conv := normalizeDayConvention(deal.DayConvention)

	ctx := flowContext{
		StartDate:             start,
		Remaining:             remaining,
		DayConvention:         conv,
		IsDeposit:             isDeposit,
		Indices:               indices,
	}

	sched, err := buildSchedules(deal, ctx)
	if err != nil {
		return flowContext{}, nil
	}

	ctx.RateAt = makeRateAt(deal, sched, ctx.Indices)
	ctx.schedules = sched

	return ctx, nil
}

func normalizeDayConvention(s string) string {
	switch s {
	case "ACT/365", "ACT/ACT", "ACT/ACT ISDA":
		return s
	default:
		return "ACT/365"
	}
}