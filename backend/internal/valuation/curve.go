package valuation

import "math"

// CurveProvider — интерфейс для получения коэффициента дисконтирования.
// Принимает tenor (срок в годах).
type CurveProvider interface {
	GetDiscountFactor(tenor float64) (float64, error)
	GetScenarioName() string
}

type FlatCurve struct {
	AnnualRate float64
	Scenario   string
}

func (c *FlatCurve) GetScenarioName() string {
	if c.Scenario == "" {
		return "Baseline"
	}
	return c.Scenario
}

func (c *FlatCurve) GetDiscountFactor(tenor float64) (float64, error) {
	return math.Pow(1+c.AnnualRate, -tenor), nil
}


type ExcelTestCurve struct{}

func (c *ExcelTestCurve) GetScenarioName() string { return "Baseline_Excel" }

func (c *ExcelTestCurve) GetDiscountFactor(tenor float64) (float64, error) {
	rates := []struct {
		T float64
		R float64
	}{
		{0.0028, 0.1327}, // 1d
		{0.0417, 0.1327}, // 1m
		{0.1667, 0.1327}, // 3m
		{0.875,  0.1363}, // 1y
		{1.75,   0.1445}, // 2y
		{2.5,    0.1504}, // 3y
		{25.0,   0.1634}, // 30y
	}

	for i := 0; i < len(rates)-1; i++ {
		if tenor >= rates[i].T && tenor <= rates[i+1].T {
			w := (tenor - rates[i].T) / (rates[i+1].T - rates[i].T)
			r := rates[i].R + w*(rates[i+1].R-rates[i].R)
			return math.Pow(1+r, -tenor), nil
		}
	}

	if tenor < rates[0].T {
		return math.Pow(1+rates[0].R, -tenor), nil
	}
	
	last := rates[len(rates)-1]
	return math.Pow(1+last.R, -tenor), nil
}