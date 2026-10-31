package cashflow

import (
	"fmt"
	"net/http"
	"analytics/configs"
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
		for _, deal := range deals {
			flows := GenerateCashFlows(deal)
			allCashFlows = append(allCashFlows, flows...)
		}

		fmt.Printf("Всего сгенерировано строк CashFlow: %d\n", len(allCashFlows))
		
		for i, cf := range allCashFlows {
			if (i < 10 || i > len(allCashFlows) - 10) {
				fmt.Printf(
					"Load: %s | Src: %s | DealID: %d | Curr: %s | Start: %s | PayDate: %s | Days: %d | Bucket: %s | Type: %s | Prin: %.2f | Int: %.2f | RemBal: %.2f\n",
					cf.LoadDate.Format("2006-01-02 15:04:05"),
					cf.SourceCode,
					cf.DealID,
					cf.Currency,
					cf.StartDate.Format("02.01.2006"),
					cf.PaymentDate.Format("02.01.2006"),
					cf.DaysToPayment,
					cf.TimeBucket,
					cf.PaymentType,
					cf.PrincipalAmount,
					cf.InterestAmount,
					cf.RemainingBalance,
				)
			}
		}

		err = SaveToCSV(allCashFlows, "../history")
		if err != nil {
			fmt.Printf("Ошибка сохранения истории: %v\n", err)
		}

		// Отвечаем пользователю
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"status": "success", "deals_loaded": %d, "cashflows_generated": %d}`, len(deals), len(allCashFlows))))
	}
}