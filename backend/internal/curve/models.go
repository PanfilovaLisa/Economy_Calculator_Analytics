package curve

type APIResponse struct {
	DateFrom  string     `json:"date_from"`
	DateTo    string     `json:"date_to"`
	Missing   []string   `json:"missing_dates"`
	Snapshots []Snapshot `json:"snapshots"`
}

type Snapshot struct {
	CurveDate string  `json:"curve_date"`
	QuoteTime string  `json:"quote_time"`
	Curves    []Curve `json:"curves"`
}

type Curve struct {
	Scenario        string  `json:"scenario"`
	ScenarioVersion *string `json:"scenario_version"`
	Points          []Point `json:"points"`
}

type Point struct {
	Tenor          float64 `json:"tenor,string"`
	YieldRate      float64 `json:"yield_rate,string"`
	DiscountFactor float64 `json:"discount_factor,string"`
	IsInterpolated bool    `json:"is_interpolated"`
}