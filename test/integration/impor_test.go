package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/pkg/csvx"
)

func TestImporBarangMasukAllOrNothing(t *testing.T) {
	eng, tokAdmin, _, cleanup := setupBarangMasukRouter(t)
	defer cleanup()
	r := eng.r

	suffix := time.Now().UnixNano() % 100000
	kodeOK := fmt.Sprintf("SE04_OK_%d", suffix)
	kodeBad := fmt.Sprintf("SE04_BAD_%d", suffix)

	// Baris 3 qty invalid → seluruh berkas ditolak
	csvBad := string(csvx.BOMUTF8) + strings.Join([]string{
		"kode_barang,nama_item,brand,no_faktur,no_batch,exp,tanggal_masuk,qty,harga,disc_hpp_1,disc_hpp_2,disc_hpp_3,markup_mt_type,markup_mt_amount,markup_gt_type,markup_gt_amount",
		fmt.Sprintf("%s,Item OK,BR,F1,B1,2027-12-31,2026-01-10,10,10000.00,0,0,0,percent,0,percent,0", kodeOK),
		fmt.Sprintf("%s,Item Bad,BR,F2,B2,2027-12-31,2026-01-10,0,10000.00,0,0,0,percent,0,percent,0", kodeBad),
	}, "\n")

	postImpor := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		part, _ := mw.CreateFormFile("file", "impor.csv")
		_, _ = part.Write([]byte(body))
		_ = mw.Close()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/barang-masuk/impor", &buf)
		req.Header.Set("Authorization", "Bearer "+tokAdmin)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		r.ServeHTTP(w, req)
		return w
	}

	wBad := postImpor(csvBad)
	if wBad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 got %d %s", wBad.Code, wBad.Body.String())
	}
	if !strings.Contains(wBad.Body.String(), "baris_3") && !strings.Contains(wBad.Body.String(), `"baris":3`) {
		// field encoded as baris_3.qty
		if !strings.Contains(wBad.Body.String(), "qty") {
			t.Fatalf("galat baris: %s", wBad.Body.String())
		}
	}

	// Pastikan tidak ada barang dari baris valid
	wList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/barang?q="+kodeOK, nil)
	reqList.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wList, reqList)
	if strings.Contains(wList.Body.String(), kodeOK) {
		t.Fatalf("all-or-nothing gagal: data parsial tertulis: %s", wList.Body.String())
	}

	csvOK := string(csvx.BOMUTF8) + strings.Join([]string{
		"kode_barang,nama_item,brand,no_faktur,no_batch,exp,tanggal_masuk,qty,harga,disc_hpp_1,disc_hpp_2,disc_hpp_3,markup_mt_type,markup_mt_amount,markup_gt_type,markup_gt_amount",
		fmt.Sprintf("%s,Item OK,BR,F1,B1,2027-12-31,2026-01-10,10,10000.00,0,0,0,percent,0,percent,0", kodeOK),
		fmt.Sprintf("%s_2,Item OK2,BR,F2,B2,2027-12-31,2026-01-10,5,10000.00,0,0,0,percent,0,percent,0", kodeOK),
	}, "\n")
	wOK := postImpor(csvOK)
	if wOK.Code != http.StatusOK {
		t.Fatalf("impor ok: %d %s", wOK.Code, wOK.Body.String())
	}
	var env struct {
		Data struct {
			JumlahBaris int `json:"jumlah_baris"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wOK.Body.Bytes(), &env)
	if env.Data.JumlahBaris != 2 {
		t.Fatalf("jumlah=%d", env.Data.JumlahBaris)
	}
}

func TestImporPelangganAllOrNothing(t *testing.T) {
	eng, _, tokAdmin, cleanup := setupPelangganRouter(t)
	defer cleanup()
	r := eng.r

	suffix := time.Now().UnixNano() % 100000
	kode1 := fmt.Sprintf("SB06_I1_%d", suffix)
	kode2 := fmt.Sprintf("SB06_I2_%d", suffix)

	header := "kode_pelanggan,nama_pelanggan,tgl_registrasi,phone,territory,distrik,alamat_toko,provinsi,kabupaten,kecamatan,kelurahan,channel_outlet,npwp_nik,rt_rw,kode_pos,nominal_pengambilan_pertama,estimasi_batas_kredit"
	csvBad := string(csvx.BOMUTF8) + strings.Join([]string{
		header,
		fmt.Sprintf("%s,Toko1,2026-01-15,081,Jakarta,Selatan,Jl1,DKI,Jaksel,Kebayoran,Senayan,General Trade,,,,0.00,0.00", kode1),
		fmt.Sprintf("%s,Toko2,2026-01-15,081,Jakarta,Selatan,Jl2,DKI,Jaksel,Kebayoran,Senayan,CHANNEL_SALAH,,,,0.00,0.00", kode2),
	}, "\n")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", "plg.csv")
	_, _ = part.Write([]byte(csvBad))
	_ = mw.Close()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pelanggan/impor", &buf)
	req.Header.Set("Authorization", "Bearer "+tokAdmin)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 got %d %s", w.Code, w.Body.String())
	}

	wGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/pelanggan?q="+kode1, nil)
	reqGet.Header.Set("Authorization", "Bearer "+tokAdmin)
	r.ServeHTTP(wGet, reqGet)
	if strings.Contains(wGet.Body.String(), kode1) {
		t.Fatalf("pelanggan parsial tertulis: %s", wGet.Body.String())
	}
}
