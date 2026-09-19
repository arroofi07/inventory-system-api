package dto

// Envelope membungkus respons sukses tunggal.
type Envelope[T any] struct {
	Data T `json:"data"`
}

// PageMeta metadata paginasi.
type PageMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// ListEnvelope membungkus respons daftar.
type ListEnvelope[T any] struct {
	Data []T      `json:"data"`
	Meta PageMeta `json:"meta"`
}

// ErrorBody isi galat API.
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details []FieldError   `json:"details,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// FieldError galat validasi per field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorResponse envelope galat.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}
