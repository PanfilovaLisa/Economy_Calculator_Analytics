package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"analytics/configs"
	"analytics/internal/cashflow"
	"analytics/internal/valuation"
	"analytics/pkg/response"
)

type CashFlowHandlerDeps struct {
	CashFlowService *cashflow.CashFlowService
	ValuationEngine *valuation.Engine
	Config          *configs.Config
	Logger          *slog.Logger
}

type CashFlowHandler struct {
	svc       *cashflow.CashFlowService
	valEngine *valuation.Engine
	conf      *configs.Config
	log       *slog.Logger
	maxUpload int64
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
	}
	if deps.Config != nil && deps.Config.Server.MaxUploadBytes > 0 {
		h.maxUpload = deps.Config.Server.MaxUploadBytes
	}

	router.HandleFunc("POST /calculate", h.Calculate())
}

type calcResponse struct {
	Status             string                `json:"status"`
	DealsLoaded        int                   `json:"deals_loaded"`
	CashflowsGenerated int                   `json:"cashflows_generated"`
	Errors             int                   `json:"errors"`
	RowErrors          int                   `json:"row_errors"`
	FirstDealError     string                `json:"first_deal_error,omitempty"`
	FirstRowError      string                `json:"first_row_error,omitempty"`
	TotalPV            float64               `json:"total_pv"`
	Cashflows          []cashflow.CashFlow   `json:"cashflows"`
}

func (h *CashFlowHandler) Calculate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Валидация HTTP
		r.Body = http.MaxBytesReader(w, r.Body, h.maxUpload)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				response.Json(w, map[string]string{"error": "file too large"}, http.StatusRequestEntityTooLarge)
				return
			}
			response.Json(w, map[string]string{"error": "bad form: " + err.Error()}, http.StatusBadRequest)
			return
		}
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
			mu       sync.Mutex
			allFlows []cashflow.CashFlow
			totalPV  float64
		)

		testCurve := &valuation.FlatCurve{AnnualRate: 0.15}

		onFlows := func(deal cashflow.Deal, flows []cashflow.CashFlow) error {
			valFlows := make([]valuation.Flow, len(flows))
			for i, cf := range flows {
				valFlows[i] = cf
			}

			dealPV, err := h.valEngine.CalculatePV(valFlows, testCurve)
			if err != nil {
				return &cashflow.DealError{DealID: deal.DealID, Err: err}
			}

			mu.Lock()
			totalPV += dealPV
			allFlows = append(allFlows, flows...)
			mu.Unlock()

			return nil
		}

		stats, err := h.svc.ProcessPortfolio(r.Context(), file, onFlows)
		if err != nil {
			h.log.Error("process failed", "err", err)
			response.Json(w, map[string]string{"error": "internal error"}, http.StatusInternalServerError)
			return
		}


		h.log.Info("calculate: done",
			"file", fh.Filename,
			"loaded", stats.Loaded,
			"generated", len(allFlows),
			"errors", len(stats.DealErrors),
			"total_pv", totalPV,
		)

		firstDeal := ""
		if len(stats.DealErrors) > 0 {
			firstDeal = stats.DealErrors[0].Error()
		}

		response.Json(w, calcResponse{
			Status:             "success",
			DealsLoaded:        stats.Loaded,
			CashflowsGenerated: len(allFlows),
			Errors:             len(stats.DealErrors),
			RowErrors:          stats.RowErrors,
			FirstDealError:     firstDeal,
			FirstRowError:      stats.FirstRowError,
			TotalPV:            totalPV,
			Cashflows:          allFlows,
		}, http.StatusOK)
	}
}