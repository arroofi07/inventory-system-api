package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/csvx"
	appvalidator "app/internal/pkg/validator"
	"gorm.io/gorm"
)

// ImportService impor CSV all-or-nothing (SE-04).
type ImportService struct {
	db *gorm.DB
	bm *BarangMasukService
	pl *PelangganService
}

func NewImportService(db *gorm.DB, bm *BarangMasukService, pl *PelangganService) *ImportService {
	return &ImportService{db: db, bm: bm, pl: pl}
}

// HasilImpor ringkasan sukses.
type HasilImpor struct {
	JumlahBaris int `json:"jumlah_baris"`
}

// ImporBarangMasuk memvalidasi seluruh berkas lalu menulis dalam satu transaksi.
func (s *ImportService) ImporBarangMasuk(ctx context.Context, data []byte, meta AuditMeta) (*HasilImpor, error) {
	reqs, galat, err := parseBarangMasukCSV(data)
	if err != nil {
		return nil, err
	}
	if len(galat) > 0 {
		return nil, &domain.ErrImporCSV{Galat: galat}
	}
	if len(reqs) == 0 {
		return nil, &domain.ErrImporCSV{Galat: []domain.GalatImporBaris{{
			Baris: 1, Message: "tidak ada baris data (hanya header atau komentar)",
		}}}
	}
	for i, req := range reqs {
		if err := appvalidator.Struct(req); err != nil {
			galat = append(galat, galatDariValidasi(i+2, err)...)
		}
	}
	if len(galat) > 0 {
		return nil, &domain.ErrImporCSV{Galat: galat}
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inner := *s.bm
		inner.db = tx
		for i, req := range reqs {
			if _, err := inner.Buat(ctx, req, meta); err != nil {
				return bungkusGalatImpor(i+2, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &HasilImpor{JumlahBaris: len(reqs)}, nil
}

// ImporPelanggan memvalidasi seluruh berkas lalu menulis dalam satu transaksi.
func (s *ImportService) ImporPelanggan(ctx context.Context, data []byte, meta AuditMeta) (*HasilImpor, error) {
	reqs, galat, err := parsePelangganCSV(data)
	if err != nil {
		return nil, err
	}
	if len(galat) > 0 {
		return nil, &domain.ErrImporCSV{Galat: galat}
	}
	if len(reqs) == 0 {
		return nil, &domain.ErrImporCSV{Galat: []domain.GalatImporBaris{{
			Baris: 1, Message: "tidak ada baris data (hanya header atau komentar)",
		}}}
	}
	for i, req := range reqs {
		if err := appvalidator.Struct(req); err != nil {
			galat = append(galat, galatDariValidasi(i+2, err)...)
		} else if !domain.ChannelOutletValid(domain.ChannelOutlet(req.ChannelOutlet)) {
			galat = append(galat, domain.GalatImporBaris{Baris: i + 2, Field: "channel_outlet", Message: "harus salah satu dari 5 channel outlet"})
		}
	}
	if len(galat) > 0 {
		return nil, &domain.ErrImporCSV{Galat: galat}
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inner := *s.pl
		inner.db = tx
		for i, req := range reqs {
			if _, err := inner.Buat(ctx, req, meta); err != nil {
				return bungkusGalatImpor(i+2, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &HasilImpor{JumlahBaris: len(reqs)}, nil
}

func bungkusGalatImpor(baris int, err error) error {
	if err == nil {
		return nil
	}
	var ie *domain.ErrImporCSV
	if errors.As(err, &ie) {
		return ie
	}
	return &domain.ErrImporCSV{Galat: galatDariValidasi(baris, err)}
}

func galatDariValidasi(baris int, err error) []domain.GalatImporBaris {
	var ve *appvalidator.Errors
	if errors.As(err, &ve) {
		galat := make([]domain.GalatImporBaris, 0, len(ve.Fields))
		for _, f := range ve.Fields {
			galat = append(galat, domain.GalatImporBaris{Baris: baris, Field: f.Field, Message: f.Message})
		}
		if len(galat) > 0 {
			return galat
		}
	}
	field, msg := pecahErrorField(err)
	return []domain.GalatImporBaris{{Baris: baris, Field: field, Message: msg}}
}

func pecahErrorField(err error) (field, msg string) {
	msg = err.Error()
	// ValidationError dari appvalidator sering "field: pesan"
	if parts := strings.SplitN(msg, ": ", 2); len(parts) == 2 && !strings.Contains(parts[0], " ") {
		return parts[0], parts[1]
	}
	return "", msg
}

func parseBarangMasukCSV(data []byte) ([]dto.BarangMasukCreateRequest, []domain.GalatImporBaris, error) {
	records, err := csvx.BacaRecords(data)
	if err != nil {
		return nil, []domain.GalatImporBaris{{Baris: 1, Message: "CSV tidak dapat dibaca: " + err.Error()}}, nil
	}
	if len(records) == 0 {
		return nil, []domain.GalatImporBaris{{Baris: 1, Message: "berkas kosong"}}, nil
	}
	idx := csvx.IndeksHeader(records[0])
	wajib := []string{"kode_barang", "no_faktur", "no_batch", "exp", "tanggal_masuk", "qty", "harga"}
	var galat []domain.GalatImporBaris
	for _, k := range wajib {
		if _, ok := idx[k]; !ok {
			galat = append(galat, domain.GalatImporBaris{Baris: 1, Field: k, Message: "kolom wajib hilang di header"})
		}
	}
	if len(galat) > 0 {
		return nil, galat, nil
	}

	var reqs []dto.BarangMasukCreateRequest
	seenBatch := map[string]int{}
	for i := 1; i < len(records); i++ {
		row := records[i]
		baris := i + 1
		if csvx.LewatiBaris(row) {
			continue
		}
		qtyStr := csvx.AmbilKolom(row, idx, "qty")
		qty, errQty := strconv.Atoi(qtyStr)
		if errQty != nil || qty < 1 {
			galat = append(galat, domain.GalatImporBaris{Baris: baris, Field: "qty", Message: "harus bilangan bulat ≥ 1"})
			continue
		}
		kode := csvx.AmbilKolom(row, idx, "kode_barang")
		noFaktur := csvx.AmbilKolom(row, idx, "no_faktur")
		noBatch := csvx.AmbilKolom(row, idx, "no_batch")
		key := kode + "|" + noFaktur + "|" + noBatch
		if prev, ok := seenBatch[key]; ok {
			galat = append(galat, domain.GalatImporBaris{
				Baris: baris, Field: "no_batch",
				Message: fmt.Sprintf("duplikat dengan baris %d dalam berkas yang sama", prev),
			})
			continue
		}
		seenBatch[key] = baris

		nama := csvx.AmbilKolom(row, idx, "nama_item")
		brand := csvx.AmbilKolom(row, idx, "brand")
		buatBaru := nama != "" || brand != ""
		mtType := csvx.AmbilKolom(row, idx, "markup_mt_type")
		if mtType == "" {
			mtType = "percent"
		}
		gtType := csvx.AmbilKolom(row, idx, "markup_gt_type")
		if gtType == "" {
			gtType = "percent"
		}
		mtAmt := csvx.AmbilKolom(row, idx, "markup_mt_amount")
		if mtAmt == "" {
			mtAmt = "0.00"
		}
		gtAmt := csvx.AmbilKolom(row, idx, "markup_gt_amount")
		if gtAmt == "" {
			gtAmt = "0.00"
		}
		disc1 := csvx.AmbilKolom(row, idx, "disc_hpp_1")
		if disc1 == "" {
			disc1 = "0.00"
		}
		disc2 := csvx.AmbilKolom(row, idx, "disc_hpp_2")
		if disc2 == "" {
			disc2 = "0.00"
		}
		disc3 := csvx.AmbilKolom(row, idx, "disc_hpp_3")
		if disc3 == "" {
			disc3 = "0.00"
		}

		req := dto.BarangMasukCreateRequest{
			KodeBarang: kode, BuatBarangBaru: buatBaru, NamaItem: nama, Brand: brand,
			Satuan: csvx.AmbilKolom(row, idx, "satuan"),
			NoFaktur: noFaktur, NoBatch: noBatch,
			Exp: csvx.AmbilKolom(row, idx, "exp"), TanggalMasuk: csvx.AmbilKolom(row, idx, "tanggal_masuk"),
			Qty: qty, Harga: csvx.AmbilKolom(row, idx, "harga"),
			DiscHPP1: disc1, DiscHPP2: disc2, DiscHPP3: disc3,
			MarkupMTType: mtType, MarkupMTAmount: mtAmt,
			MarkupGTType: gtType, MarkupGTAmount: gtAmt,
		}
		if req.KodeBarang == "" {
			galat = append(galat, domain.GalatImporBaris{Baris: baris, Field: "kode_barang", Message: "wajib"})
			continue
		}
		if buatBaru && (nama == "" || brand == "") {
			galat = append(galat, domain.GalatImporBaris{Baris: baris, Field: "nama_item", Message: "nama_item dan brand wajib bila membuat barang baru"})
			continue
		}
		reqs = append(reqs, req)
	}
	return reqs, galat, nil
}

func parsePelangganCSV(data []byte) ([]dto.PelangganCreateRequest, []domain.GalatImporBaris, error) {
	records, err := csvx.BacaRecords(data)
	if err != nil {
		return nil, []domain.GalatImporBaris{{Baris: 1, Message: "CSV tidak dapat dibaca: " + err.Error()}}, nil
	}
	if len(records) == 0 {
		return nil, []domain.GalatImporBaris{{Baris: 1, Message: "berkas kosong"}}, nil
	}
	idx := csvx.IndeksHeader(records[0])
	wajib := []string{
		"kode_pelanggan", "nama_pelanggan", "tgl_registrasi", "phone",
		"territory", "distrik", "alamat_toko", "provinsi", "kabupaten", "kecamatan", "kelurahan",
		"channel_outlet",
	}
	var galat []domain.GalatImporBaris
	for _, k := range wajib {
		if _, ok := idx[k]; !ok {
			galat = append(galat, domain.GalatImporBaris{Baris: 1, Field: k, Message: "kolom wajib hilang di header"})
		}
	}
	if len(galat) > 0 {
		return nil, galat, nil
	}

	var reqs []dto.PelangganCreateRequest
	seen := map[string]int{}
	for i := 1; i < len(records); i++ {
		row := records[i]
		baris := i + 1
		if csvx.LewatiBaris(row) {
			continue
		}
		kode := csvx.AmbilKolom(row, idx, "kode_pelanggan")
		if kode == "" {
			galat = append(galat, domain.GalatImporBaris{Baris: baris, Field: "kode_pelanggan", Message: "wajib"})
			continue
		}
		if prev, ok := seen[kode]; ok {
			galat = append(galat, domain.GalatImporBaris{
				Baris: baris, Field: "kode_pelanggan",
				Message: fmt.Sprintf("duplikat dengan baris %d dalam berkas yang sama", prev),
			})
			continue
		}
		seen[kode] = baris

		npwp := ptrOrNil(csvx.AmbilKolom(row, idx, "npwp_nik"))
		rt := ptrOrNil(csvx.AmbilKolom(row, idx, "rt_rw"))
		pos := ptrOrNil(csvx.AmbilKolom(row, idx, "kode_pos"))
		nominal := ptrOrNil(csvx.AmbilKolom(row, idx, "nominal_pengambilan_pertama"))
		kredit := ptrOrNil(csvx.AmbilKolom(row, idx, "estimasi_batas_kredit"))

		reqs = append(reqs, dto.PelangganCreateRequest{
			KodePelanggan: kode,
			NamaPelanggan: csvx.AmbilKolom(row, idx, "nama_pelanggan"),
			TglRegistrasi: csvx.AmbilKolom(row, idx, "tgl_registrasi"),
			Phone:         csvx.AmbilKolom(row, idx, "phone"),
			NPWPNIK:       npwp,
			Territory:     csvx.AmbilKolom(row, idx, "territory"),
			Distrik:       csvx.AmbilKolom(row, idx, "distrik"),
			AlamatToko:    csvx.AmbilKolom(row, idx, "alamat_toko"),
			RTRW:          rt,
			Provinsi:      csvx.AmbilKolom(row, idx, "provinsi"),
			Kabupaten:     csvx.AmbilKolom(row, idx, "kabupaten"),
			Kecamatan:     csvx.AmbilKolom(row, idx, "kecamatan"),
			Kelurahan:     csvx.AmbilKolom(row, idx, "kelurahan"),
			KodePos:       pos,
			ChannelOutlet: csvx.AmbilKolom(row, idx, "channel_outlet"),
			NominalPengambilanPertama: nominal,
			EstimasiBatasKredit:       kredit,
		})
	}
	return reqs, galat, nil
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
