package request

import (
	"github.com/go-playground/validator/v10"
)

func IsValidate[T any](payload T) error {
	validate := validator.New()
	err := validate.Struct(payload)
	return err
}
