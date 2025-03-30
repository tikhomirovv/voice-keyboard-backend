package pkg

import (
	"fmt"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

func NewValidator() (*validator.Validate, error) {
	vl := validator.New(validator.WithRequiredStructEnabled())
	return vl, nil
}

func NewValidatorTranslator(validate *validator.Validate) (ut.Translator, error) {
	enl := en.New()
	uni := ut.New(enl, enl)
	trans, found := uni.GetTranslator("en")
	if !found {
		return nil, fmt.Errorf("translator not found")
	}

	err := en_translations.RegisterDefaultTranslations(validate, trans)
	if err != nil {
		return nil, err
	}
	return trans, nil
}
