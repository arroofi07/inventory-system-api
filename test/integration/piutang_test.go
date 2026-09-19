package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPenyesuaianLunasTulisRiwayat(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	kodePlg := fmt.Sprintf("SC02_%d", suffix)
	kodeA := fmt.Sprintf("SC02_A_%d", suffix)

	plBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "nama_pelanggan": "Toko Lunas Adj",
		"tgl_registrasi": "2026-01-15", "phone": "08123456789",
		"territory": "Bandung", "distrik": "Timur", "alamat_toko": "Jl. Adj",
		"provinsi": "Jawa Barat", "kabupaten": "Bandung", "kecamatan": "Cibiru", "kelurahan": "Cipadung",
		"channel_outlet": "General Trade",
	})
	wPl := httptest.NewRecorder()
	reqPl := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan", bytes.NewReader(plBody))
	reqPl.Header.Set("Content-Type", "application/json")
	reqPl.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPl, reqPl)

	bmBody, _ := json.Marshal(map[string]any{
		"kode_barang": kodeA, "buat_barang_baru": true, "nama_item": "Item Adj",
		"brand": "ADJ", "no_faktur": fmt.Sprintf("ADJ_F_%d", suffix), "no_batch": "B1",
		"exp": "2027-12-31", "tanggal_masuk": "2026-01-10", "qty": 50, "harga": "10000.00",
		"markup_mt_type": "percent", "markup_mt_amount": "0.00",
		"markup_gt_type": "percent", "markup_gt_amount": "0.00",
	})
	wBM := httptest.NewRecorder()
	reqBM := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk", bytes.NewReader(bmBody))
	reqBM.Header.Set("Content-Type", "application/json")
	reqBM.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBM, reqBM)

	// Buat pending lunas: 1 item × 10000 + PPN 11% = 11100
	trxBody, _ := json.Marshal(map[string]any{
		"kode_pelanggan": kodePlg, "tanggal": "2026-09-04", "area": "Bandung",
		"item": map[string]any{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
		"ppn_persen": "11.00", "nominal_dibayar": "11100.00",
	})
	wTrx := httptest.NewRecorder()
	reqTrx := httptest.NewRequest(http.MethodPost, "/api/v1/transaksi", bytes.NewReader(trxBody))
	reqTrx.Header.Set("Content-Type", "application/json")
	reqTrx.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wTrx, reqTrx)
	if wTrx.Code != http.StatusCreated {
		t.Fatalf("buat: %d %s", wTrx.Code, wTrx.Body.String())
	}
	var trxOut struct {
		Data struct {
			ID               uint64 `json:"id"`
			StatusPembayaran string `json:"status_pembayaran"`
			JumlahDibayar    string `json:"jumlah_dibayar"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wTrx.Body.Bytes(), &trxOut)
	if trxOut.Data.StatusPembayaran != "lunas" {
		t.Fatalf("status=%s", trxOut.Data.StatusPembayaran)
	}

	// Tambah item → total naik → penyesuaian lunas tulis riwayat
	addBody, _ := json.Marshal(map[string]any{
		"item": map[string]any{"kode_item": kodeA, "qty": 1, "harga": "10000.00"},
	})
	wAdd := httptest.NewRecorder()
	reqAdd := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/items", trxOut.Data.ID), bytes.NewReader(addBody))
	reqAdd.Header.Set("Content-Type", "application/json")
	reqAdd.Header.Set("Authorization", "Bearer "+tokSales)
	r.ServeHTTP(wAdd, reqAdd)
	if wAdd.Code != http.StatusOK {
		t.Fatalf("add: %d %s", wAdd.Code, wAdd.Body.String())
	}
	var addOut struct {
		Data struct {
			StatusPembayaran string `json:"status_pembayaran"`
			JumlahDibayar    string `json:"jumlah_dibayar"`
			TotalAkhir       string `json:"total_akhir"`
			SisaHutang       string `json:"sisa_hutang"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wAdd.Body.Bytes(), &addOut)
	if addOut.Data.StatusPembayaran != "lunas" || addOut.Data.SisaHutang != "0.00" {
		t.Fatalf("setelah adj: %+v", addOut.Data)
	}
	if addOut.Data.JumlahDibayar != addOut.Data.TotalAkhir {
		t.Fatalf("dibayar=%s total=%s", addOut.Data.JumlahDibayar, addOut.Data.TotalAkhir)
	}

	var ket *string
	var nominal string
	_ = db.Raw(`SELECT keterangan, nominal_pembayaran FROM riwayat_pembayaran WHERE transaksi_penjualan_id = ? ORDER BY id DESC LIMIT 1`,
		trxOut.Data.ID).Row().Scan(&ket, &nominal)
	if ket == nil || *ket != "penyesuaian akibat perubahan total" {
		t.Fatalf("keterangan=%v", ket)
	}
	if nominal != "11100.00" {
		t.Fatalf("nominal delta=%s want 11100.00", nominal)
	}
}

func TestPiutangDaftarOverdueDanPenerimaan(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix, 5)

	// Set jatuh tempo ke masa lalu
	_ = db.Exec(`UPDATE transaksi_penjualan SET tanggal_jatuh_tempo = '2020-01-01' WHERE id = ?`, trxID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/piutang/overdue", nil)
	req.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("overdue: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Data []struct {
			TransaksiID        uint64 `json:"transaksi_id"`
			KategoriJatuhTempo string `json:"kategori_jatuh_tempo"`
		} `json:"data"`
		Ringkasan struct {
			JumlahTransaksiOverdue int `json:"jumlah_transaksi_overdue"`
		} `json:"ringkasan"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	found := false
	for _, row := range out.Data {
		if row.TransaksiID == trxID {
			found = true
			if row.KategoriJatuhTempo != "overdue" {
				t.Fatalf("kat=%s", row.KategoriJatuhTempo)
			}
		}
	}
	if !found {
		t.Fatal("trx overdue tidak muncul")
	}

	// Bayar sebagian lalu cek penerimaan
	bayarBody, _ := json.Marshal(map[string]any{
		"nominal_pembayaran": "10000.00",
		"tanggal_pembayaran": "2026-09-10",
	})
	wBayar := httptest.NewRecorder()
	reqBayar := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", trxID), bytes.NewReader(bayarBody))
	reqBayar.Header.Set("Content-Type", "application/json")
	reqBayar.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wBayar, reqBayar)

	wPen := httptest.NewRecorder()
	reqPen := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/penerimaan?date_from=2026-09-01&date_to=2026-09-30", nil)
	reqPen.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPen, reqPen)
	if wPen.Code != http.StatusOK {
		t.Fatalf("penerimaan: %d %s", wPen.Code, wPen.Body.String())
	}

	wNotif := httptest.NewRecorder()
	reqNotif := httptest.NewRequest(http.MethodGet, "/api/v1/notifikasi/piutang", nil)
	reqNotif.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wNotif, reqNotif)
	if wNotif.Code != http.StatusOK {
		t.Fatalf("notif: %d %s", wNotif.Code, wNotif.Body.String())
	}
}
