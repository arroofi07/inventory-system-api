package dto

// ListQuery parameter daftar seragam (paginasi, sort, search).
// Dipakai semua modul master sebagai pola SB-01.
type ListQuery struct {
	Page    int    `form:"page" validate:"omitempty,min=1"`
	PerPage int    `form:"per_page" validate:"omitempty,min=1,max=100"`
	Sort    string `form:"sort"`
	Q       string `form:"q"`
}

// Normalize mengisi default page/per_page.
func (q *ListQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 {
		q.PerPage = 20
	}
	if q.PerPage > 100 {
		q.PerPage = 100
	}
}

func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.PerPage
}

// NewPageMeta menghitung total_pages dari total baris.
func NewPageMeta(page, perPage int, total int64) PageMeta {
	tp := 0
	if perPage > 0 {
		tp = int((total + int64(perPage) - 1) / int64(perPage))
	}
	if tp == 0 && total == 0 {
		tp = 0
	}
	return PageMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: tp,
	}
}
