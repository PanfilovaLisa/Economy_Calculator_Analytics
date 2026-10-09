package calc

import (
	"net/http"
	"analytics/configs"
)

type CalcHandlerDeps struct {
	CalcService *CalcService
	Config *configs.Config
}

type CalcHandler struct {
	CalcService *CalcService
	Config *configs.Config
}


func NewCalcHandler(router *http.ServeMux, deps CalcHandlerDeps) {
	handler := &CalcHandler{}

	router.HandleFunc("POST /calculate", handler.Calculate())

}
func (handler *CalcHandler) Calculate() http.HandlerFunc { // POST
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(2024)
		r.FormFile("file")
	}
}
