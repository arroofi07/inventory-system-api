package validator_test

import (
	"errors"
	"testing"

	appvalidator "app/internal/pkg/validator"
)

func TestStructRequired(t *testing.T) {
	type req struct {
		Nama string `validate:"required"`
	}
	err := appvalidator.Struct(req{})
	var ve *appvalidator.Errors
	if !errors.As(err, &ve) || len(ve.Fields) == 0 {
		t.Fatalf("want validation errors, got %v", err)
	}
	if err := appvalidator.Struct(req{Nama: "ok"}); err != nil {
		t.Fatal(err)
	}
}
