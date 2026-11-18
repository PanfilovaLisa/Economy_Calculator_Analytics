package cashflow

import (
	"fmt"
	"net/http"
	"analytics/configs"
	"errors"
	"log"
)

type CashFlowHandlerDeps struct {
	CashFlowService *CashFlowService
	Config          *configs.Config
}

type CashFlowHandler struct {
	CashFlowService *CashFlowService
	Config          *configs.Config
}

func NewCashFlowHandler(router *http.ServeMux, deps CashFlowHandlerDeps) {
	handler := &CashFlowHandler{
		CashFlowService: deps.CashFlowService,
		Config:          deps.Config,
	}

	router.HandleFunc("POST /calculate", handler.Calculate())
}

func (handler *CashFlowHandler) Calculate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, "Ошибка при чтении формы", http.StatusBadRequest)
			return
		}

		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Файл не найден в запросе", http.StatusBadRequest)
			return
		}
		defer file.Close()

		fmt.Printf("Получен файл: %s\n", fileHeader.Filename)

		deals, err := ReadDeals(file)
		if err != nil {
			http.Error(w, "Ошибка парсинга CSV: "+err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Printf("Успешно прочитано сделок: %d\n", len(deals))

		// Генерируем денежные потоки
		var allCashFlows []CashFlow
		var dealErrs []*DealError

		for _, deal := range deals {
			flows, err := handler.CashFlowService.GenerateCashFlows(deal)
			if err != nil {
				var de *DealError
				if errors.As(err, &de) {
					dealErrs = append(dealErrs, de)
					log.Printf("deal %d skipped: %v", de.DealID, de.Err)
					continue
				}
				http.Error(w, "internal: "+err.Error(), http.StatusInternalServerError)
				return
			}
			allCashFlows = append(allCashFlows, flows...)
		}

		fmt.Printf("Всего сгенерировано строк CashFlow: %d\n", len(allCashFlows))

		err = SaveToCSV(allCashFlows, "../history", fileHeader.Filename)
		if err != nil {
			fmt.Printf("Ошибка сохранения истории: %v\n", err)
		}

		// Отвечаем пользователю
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(
			`{"status": "success", "deals_loaded": %d, "cashflows_generated": %d, "errors": %d}`,
			len(deals), len(allCashFlows), len(dealErrs),
		)))
	}
}