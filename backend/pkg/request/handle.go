package request

import (
	"net/http"

	"analytics/pkg/response"
)

func HandleBody[T any](w http.ResponseWriter, r *http.Request) (*T, error) {
	body, err := Decode[T](r.Body)
	if err != nil {
		response.Json(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
		return nil, err
	}
	if err := IsValidate(body); err != nil {
		response.Json(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
		return nil, err
	}
	return &body, nil
}