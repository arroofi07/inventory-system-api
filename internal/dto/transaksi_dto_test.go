package dto_test

import (
	"encoding/json"
	"testing"

	"app/internal/dto"
)

func TestNormalisasiItemsModeSingleDanMulti(t *testing.T) {
	t.Parallel()

	t.Run("multi items", func(t *testing.T) {
		t.Parallel()
		req := dto.TransaksiPratinjauRequest{
			KodePelanggan: "0874",
			Tanggal:       "2026-09-04",
			Items: []dto.TransaksiItemRequest{
				{KodeItem: "SHS001", Qty: 10, Harga: "20000.00", KodePromos: []string{"PROMO2026090001"}},
				{KodeItem: "MSK014", Qty: 5, Harga: "30000.00"},
			},
		}
		items, err := req.NormalisasiItems()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || !dto.IsMultiItem(items) {
			t.Fatalf("want multi 2 items, got %+v", items)
		}
		if len(items[0].KodePromos) != 1 || items[0].KodePromos[0] != "PROMO2026090001" {
			t.Fatalf("promo by kode: %+v", items[0].KodePromos)
		}
	})

	t.Run("single item di header", func(t *testing.T) {
		t.Parallel()
		req := dto.TransaksiCreateRequest{
			KodePelanggan: "0874",
			Tanggal:       "2026-09-04",
			Area:          "Bandung",
			Item: &dto.TransaksiItemRequest{
				KodeItem:   "SHS001",
				Qty:        3,
				Harga:      "20000.00",
				KodePromos: []string{"P1"},
			},
		}
		items, err := req.NormalisasiItems()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || dto.IsMultiItem(items) {
			t.Fatalf("want single, got %+v", items)
		}
	})

	t.Run("tolak item sekaligus items", func(t *testing.T) {
		t.Parallel()
		req := dto.TransaksiAddItemsRequest{
			Item:  &dto.TransaksiItemRequest{KodeItem: "A", Qty: 1, Harga: "1"},
			Items: []dto.TransaksiItemRequest{{KodeItem: "B", Qty: 1, Harga: "1"}},
		}
		if _, err := req.NormalisasiItems(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("json roundtrip jumlah dan total_qty_keluar", func(t *testing.T) {
		t.Parallel()
		item := dto.TransaksiItemResponse{
			KodeItem:       "SHS001",
			Jumlah:         10,
			QtyPromo:       1,
			TotalQtyKeluar: 11,
			Harga:          "20000.00",
			PromoDiterapkan: []dto.PromoTerapanResponse{
				{KodePromo: "PROMO2026090001", NamaPromo: "Beli 10 Gratis 1", QtyBonus: 1, NilaiDiskon: "0.00"},
			},
		}
		b, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := raw["jumlah"]; !ok {
			t.Fatal("jumlah wajib ada di JSON")
		}
		if _, ok := raw["total_qty_keluar"]; !ok {
			t.Fatal("total_qty_keluar wajib ada di JSON")
		}
		if raw["jumlah"].(float64) != 10 || raw["total_qty_keluar"].(float64) != 11 {
			t.Fatalf("nilai qty salah: %v", raw)
		}
		promos := raw["promo_diterapkan"].([]any)
		p0 := promos[0].(map[string]any)
		if p0["kode_promo"] != "PROMO2026090001" {
			t.Fatalf("promo by kode: %v", p0)
		}
	})
}
