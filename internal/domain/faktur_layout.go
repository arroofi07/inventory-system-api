package domain

// LayoutFaktur jenis layout cetak (04 §6.3).
type LayoutFaktur string

const (
	LayoutFakturHalf      LayoutFaktur = "half"
	LayoutFakturFull      LayoutFaktur = "full"
	LayoutFakturPaginated LayoutFaktur = "paginated"
)

// PilihLayoutFaktur menentukan layout dan total halaman dari jumlah baris cetak.
// halfMax / fullMax / perPage dari env FAKTUR_ITEMS_*.
func PilihLayoutFaktur(jumlahBaris, halfMax, fullMax, perPage int) (LayoutFaktur, int) {
	if halfMax < 1 {
		halfMax = 10
	}
	if fullMax < halfMax {
		fullMax = 30
	}
	if perPage < 1 {
		perPage = 30
	}
	if jumlahBaris <= 0 {
		return LayoutFakturHalf, 1
	}
	if jumlahBaris <= halfMax {
		return LayoutFakturHalf, 1
	}
	if jumlahBaris <= fullMax {
		return LayoutFakturFull, 1
	}
	halaman := (jumlahBaris + perPage - 1) / perPage
	if halaman < 1 {
		halaman = 1
	}
	return LayoutFakturPaginated, halaman
}

// HitungJumlahBarisCetakFaktur: setiap detail = 1 baris; bonus qty_promo > 0 menambah 1 baris.
func HitungJumlahBarisCetakFaktur(details []TransaksiDetail) int {
	n := len(details)
	for _, d := range details {
		if d.QtyPromo > 0 {
			n++
		}
	}
	return n
}
