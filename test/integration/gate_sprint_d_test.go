package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/domain"
)

// TestGateSprintDF5 mengikat kriteria selesai F5 (SD-13) dalam satu alur E2E.
func TestGateSprintDF5(t *testing.T) {
	r, tokSales, tokSA, db, cleanup := setupTransaksiPratinjauRouter(t)
	defer cleanup()

	emailAdmin := fmt.Sprintf("sd13_adm_%d@pkb.test", time.Now().UnixNano()%100000)
	createUser(t, emailAdmin, "rahasia123", domain.RoleAdmin, true)
	tokAdmin := loginToken(t, r, emailAdmin, "rahasia123")

	suffix := time.Now().UnixNano() % 100000
	_, _, trxID := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix, 10)

	var totalAkhir, status string
	_ = db.Raw(`SELECT total_akhir, status_pembayaran FROM transaksi_penjualan WHERE id = ?`, trxID).
		Row().Scan(&totalAkhir, &status)
	if status != "hutang" {
		t.Fatalf("awal status=%s", status)
	}

	bayar := func(nominal, key string) (int, string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"nominal_pembayaran": nominal,
			"tanggal_pembayaran": "2026-09-10",
			"metode_pembayaran":  "Transfer",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", trxID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokAdmin)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		r.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	assertInvariant := func() {
		t.Helper()
		var dibayar, sisa, total string
		_ = db.Raw(`SELECT jumlah_dibayar, sisa_hutang, total_akhir FROM transaksi_penjualan WHERE id = ?`, trxID).
			Row().Scan(&dibayar, &sisa, &total)
		// bandingkan sebagai string fixed 2 desimal dari DB
		var ok int
		_ = db.Raw(`SELECT ABS(jumlah_dibayar + sisa_hutang - total_akhir) < 0.01 FROM transaksi_penjualan WHERE id = ?`, trxID).Scan(&ok)
		if ok != 1 {
			t.Fatalf("invariant I1 dilanggar dibayar=%s sisa=%s total=%s", dibayar, sisa, total)
		}
	}

	// --- Bayar parsial → sebagian ---
	code1, body1 := bayar("50000.00", "gate-d-1")
	if code1 != http.StatusOK {
		t.Fatalf("bayar1: %d %s", code1, body1)
	}
	var out1 struct {
		Data struct {
			StatusPembayaran string `json:"status_pembayaran"`
			JumlahDibayar    string `json:"jumlah_dibayar"`
			SisaHutang       string `json:"sisa_hutang"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body1), &out1)
	if out1.Data.StatusPembayaran != "sebagian" {
		t.Fatalf("status setelah parsial=%s", out1.Data.StatusPembayaran)
	}
	assertInvariant()

	// --- Overpay ditolak ---
	codeOver, bodyOver := bayar("999999.00", "")
	if codeOver != http.StatusConflict || !strings.Contains(bodyOver, "KELEBIHAN_BAYAR") {
		t.Fatalf("overpay: %d %s", codeOver, bodyOver)
	}
	assertInvariant()

	// --- Pending ditolak ---
	_, _, pendingID := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix+7, 2)
	_ = db.Exec(`UPDATE transaksi_penjualan SET status_approval='pending', no_transaksi=NULL WHERE id=?`, pendingID)
	wPend := httptest.NewRecorder()
	pendBody, _ := json.Marshal(map[string]any{"nominal_pembayaran": "1000.00"})
	reqPend := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", pendingID), bytes.NewReader(pendBody))
	reqPend.Header.Set("Content-Type", "application/json")
	reqPend.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wPend, reqPend)
	if wPend.Code != http.StatusConflict || !strings.Contains(wPend.Body.String(), "STATUS_TIDAK_VALID") {
		t.Fatalf("pending bayar: %d %s", wPend.Code, wPend.Body.String())
	}

	// --- Pelunasan → lunas + riwayat lengkap ---
	var sisaStr string
	_ = db.Raw(`SELECT sisa_hutang FROM transaksi_penjualan WHERE id = ?`, trxID).Scan(&sisaStr)
	code2, body2 := bayar(sisaStr, "gate-d-2")
	if code2 != http.StatusOK {
		t.Fatalf("lunas: %d %s", code2, body2)
	}
	var out2 struct {
		Data struct {
			StatusPembayaran string `json:"status_pembayaran"`
			SisaHutang       string `json:"sisa_hutang"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body2), &out2)
	if out2.Data.StatusPembayaran != "lunas" || out2.Data.SisaHutang != "0.00" {
		t.Fatalf("lunas out=%+v", out2.Data)
	}
	assertInvariant()

	wHist := httptest.NewRecorder()
	reqHist := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/pembayaran", trxID), nil)
	reqHist.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wHist, reqHist)
	var hist struct {
		Data []struct {
			NominalPembayaran string `json:"nominal_pembayaran"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wHist.Body.Bytes(), &hist)
	if len(hist.Data) < 2 {
		t.Fatalf("riwayat len=%d", len(hist.Data))
	}

	// --- Penerimaan = SUM(nominal_pembayaran) ---
	wPen := httptest.NewRecorder()
	reqPen := httptest.NewRequest(http.MethodGet, "/api/v1/laporan/penerimaan?date_from=2026-09-01&date_to=2026-09-30", nil)
	reqPen.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wPen, reqPen)
	if wPen.Code != http.StatusOK {
		t.Fatalf("penerimaan: %d %s", wPen.Code, wPen.Body.String())
	}
	var pen struct {
		Data struct {
			TotalNominal string `json:"total_nominal"`
			JumlahBaris  int    `json:"jumlah_baris"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wPen.Body.Bytes(), &pen)
	var sumAll string
	_ = db.Raw(`
		SELECT COALESCE(SUM(nominal_pembayaran),0)
		FROM riwayat_pembayaran
		WHERE tanggal_pembayaran BETWEEN '2026-09-01' AND '2026-09-30'`).Scan(&sumAll)
	var matchAll int
	_ = db.Raw(`SELECT ABS(CAST(? AS DECIMAL(15,2)) - CAST(? AS DECIMAL(15,2))) < 0.01`, pen.Data.TotalNominal, sumAll).Scan(&matchAll)
	if matchAll != 1 {
		t.Fatalf("penerimaan API=%s sumDB=%s", pen.Data.TotalNominal, sumAll)
	}
	if pen.Data.JumlahBaris < 2 {
		t.Fatalf("penerimaan baris=%d total=%s", pen.Data.JumlahBaris, pen.Data.TotalNominal)
	}

	// --- Faktur: cetak → non-SA diblokir → SA cetak ulang ---
	getFaktur := func(tok string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/transaksi/%d/faktur", trxID), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		return w
	}

	wF1 := getFaktur(tokAdmin)
	if wF1.Code != http.StatusOK {
		t.Fatalf("faktur1: %d %s", wF1.Code, wF1.Body.String())
	}
	var f1 struct {
		Data struct {
			Layout     string `json:"layout"`
			CetakUlang bool   `json:"cetak_ulang"`
			Ringkasan  struct {
				TotalAkhir string `json:"total_akhir"`
				Terbilang  string `json:"terbilang"`
			} `json:"ringkasan"`
			Items []struct {
				TotalFinalBaris string `json:"total_final_baris"`
				IsBonus         bool   `json:"is_bonus"`
			} `json:"items"`
			FakturDicetakAt *string `json:"faktur_dicetak_at"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wF1.Body.Bytes(), &f1)
	if f1.Data.Layout != "half" {
		t.Fatalf("layout 1 item (+bonus?) = %s", f1.Data.Layout)
	}
	if f1.Data.CetakUlang || f1.Data.FakturDicetakAt == nil {
		t.Fatalf("cetak pertama: ulang=%v at=%v", f1.Data.CetakUlang, f1.Data.FakturDicetakAt)
	}
	if !strings.Contains(f1.Data.Ringkasan.Terbilang, "rupiah") {
		t.Fatalf("terbilang=%q", f1.Data.Ringkasan.Terbilang)
	}
	var sumFinal float64
	for _, it := range f1.Data.Items {
		if it.IsBonus {
			continue
		}
		var v float64
		fmt.Sscanf(it.TotalFinalBaris, "%f", &v)
		sumFinal += v
	}
	var totalF float64
	fmt.Sscanf(f1.Data.Ringkasan.TotalAkhir, "%f", &totalF)
	if abs(sumFinal-totalF) >= 0.01 {
		t.Fatalf("alokasi sum=%f total=%f", sumFinal, totalF)
	}

	var dicetakAt1 time.Time
	_ = db.Raw(`SELECT faktur_dicetak_at FROM transaksi_penjualan WHERE id = ?`, trxID).Scan(&dicetakAt1)
	if dicetakAt1.IsZero() {
		t.Fatal("faktur_dicetak_at kosong di DB")
	}

	wF2 := getFaktur(tokAdmin)
	if wF2.Code != http.StatusConflict || !strings.Contains(wF2.Body.String(), "FAKTUR_TERKUNCI") {
		t.Fatalf("non-SA cetak2: %d %s", wF2.Code, wF2.Body.String())
	}

	wF3 := getFaktur(tokSA)
	if wF3.Code != http.StatusOK {
		t.Fatalf("SA cetak ulang: %d %s", wF3.Code, wF3.Body.String())
	}
	var f3 struct {
		Data struct {
			CetakUlang bool `json:"cetak_ulang"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wF3.Body.Bytes(), &f3)
	if !f3.Data.CetakUlang {
		t.Fatal("SA harus cetak_ulang=true")
	}
	var dicetakAt2 time.Time
	_ = db.Raw(`SELECT faktur_dicetak_at FROM transaksi_penjualan WHERE id = ?`, trxID).Scan(&dicetakAt2)
	if !dicetakAt1.Equal(dicetakAt2) {
		t.Fatalf("timestamp berubah: %v → %v", dicetakAt1, dicetakAt2)
	}

	wSalesF := getFaktur(tokSales)
	if wSalesF.Code != http.StatusForbidden {
		t.Fatalf("sales faktur: %d", wSalesF.Code)
	}

	// --- Overdue + notifikasi (lonceng) ---
	_, _, overdueID := seedApprovedTransaksi(t, r, tokSales, tokSA, suffix+99, 3)
	_ = db.Exec(`UPDATE transaksi_penjualan SET tanggal_jatuh_tempo='2026-08-01', status_pembayaran='hutang', jumlah_dibayar=0, sisa_hutang=total_akhir, faktur_dicetak_at=NULL WHERE id=?`, overdueID)

	wOv := httptest.NewRecorder()
	reqOv := httptest.NewRequest(http.MethodGet, "/api/v1/piutang/overdue", nil)
	reqOv.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wOv, reqOv)
	if wOv.Code != http.StatusOK {
		t.Fatalf("overdue: %d %s", wOv.Code, wOv.Body.String())
	}
	if !strings.Contains(wOv.Body.String(), fmt.Sprintf(`"transaksi_id":%d`, overdueID)) &&
		!strings.Contains(wOv.Body.String(), fmt.Sprintf(`"transaksi_id": %d`, overdueID)) {
		// JSON number without space
		var ov struct {
			Data []struct {
				TransaksiID uint64 `json:"transaksi_id"`
				Kategori    string `json:"kategori_jatuh_tempo"`
			} `json:"data"`
		}
		_ = json.Unmarshal(wOv.Body.Bytes(), &ov)
		found := false
		for _, row := range ov.Data {
			if row.TransaksiID == overdueID {
				found = true
				if row.Kategori != "overdue" {
					t.Fatalf("kat=%s", row.Kategori)
				}
			}
		}
		if !found {
			t.Fatalf("overdue tidak memuat trx %d: %s", overdueID, wOv.Body.String())
		}
	}

	wNot := httptest.NewRecorder()
	reqNot := httptest.NewRequest(http.MethodGet, "/api/v1/notifikasi/piutang", nil)
	reqNot.Header.Set("Authorization", "Bearer "+tokSA)
	r.ServeHTTP(wNot, reqNot)
	if wNot.Code != http.StatusOK {
		t.Fatalf("notifikasi: %d %s", wNot.Code, wNot.Body.String())
	}
	var notif struct {
		Data struct {
			Overdue int `json:"overdue"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wNot.Body.Bytes(), &notif)
	if notif.Data.Overdue < 1 {
		t.Fatalf("notifikasi overdue=%d", notif.Data.Overdue)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
