package cashflow

import (
    "analytics/configs"
)

type CashFlowService struct {
    Config  *configs.Config
    Indices map[string]indexSeries
}

func NewCashFlowService(conf *configs.Config) *CashFlowService {
    return &CashFlowService{
        Config:  conf,
        Indices: loadIndices("./data"),   // путь к папке с cbkey.txt
    }
}

// GenerateCashFlows — метод сервиса, чтобы иметь доступ к Indices.
func (s *CashFlowService) GenerateCashFlows(deal Deal) []CashFlow {
    ctx, ok := prepareContext(deal, s.Indices)
    if !ok {
        return nil
    }

    switch deal.Discriminator {
    case DiscANN:
        return generateAnnuityFlows(deal, ctx)
    case DiscRGM:
        return generateDifferentiatedFlows(deal, ctx)
    case DiscSIMPLE:
        return generateSimpleFlows(deal, ctx)
    default:
        return nil
    }
}