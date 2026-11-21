package request

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validateOnce sync.Once
	validate     *validator.Validate
)

func getValidator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New()
	})
	return validate
}

func IsValidate[T any](payload T) error {
	return getValidator().Struct(payload)
}