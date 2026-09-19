package validator

import (
	"fmt"
	"strings"

	playground "github.com/go-playground/validator/v10"
)

var v = playground.New()

// FieldError galat validasi per field (siap dipetakan ke respons API).
type FieldError struct {
	Field   string
	Message string
}

// Errors kumpulan galat validasi.
type Errors struct {
	Fields []FieldError
}

func (e *Errors) Error() string {
	if len(e.Fields) == 0 {
		return "validasi gagal"
	}
	parts := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		parts = append(parts, f.Field+": "+f.Message)
	}
	return strings.Join(parts, "; ")
}

// Struct memvalidasi struct memakai tag `validate`.
func Struct(s any) error {
	err := v.Struct(s)
	if err == nil {
		return nil
	}
	ve, ok := err.(playground.ValidationErrors)
	if !ok {
		return err
	}
	out := &Errors{Fields: make([]FieldError, 0, len(ve))}
	for _, fe := range ve {
		out.Fields = append(out.Fields, FieldError{
			Field:   jsonFieldName(fe),
			Message: pesanTag(fe),
		})
	}
	return out
}

func jsonFieldName(fe playground.FieldError) string {
	// Prefer nama field; handler bisa map ke json tag bila perlu.
	return strings.ToLower(fe.Field())
}

func pesanTag(fe playground.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "wajib diisi"
	case "email":
		return "format email tidak valid"
	case "min":
		return fmt.Sprintf("minimal %s", fe.Param())
	case "max":
		return fmt.Sprintf("maksimal %s", fe.Param())
	case "oneof":
		return "nilai tidak diizinkan"
	default:
		return "tidak valid"
	}
}
