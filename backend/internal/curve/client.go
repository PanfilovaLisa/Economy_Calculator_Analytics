package curve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

type Client struct {
	Scenario string
	Points   []Point
}

func FetchCurve(baseURL, date string, targetScenario string) (*Client, error) {
	url := fmt.Sprintf("%s/curves/processed?date=%s", baseURL, date)

	// GET-запрос
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	// Парсим JSON
	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("json decode failed: %w", err)
	}

	// Ищем нужный снимок
	if len(apiResp.Snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found for date %s", date)
	}
	snapshot := apiResp.Snapshots[len(apiResp.Snapshots)-1]

	// Ищем нужный сценарий (Baseline или стрессовый)
	var targetCurve *Curve
	for _, c := range snapshot.Curves {
		if c.Scenario == targetScenario {
			targetCurve = &c
			break
		}
	}
	if targetCurve == nil {
		return nil, fmt.Errorf("scenario %s not found", targetScenario)
	}

	// Сортируем точки по возрастанию тенора
	sort.Slice(targetCurve.Points, func(i, j int) bool {
		return targetCurve.Points[i].Tenor < targetCurve.Points[j].Tenor
	})

	return &Client{
		Scenario: targetScenario,
		Points:   targetCurve.Points,
	}, nil
}

func (c *Client) GetScenarioName() string {
	return c.Scenario
}

// GetDiscountFactor находит DF для любого тенора с линейной интерполяцией
func (c *Client) GetDiscountFactor(targetTenor float64) (float64, error) {
	if len(c.Points) == 0 {
		return 0, fmt.Errorf("empty curve")
	}

	if targetTenor <= c.Points[0].Tenor {
		return c.Points[0].DiscountFactor, nil
	}

	last := c.Points[len(c.Points)-1]
	if targetTenor >= last.Tenor {
		return last.DiscountFactor, nil
	}

	for i := 0; i < len(c.Points)-1; i++ {
		p1 := c.Points[i]
		p2 := c.Points[i+1]

		if targetTenor >= p1.Tenor && targetTenor <= p2.Tenor {
			slope := (p2.DiscountFactor - p1.DiscountFactor) / (p2.Tenor - p1.Tenor)
			df := p1.DiscountFactor + (targetTenor-p1.Tenor)*slope
			return df, nil
		}
	}

	return 0, fmt.Errorf("interpolation failed for tenor %f", targetTenor)
}
