package main

import (
	"analytics/configs"
	"analytics/internal/calc"
	"fmt"
	"log"
	"net/http"
)

type Responce struct {
	StatusCode int
}

func main() {
	conf := configs.LoadConfig()
	if conf == nil {
		log.Fatalln("config is nil")
		return
	}
	// db := db.NewDb(conf) // db
	// if conf == nil {
	// 	log.Fatalln("db is nil")
	// 	return
	// }
	router := http.NewServeMux()

	// Repositories

	// Services
	calcService := calc.NewCalcService(calc.CalcServiceDeps{
		Config: conf,
	})

	// Handlers
	calc.NewCalcHandler(router, calc.CalcHandlerDeps{
		CalcService: calcService,
		Config:      conf,
	})

	server := http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	fmt.Println("Сервер запущен на порте 8082")
	server.ListenAndServe()
}
