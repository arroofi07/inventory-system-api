package domain

import (
	"errors"
	"fmt"
)

var (
	ErrStokTidakCukup   = errors.New("stok tidak cukup")
	ErrHargaTidakSesuai = errors.New("harga tidak sesuai channel batch")
	ErrFakturTerkunci   = errors.New("faktur sudah dicetak")
	ErrStatusTidakValid = errors.New("status tidak valid")
	ErrKelebihanBayar   = errors.New("kelebihan bayar")
	ErrTidakDitemukan   = errors.New("data tidak ditemukan")
	ErrTidakDiizinkan   = errors.New("tidak diizinkan")
	ErrDuplikat         = errors.New("data duplikat")
	ErrImporValidasi    = errors.New("impor CSV gagal validasi")

	ErrKredensialSalah         = errors.New("kredensial salah")
	ErrAkunNonaktif            = errors.New("akun nonaktif")
	ErrRefreshTokenTidakValid  = errors.New("refresh token tidak valid")
	ErrRefreshTokenKedaluwarsa = errors.New("refresh token kedaluwarsa")
	ErrTerlaluBanyakPercobaan  = errors.New("terlalu banyak percobaan")
)

// GalatImporBaris satu kesalahan pada baris CSV (1-based termasuk header sebagai baris 1).
type GalatImporBaris struct {
	Baris   int    `json:"baris"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// ErrImporCSV validasi impor gagal; tidak ada data yang ditulis.
type ErrImporCSV struct {
	Galat []GalatImporBaris
}

func (e *ErrImporCSV) Error() string {
	return fmt.Sprintf("%s (%d galat)", ErrImporValidasi.Error(), len(e.Galat))
}

func (e *ErrImporCSV) Unwrap() error { return ErrImporValidasi }

// StokKurangBaris rincian SKU yang gagal di approval gate.
type StokKurangBaris struct {
	KodeItem string
	NamaItem string
	Diminta  int
	Tersedia int
}

// ErrStokApproval dibungkus saat approval gagal karena stok (tetap pending).
type ErrStokApproval struct {
	Details []StokKurangBaris
}

func (e *ErrStokApproval) Error() string {
	return fmt.Sprintf("%s (%d SKU)", ErrStokTidakCukup.Error(), len(e.Details))
}

func (e *ErrStokApproval) Unwrap() error { return ErrStokTidakCukup }
