package query

import (
	"strings"

	"gorm.io/gorm"
)

// TerapkanSort menerjemahkan parameter sort ke ORDER BY dengan daftar putih kolom.
// Prefix "-" = DESC. Kolom di luar whitelist diganti bawaan.
func TerapkanSort(db *gorm.DB, sort string, izinkan map[string]string, bawaan string) *gorm.DB {
	if sort == "" {
		return db.Order(bawaan)
	}
	arah := "ASC"
	kolom := sort
	if strings.HasPrefix(kolom, "-") {
		arah = "DESC"
		kolom = kolom[1:]
	}
	nyata, ok := izinkan[kolom]
	if !ok {
		return db.Order(bawaan)
	}
	return db.Order(nyata + " " + arah)
}
