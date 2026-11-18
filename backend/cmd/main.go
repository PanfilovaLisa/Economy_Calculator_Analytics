package main

import (
	"log"
	"net/http"
	"time"

	"analytics/configs"
	"analytics/internal/cashflow"
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

	server := &http.Server{
		Addr:              ":8082",
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Println("Сервер запущен на порте 8082")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ОШИБКА ЗАПУСКА СЕРВЕРА: %v", err)
	}
}