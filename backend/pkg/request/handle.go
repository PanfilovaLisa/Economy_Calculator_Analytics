package request

import (
	"net/http"
	"analytics/pkg/responce"
)

func HandleBody[T any](w *http.ResponseWriter, r *http.Request) (*T, error) {

	body, err := Decode[T](r.Body)
	if err != nil {
		responce.Json(*w, err.Error(), 400)
		return nil, err
	}

	err = IsValidate(body)
	if err != nil {
		responce.Json(*w, err.Error(), 400)
		return nil, err
	}

	return &body, nil
}
