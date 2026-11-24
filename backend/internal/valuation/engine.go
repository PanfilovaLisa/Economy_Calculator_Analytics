package valuation

import (
	"fmt"
	"time"
)

type Flow interface {
	GetDealID() int
	GetPaymentDate() time.Time
	GetPaymentAmount() float64
	GetReportDate() time.Time
}

type Engine struct{}

// PortfolioResult - результат оценки всего портфеля
type PortfolioResult struct {
	TotalPV   float64
	DealsCount int
	DealPVs   map[int]float64 // PV по каждой сделке (deal_id -> PV)
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) CalculatePV(flows []Flow, curve CurveProvider) (float64, error) {
	if len(flows) == 0 {
		return 0, nil
	}

	var pv float64
	reportDate := flows[0].GetReportDate()

	for _, flow := range flows {
		amount := flow.GetPaymentAmount()
		
		if amount == 0 {
			continue
		}

		payDate := flow.GetPaymentDate()
		days := payDate.Sub(reportDate).Hours() / 24.0
		tenor := days / 365.0

		if tenor < 0 {
			continue 
		}

		df, err := curve.GetDiscountFactor(tenor)
		if err != nil {
			return 0, fmt.Errorf("failed to get discount factor: %w", err)
		}
		
		if df == 0 {
			continue
		}

		pv += amount * df
	}

	return pv, nil
}



// EvaluatePortfolio группирует потоки по сделкам и считает PV для каждой
func (e *Engine) EvaluatePortfolio(flows []Flow, curve CurveProvider) (PortfolioResult, error) {
	result := PortfolioResult{
		DealPVs: make(map[int]float64),
	}

	if len(flows) == 0 {
		return result, nil
	}

	// 1. Группируем потоки по DealID
	flowsByDeal := make(map[int][]Flow)
	for _, f := range flows {
		flowsByDeal[f.GetDealID()] = append(flowsByDeal[f.GetDealID()], f)
	}

	// 2. Считаем PV для каждой сделки
	for dealID, dealFlows := range flowsByDeal {
		pv, err := e.CalculatePV(dealFlows, curve)
		if err != nil {
			return result, fmt.Errorf("deal %d: %w", dealID, err)
		}
		result.DealPVs[dealID] = pv
		result.TotalPV += pv
	}

	result.DealsCount = len(flowsByDeal)
	return result, nil
}