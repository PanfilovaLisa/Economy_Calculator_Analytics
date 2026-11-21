package cashflow

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"analytics/configs"
	"analytics/pkg/response"
	"golang.org/x/sync/errgroup"
)

type CashFlowHandlerDeps struct {
	CashFlowService *CashFlowService
	Config          *configs.Config
	Logger          *slog.Logger // опционально; nil → slog.Default()
}

type CashFlowHandler struct {
	svc       *CashFlowService
	conf      *configs.Config
	log       *slog.Logger
	maxUpload int64
	workers   int
}

func NewCashFlowHandler(router *http.ServeMux, deps CashFlowHandlerDeps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	h := &CashFlowHandler{
		svc:       deps.CashFlowService,
		conf:      deps.Config,
		log:       logger,
		maxUpload: 32 << 20, // 32 MB
		workers:   4,
	}
	if deps.Config != nil {
		if deps.Config.Server.MaxUploadBytes > 0 {
			h.maxUpload = deps.Config.Server.MaxUploadBytes
		}
		if deps.Config.Server.Workers > 0 {
			h.workers = deps.Config.Server.Workers
		}
	}

	router.HandleFunc("POST /calculate", h.Calculate())
}

type calcResponse struct {
	Status             string     `json:"status"`
	DealsLoaded        int        `json:"deals_loaded"`
	CashflowsGenerated int        `json:"cashflows_generated"`
	Errors             int        `json:"errors"`
	RowErrors          int        `json:"row_errors"`
	FirstDealError     string     `json:"first_deal_error,omitempty"`
	FirstRowError      string     `json:"first_row_error,omitempty"`
	Cashflows          []CashFlow `json:"cashflows"`
}

func (h *CashFlowHandler) Calculate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// A6: ограничение размера тела до чтения
		r.Body = http.MaxBytesReader(w, r.Body, h.maxUpload)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			// MaxBytesError → 413; прочее → 400
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				response.Json(w, map[string]string{"error": "file too large"}, http.StatusRequestEntityTooLarge)
				return
			}
			response.Json(w, map[string]string{"error": "bad form: " + err.Error()}, http.StatusBadRequest)
			return
		}
		// B10: чистим временные файлы multipart
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}

		file, fh, err := r.FormFile("file")
		if err != nil {
			response.Json(w, map[string]string{"error": "file required (field 'file')"}, http.StatusBadRequest)
			return
		}
		defer file.Close()

		h.log.Info("calculate: start", "file", fh.Filename, "size", fh.Size)

		var (
			mu        sync.Mutex
			loaded    int           // A4: считаем в producer/workers по факту передачи в расчёт
			dealErrs  []*DealError
			rowErrsN  int
			firstRow  string
			all       []CashFlow    // осознанно: JSON в Body требует материализации
		)

		dealsCh := make(chan Deal, h.workers*2)
		flowsCh := make(chan []CashFlow, h.workers*2)

		g, gctx := errgroup.WithContext(ctx)

		// producer
		g.Go(func() error {
			defer close(dealsCh)
			rowErrs, err := StreamDeals(gctx, file, dealsCh, 1000)
			mu.Lock()
			rowErrsN = len(rowErrs)
			if len(rowErrs) > 0 {
				firstRow = rowErrs[0].Error()
			}
			mu.Unlock()
			return err
		})

		// workers
		for i := 0; i < h.workers; i++ {
			g.Go(func() error {
				for d := range dealsCh {
					mu.Lock()
					loaded++
					mu.Unlock()

					flows, err := h.svc.GenerateCashFlows(d)
					if err != nil {
						var de *DealError
						if errors.As(err, &de) {
							mu.Lock()
							dealErrs = append(dealErrs, de)
							mu.Unlock()
							h.log.Warn("deal skipped", "deal_id", de.DealID, "err", de.Err)
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

		// closer: ждём producer+workers, закрываем flowsCh
		go func() {
			_ = g.Wait()
			close(flowsCh)
		}()

		// consumer: собираем flows
		for flows := range flowsCh {
			mu.Lock()
			all = append(all, flows...)
			mu.Unlock()
		}

		// дожидаемся завершения и проверяем ошибки
		if err := g.Wait(); err != nil {
			if errors.Is(err, context.Canceled) {
				response.Json(w, map[string]string{"error": "request canceled"}, http.StatusRequestTimeout)
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				response.Json(w, map[string]string{"error": "deadline exceeded"}, http.StatusRequestTimeout)
				return
			}
			response.Json(w, map[string]string{"error": err.Error()}, http.StatusInternalServerError)
			return
		}

		mu.Lock()
		errs := len(dealErrs)
		firstDeal := ""
		if errs > 0 {
			firstDeal = dealErrs[0].Error()
		}
		mu.Unlock()

		h.log.Info("calculate: done",
			"file", fh.Filename,
			"loaded", loaded,
			"generated", len(all),
			"errors", errs,
			"row_errors", rowErrsN,
		)

		response.Json(w, calcResponse{
			Status:             "success",
			DealsLoaded:        loaded,
			CashflowsGenerated: len(all),
			Errors:             errs,
			RowErrors:          rowErrsN,
			FirstDealError:     firstDeal,
			FirstRowError:      firstRow,
			Cashflows:          all,
		}, http.StatusOK)
	}
}