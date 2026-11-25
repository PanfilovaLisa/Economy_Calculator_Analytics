package cashflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"analytics/configs"

	"golang.org/x/sync/errgroup"
)

type FlowHandler func(deal Deal, flows []CashFlow) error

type CashFlowService struct {
	Config  *configs.Config
	Indices map[string]indexSeries
	Workers int
}

func NewCashFlowService(conf *configs.Config) *CashFlowService {
	workers := 4
	if conf != nil && conf.Server.Workers > 0 {
		workers = conf.Server.Workers
	}
	return &CashFlowService{
		Config:  conf,
		Indices: loadIndices("./data"),
		Workers: workers,
	}
}

func (s *CashFlowService) GenerateCashFlows(deal Deal) ([]CashFlow, error) {
	ctx, err := prepareContext(deal, s.Indices)
	if err != nil {
		return nil, &DealError{DealID: deal.DealID, Err: err}
	}

	switch deal.Discriminator {
	case DiscANN:
		return generateAnnFlows(deal, ctx)
	case DiscRGM:
		return generateRGMFlows(deal, ctx)
	case DiscSIMPLE:
		return generateSimpleFlows(deal, ctx)
	default:
		return nil, &DealError{
			DealID: deal.DealID,
			Err:    fmt.Errorf("unknown discriminator %q", deal.Discriminator),
		}
	}
}

type Stats struct {
	Loaded        int
	DealErrors    []*DealError
	RowErrors     int
	FirstRowError string
}

type Result struct {
	Stats
	Cashflows []CashFlow
	TotalPV   float64
}

func (s *CashFlowService) ProcessPortfolio(ctx context.Context, file io.Reader, onFlows FlowHandler) (Stats, error) {
	var (
		mu    sync.Mutex
		stats Stats
	)

	dealsCh := make(chan Deal, s.Workers*2)
	g, gctx := errgroup.WithContext(ctx)

	// Producer
	g.Go(func() error {
		defer close(dealsCh)
		rowErrs, err := StreamDeals(gctx, file, dealsCh, 1000)
		mu.Lock()
		stats.RowErrors = len(rowErrs)
		if len(rowErrs) > 0 {
			stats.FirstRowError = rowErrs[0].Error()
		}
		mu.Unlock()
		return err
	})

	// Workers
	for i := 0; i < s.Workers; i++ {
		g.Go(func() error {
			for d := range dealsCh {
				mu.Lock()
				stats.Loaded++
				mu.Unlock()

				flows, err := s.GenerateCashFlows(d)
				if err != nil {
					var de *DealError
					if errors.As(err, &de) {
						mu.Lock()
						stats.DealErrors = append(stats.DealErrors, de)
						mu.Unlock()
						continue
					}
					return err
				}

				if onFlows != nil {
					if err := onFlows(d, flows); err != nil {
						var de *DealError
						if errors.As(err, &de) {
							mu.Lock()
							stats.DealErrors = append(stats.DealErrors, de)
							mu.Unlock()
							continue
						}
						return fmt.Errorf("deal %d handler failed: %w", d.DealID, err)
					}
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return stats, err
	}

	return stats, nil
}