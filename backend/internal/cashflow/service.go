package cashflow

import (
	"fmt"
    "analytics/configs"
)

type CashFlowService struct {
    Config  *configs.Config
    Indices map[string]indexSeries
}

func NewCashFlowService(conf *configs.Config) *CashFlowService {
    return &CashFlowService{
        Config:  conf,
        Indices: loadIndices("./data"), // путь к папке с cbkey.txt (лучше релиазовать через config)
    }
}

func (s *CashFlowService) GenerateCashFlows(deal Deal) ([]CashFlow, error) {
	ctx, err := prepareContext(deal, s.Indices)
    if err != nil {
        return nil, &DealError{DealID: deal.DealID, Err: err}
    }

	switch deal.Discriminator {
    case DiscANN:
        res, err := generateAnnFlows(deal, ctx)
        if err != nil {
            return nil, &DealError{DealID: deal.DealID, Err: err}
        }
        return res, nil
    /*case DiscRGM:
        return generateDifferentiatedFlows(deal, ctx)
    case DiscSIMPLE:
        return generateSimpleFlows(deal, ctx)*/
    default:
        return nil, &DealError{
            DealID: deal.DealID,
            Err:    fmt.Errorf("unknown discriminator %q", deal.Discriminator),
        }
    }
}