package cashflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"analytics/configs"
	"analytics/internal/valuation"

	"golang.org/x/sync/errgroup"
)

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

func (s *CashFlowService) ProcessPortfolio(ctx context.Context, file io.Reader, filename string) (*Result, error) {
	var (
		mu       sync.Mutex
		stats    Stats
		all      []CashFlow
	)

	dealsCh := make(chan Deal, s.Workers*2)
	flowsCh := make(chan []CashFlow, s.Workers*2)

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

				select {
				case flowsCh <- flows:
				case <-gctx.Done():
					return gctx.Err()
				}
			}
			return nil
		})
	}

	go func() {
		_ = g.Wait()
		close(flowsCh)
	}()

	for flows := range flowsCh {
		all = append(all, flows...)
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	if err := SaveToCSV(all, "./../history", filename); err != nil {
		fmt.Printf("WARN: save csv failed: %v\n", err)
	}

	// Вызываем Valuation
	testCurve := &valuation.FlatCurve{AnnualRate: 0.15}
	valEngine := valuation.NewEngine()

	var valFlows []valuation.Flow
	for _, cf := range all {
		valFlows = append(valFlows, cf)
	}

	portfolioResult, err := valEngine.EvaluatePortfolio(valFlows, testCurve)
	if err != nil {
		return nil, fmt.Errorf("valuation failed: %w", err)
	}

	return &Result{
		Stats:     stats,
		Cashflows: all,
		TotalPV:   portfolioResult.TotalPV,
	}, nil
}