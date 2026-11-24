package valuation

import (
	"math"
	"testing"
	"time"
)

type MockCurve struct {
	rates map[float64]float64 
}

func (c *MockCurve) GetScenarioName() string { return "Test" }

func (c *MockCurve) GetDiscountFactor(tenor float64) (float64, error) {
	bestDiff := 1.0
	var bestRate float64
	for t, r := range c.rates {
		diff := math.Abs(t - tenor)
		if diff < bestDiff {
			bestDiff = diff
			bestRate = r
		}
	}
	return math.Pow(1+bestRate, -tenor), nil
}

type MockFlow struct {
	DealID   int
	Report   time.Time
	PayDate  time.Time
	Amount   float64
}

func (f *MockFlow) GetDealID() int             { return f.DealID }
func (f *MockFlow) GetPaymentDate() time.Time  { return f.PayDate }
func (f *MockFlow) GetPaymentAmount() float64  { return f.Amount }
func (f *MockFlow) GetReportDate() time.Time   { return f.Report }

func TestValuationEngine(t *testing.T) {
	reportDate := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	
	curve := &MockCurve{
		rates: map[float64]float64{
			0.875: 0.1363,
		},
	}

	engine := NewEngine()

	flow1 := &MockFlow{
		DealID:  1,
		Report:  reportDate,
		PayDate: reportDate.AddDate(0, 0, 319),
		Amount:  100,
	}

	pv1, err := engine.CalculatePV([]Flow{flow1}, curve)
	if err != nil {
		t.Fatalf("CalculatePV failed: %v", err)
	}


	expectedPV1 := 89.4218
	if math.Abs(pv1-expectedPV1) > 0.05 {
		t.Errorf("Test 1: expected PV %.4f, got %.4f", expectedPV1, pv1)
	}
}


func TestExcelCompliance(t *testing.T) {
	reportDate := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	curve := &ExcelTestCurve{}
	engine := NewEngine()

	flow1y := &MockFlow{
		Report:  reportDate,
		PayDate: reportDate.AddDate(0, 0, 319),
		Amount:  100,
	}
	pv1y, _ := engine.CalculatePV([]Flow{flow1y}, curve)
	if math.Abs(pv1y - 89.4218) > 0.05 {
		t.Errorf("1y mismatch: expected 89.4218, got %.4f", pv1y)
	}

	flow2y := &MockFlow{
		Report:  reportDate,
		PayDate: reportDate.AddDate(0, 0, 638),
		Amount:  100,
	}
	pv2y, _ := engine.CalculatePV([]Flow{flow2y}, curve)
	if math.Abs(pv2y - 78.9627) > 0.05 {
		t.Errorf("2y mismatch: expected 78.9627, got %.4f", pv2y)
	}
}