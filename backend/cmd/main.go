package main

import (
	"analytics/configs"
	"analytics/internal/cashflow"
	"fmt"
	"net/http"
)

func main() {
	conf := configs.LoadConfig()
	if conf == nil {
		panic("config is nil")
	}

	router := http.NewServeMux()

	cashFlowService := cashflow.NewCashFlowService(conf)

	cashflow.NewCashFlowHandler(router, cashflow.CashFlowHandlerDeps{
		CashFlowService: cashFlowService,
		Config:          conf,
	})

	server := http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	fmt.Println("Сервер запущен на порте 8082")
	
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		fmt.Printf("ОШИБКА ЗАПУСКА СЕРВЕРА: %v\n", err)
	}
}