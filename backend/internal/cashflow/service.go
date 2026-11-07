package cashflow

import (
	"math"
    "time"
    "analytics/configs"
)

type CashFlowService struct {
    Config *configs.Config
}

func NewCashFlowService(conf *configs.Config) *CashFlowService {
    return &CashFlowService{Config: conf}
}

func GenerateCashFlows(deal Deal) []CashFlow {
	ctx, ok := prepareContext(deal)
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