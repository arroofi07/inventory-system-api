package fakturpdf

import (
	"bytes"
	"strings"
	"testing"

	"app/internal/dto"
)

func TestNamaBerkas(t *testing.T) {
	no := "000042"
	if NamaBerkas(&no, 9) != "faktur-000042.pdf" {
		t.Fatalf("%s", NamaBerkas(&no, 9))
	}
	if NamaBerkas(nil, 9) != "faktur-9.pdf" {
		t.Fatal(NamaBerkas(nil, 9))
	}
	jelek := `../evil 001`
	if NamaBerkas(&jelek, 1) != "faktur-evil001.pdf" {
		t.Fatalf("%s", NamaBerkas(&jelek, 1))
	}
}

func TestBangunMemuatAngkaJSON(t *testing.T) {
	no := "000123"
	body, err := Bangun(&dto.FakturResponse{
		NoTransaksi:  &no,
		Tanggal:      "2026-09-04",
		Layout:       "half",
		TotalHalaman: 1,
		Perusahaan:   dto.FakturPerusahaan{Nama: "PT Contoh"},
		Pelanggan: dto.FakturPelanggan{
			KodePelanggan: "PL01", NamaPelanggan: "Toko A", ChannelOutlet: "General Trade",
		},
		Items: []dto.FakturItem{{
			Urutan: 1, KodeItem: "SKU1", NamaItem: "Item Uji", Qty: 5, Satuan: "PCS",
			Harga: "10000.00", TotalFinalBaris: "347985.00",
		}},
		Ringkasan: dto.FakturRingkasan{
			Total: "313500.00", PPNPersen: "11.00", PPNNominal: "34485.00",
			TotalAkhir: "347985.00",
			Terbilang:  "tiga ratus empat puluh tujuh ribu sembilan ratus delapan puluh lima rupiah",
		},
	}, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatal("bukan PDF")
	}
	s := string(body)
	for _, want := range []string{
		"347985.00",
		"tiga ratus empat puluh tujuh ribu sembilan ratus delapan puluh lima rupiah",
		"000123",
		"Toko A",
		"PT Contoh",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF tidak mengandung %q", want)
		}
	}
}

func TestBangunCetakUlangDanPaginated(t *testing.T) {
	items := make([]dto.FakturItem, 0, 31)
	for i := 1; i <= 31; i++ {
		items = append(items, dto.FakturItem{
			Urutan: uint16(i), KodeItem: "K", NamaItem: "Barang", Qty: 1, Satuan: "PCS",
			Harga: "1.00", TotalFinalBaris: "1.00",
		})
	}
	body, err := Bangun(&dto.FakturResponse{
		Tanggal:      "2026-09-19",
		Layout:       "paginated",
		TotalHalaman: 2,
		CetakUlang:   true,
		Perusahaan:   dto.FakturPerusahaan{Nama: "PT X"},
		Pelanggan:    dto.FakturPelanggan{NamaPelanggan: "Y", KodePelanggan: "Z", ChannelOutlet: "General Trade"},
		Items:        items,
		Ringkasan:    dto.FakturRingkasan{Total: "31.00", PPNPersen: "0.00", PPNNominal: "0.00", TotalAkhir: "31.00", Terbilang: "tiga puluh satu rupiah"},
	}, 30)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "CETAK ULANG") {
		t.Fatal("banner cetak ulang hilang")
	}
	if !strings.Contains(s, "Halaman 1 / 2") || !strings.Contains(s, "Halaman 2 / 2") {
		t.Fatal("paginasi hilang")
	}
	if !strings.Contains(s, "31.00") {
		t.Fatal("total_akhir tidak di PDF")
	}
}
