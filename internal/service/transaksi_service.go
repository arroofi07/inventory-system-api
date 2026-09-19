package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/pkg/money"
	appvalidator "app/internal/pkg/validator"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TransaksiService perhitungan dan persistensi penjualan.
type TransaksiService struct {
	db         *gorm.DB
	pelanggan  *repository.PelangganRepo
	barang     *repository.BarangRepo
	stock      *repository.StockRepo
	promo      *repository.PromoRepo
	trx        *repository.TransaksiRepo
	bayar      *repository.PembayaranRepo
	audit      *repository.AuditRepo
	clock      clock.Clock
	ppnDefault decimal.Decimal
}

func NewTransaksiService(
	db *gorm.DB,
	pelanggan *repository.PelangganRepo,
	barang *repository.BarangRepo,
	stock *repository.StockRepo,
	promo *repository.PromoRepo,
	trx *repository.TransaksiRepo,
	bayar *repository.PembayaranRepo,
	audit *repository.AuditRepo,
	clk clock.Clock,
	ppnDefault decimal.Decimal,
) *TransaksiService {
	if clk == nil {
		clk = clock.Real{}
	}
	return &TransaksiService{
		db:         db,
		pelanggan:  pelanggan,
		barang:     barang,
		stock:      stock,
		promo:      promo,
		trx:        trx,
		bayar:      bayar,
		audit:      audit,
		clock:      clk,
		ppnDefault: ppnDefault,
	}
}

type hitungBarisOpts struct {
	StrictStock bool
	StrictHarga bool
}

type keranjangHasil struct {
	plg                    *domain.Pelanggan
	tanggal                time.Time
	disc1G, disc2G, disc3G decimal.Decimal
	ppn                    decimal.Decimal
	baris                  []barisPratinjauHasil
	calcItems              []domain.ItemInput
	hasil                  domain.TransaksiHasil
	totalQtyDitagih        int
	totalQtyKeluar         int
	semuaCukup             bool
	peringatan             []string
}

// CekStok memeriksa ketersediaan stok per SKU/batch dari ledger tanpa menulis DB (SC-05).
// Soft-check: hasil bisa berubah sampai approval; bukan jaminan alokasi.
func (s *TransaksiService) CekStok(ctx context.Context, req dto.TransaksiCekStokRequest) (*dto.TransaksiCekStokResponse, error) {
	if len(req.Items) == 0 {
		return nil, fieldErr("items", "wajib diisi minimal 1 baris")
	}
	db := s.db.WithContext(ctx)
	hariIni := s.clock.Now().UTC()
	out := make([]dto.CekStokItemResponse, 0, len(req.Items))
	semuaCukup := true

	for i, it := range req.Items {
		prefix := fmt.Sprintf("items[%d]", i)
		kode := strings.TrimSpace(it.KodeItem)
		if kode == "" {
			return nil, fieldErr(prefix+".kode_item", "wajib diisi")
		}
		if it.Qty < 1 {
			return nil, fieldErr(prefix+".qty", "minimal 1")
		}
		brg, err := s.barang.FindByKode(db, kode)
		if err != nil {
			return nil, err
		}
		kandidat, stokSKU, err := s.kandidatBatch(db, brg, it.BarangMasukID, hariIni)
		if err != nil {
			return nil, err
		}
		tersedia := stokSKU
		if it.BarangMasukID != nil && len(kandidat) == 1 {
			tersedia = kandidat[0].QtyTersedia
		}
		cukup := tersedia >= it.Qty
		if !cukup {
			semuaCukup = false
		}
		batch := make([]dto.CekStokBatchResponse, 0, len(kandidat))
		for _, k := range kandidat {
			batch = append(batch, dto.CekStokBatchResponse{
				BarangMasukID: k.BarangMasukID,
				NoBatch:       k.NoBatch,
				Exp:           k.Exp.UTC().Format("2006-01-02"),
				QtyTersedia:   k.QtyTersedia,
			})
		}
		out = append(out, dto.CekStokItemResponse{
			KodeItem: kode, NamaItem: brg.NamaItem,
			QtyDiminta: it.Qty, StokTersedia: tersedia, StokCukup: cukup, Batch: batch,
		})
	}

	return &dto.TransaksiCekStokResponse{
		Items:          out,
		SemuaStokCukup: semuaCukup,
		Catatan:        "Hasil bersifat sementara dan konsisten dengan ledger saat ini; jaminan alokasi hanya saat approval.",
	}, nil
}

// Pratinjau menghitung transaksi tanpa menulis DB (SC-02).
func (s *TransaksiService) Pratinjau(ctx context.Context, req dto.TransaksiPratinjauRequest) (*dto.PratinjauTransaksiResponse, error) {
	itemsReq, err := req.NormalisasiItems()
	if err != nil {
		return nil, fieldErr("items", err.Error())
	}
	k, err := s.hitungKeranjang(ctx, req.KodePelanggan, req.Tanggal, itemsReq,
		req.Disc1Persen, req.Disc2Persen, req.Disc3Persen, req.PPNPersen,
		hitungBarisOpts{},
	)
	if err != nil {
		return nil, err
	}

	barisOut := make([]dto.PratinjauItemResponse, 0, len(k.baris))
	for i := range k.baris {
		resp := k.baris[i].resp
		if i < len(k.hasil.Items) {
			resp.Subtotal = dto.FormatUang(k.hasil.Items[i].Subtotal)
			resp.TotalAfterDisc = dto.FormatUang(k.hasil.Items[i].TotalAfterDisc)
			resp.DiskonPromo = dto.FormatUang(k.calcItems[i].DiskonPromo)
		}
		barisOut = append(barisOut, resp)
	}
	peringatan := k.peringatan
	if peringatan == nil {
		peringatan = []string{}
	}
	return &dto.PratinjauTransaksiResponse{
		Items: barisOut,
		Ringkasan: dto.PratinjauRingkasan{
			JumlahItem:      len(barisOut),
			TotalQtyDitagih: k.totalQtyDitagih,
			TotalQtyKeluar:  k.totalQtyKeluar,
			GrandTotal:      dto.FormatUang(k.hasil.GrandTotal),
			Total:           dto.FormatUang(k.hasil.Total),
			PPNNominal:      dto.FormatUang(k.hasil.PPNNominal),
			TotalAkhir:      dto.FormatUang(k.hasil.TotalAkhir),
		},
		SemuaStokCukup: k.semuaCukup,
		Peringatan:     peringatan,
	}, nil
}

// Buat menyimpan transaksi pending tanpa mengurangi stok (SC-03).
func (s *TransaksiService) Buat(ctx context.Context, req dto.TransaksiCreateRequest, salesID uint64, meta AuditMeta) (*dto.TransaksiResponse, error) {
	itemsReq, err := req.NormalisasiItems()
	if err != nil {
		return nil, fieldErr("items", err.Error())
	}
	area := strings.TrimSpace(req.Area)
	if area == "" {
		return nil, fieldErr("area", "wajib diisi")
	}

	k, err := s.hitungKeranjang(ctx, req.KodePelanggan, req.Tanggal, itemsReq,
		req.Disc1Persen, req.Disc2Persen, req.Disc3Persen, req.PPNPersen,
		hitungBarisOpts{StrictStock: true, StrictHarga: true},
	)
	if err != nil {
		return nil, err
	}

	nominal, err := dto.ParseUangOpsional(req.NominalDibayar, "nominal_dibayar")
	if err != nil {
		return nil, fieldErr("nominal_dibayar", err.Error())
	}
	if nominal.IsNegative() {
		return nil, fieldErr("nominal_dibayar", "tidak boleh negatif")
	}
	if nominal.GreaterThan(k.hasil.TotalAkhir) {
		return nil, domain.ErrKelebihanBayar
	}

	statusBayar, sisa, dibayar := domain.TurunkanStatusPembayaran(k.hasil.TotalAkhir, nominal)
	var tjt *datatypes.Date
	if statusBayar != domain.PembayaranLunas {
		if req.TanggalJatuhTempo == nil || strings.TrimSpace(*req.TanggalJatuhTempo) == "" {
			return nil, fieldErr("tanggal_jatuh_tempo", "wajib bila belum lunas")
		}
		jt, err := parseTanggalWajib(*req.TanggalJatuhTempo, "tanggal_jatuh_tempo")
		if err != nil {
			return nil, err
		}
		if !jt.After(k.tanggal) {
			return nil, fieldErr("tanggal_jatuh_tempo", "harus setelah tanggal transaksi")
		}
		d := datatypes.Date(jt)
		tjt = &d
	}

	alamat := strings.TrimSpace(k.plg.AlamatToko)
	if k.plg.AlamatPengantaranBarang != nil && strings.TrimSpace(*k.plg.AlamatPengantaranBarang) != "" {
		alamat = strings.TrimSpace(*k.plg.AlamatPengantaranBarang)
	}

	var ket *string
	if s := strings.TrimSpace(req.KeteranganPembayaran); s != "" {
		ket = &s
	}

	details := make([]domain.TransaksiDetail, 0, len(k.baris))
	for i, b := range k.baris {
		urutan := uint16(i + 1)
		d := domain.TransaksiDetail{
			Urutan:         urutan,
			KodeItem:       b.brg.KodeBarang,
			NamaItem:       b.brg.NamaItem,
			Satuan:         b.brg.Satuan,
			Qty:            b.Jumlah,
			QtyPromo:       b.qtyPromo,
			TotalQtyKeluar: b.TotalQtyKeluar,
			Harga:          money.RoundMoney(b.hargaDec),
			Subtotal:       money.RoundMoney(k.hasil.Items[i].Subtotal),
			Disc1Persen:    b.disc1,
			Disc2Persen:    b.disc2,
			Disc3Persen:    b.disc3,
			TotalAfterDisc: money.RoundMoney(k.hasil.Items[i].TotalAfterDisc),
			HPPSnapshot:    money.RoundMoney(b.hppSnapshot),
		}
		if len(b.alokasi) > 0 {
			id := b.alokasi[0].BarangMasukID
			bn := b.alokasi[0].NoBatch
			exp := datatypes.Date(b.alokasi[0].Exp)
			d.BarangMasukID = &id
			d.BatchNumber = &bn
			d.ExpiryDate = &exp
		}
		for _, p := range b.promoTerapan {
			d.Promos = append(d.Promos, domain.TransaksiDetailPromo{
				PromoID:     p.Promo.ID,
				KodePromo:   p.Promo.KodePromo,
				NamaPromo:   p.Promo.NamaPromo,
				TipePromo:   p.Promo.TipePromo,
				QtyBonus:    p.QtyBonus,
				NilaiDiskon: money.RoundMoney(p.NilaiDiskon),
			})
		}
		details = append(details, d)
	}

	sid := salesID
	trx := domain.TransaksiPenjualan{
		NoTransaksi:          nil,
		Tanggal:              datatypes.Date(k.tanggal),
		Periode:              k.tanggal.Format("2006-01"),
		KodePelanggan:        k.plg.KodePelanggan,
		NamaPelanggan:        k.plg.NamaPelanggan,
		Alamat:               alamat,
		ChannelOutlet:        k.plg.ChannelOutlet,
		Area:                 area,
		IsMultiItem:          len(details) > 1,
		Disc1Persen:          k.disc1G,
		Disc2Persen:          k.disc2G,
		Disc3Persen:          k.disc3G,
		PPNPersen:            k.ppn,
		Total:                k.hasil.Total,
		PPNNominal:           k.hasil.PPNNominal,
		TotalAkhir:           k.hasil.TotalAkhir,
		JumlahItem:           len(details),
		TotalQtyDitagih:      k.totalQtyDitagih,
		TotalQtyKeluar:       k.totalQtyKeluar,
		StatusApproval:       domain.ApprovalPending,
		FulfillmentStatus:    domain.FulfillmentAwaitingApproval,
		StatusPembayaran:     statusBayar,
		JumlahDibayar:        dibayar,
		SisaHutang:           sisa,
		TanggalJatuhTempo:    tjt,
		KeteranganPembayaran: ket,
		SalesID:              &sid,
		Details:              details,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.trx.CreatePending(tx, &trx); err != nil {
			return err
		}
		ringkas := fmt.Sprintf("buat transaksi pending #%d pelanggan %s", trx.ID, trx.KodePelanggan)
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "transaksi.buat", EntityType: "transaksi_penjualan",
			EntityID: &trx.ID, Ringkasan: &ringkas, DataSesudah: map[string]any{
				"id": trx.ID, "total_akhir": dto.FormatUang(trx.TotalAkhir),
				"status_approval": trx.StatusApproval, "sales_id": salesID,
			},
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
	if err != nil {
		return nil, err
	}

	loaded, err := s.trx.FindByID(s.db.WithContext(ctx), trx.ID)
	if err != nil {
		return nil, err
	}
	resp := mapTransaksiResponse(loaded)
	return &resp, nil
}

// TambahItems menambah baris ke transaksi pending dan menghitung ulang total dari nol (SC-04).
func (s *TransaksiService) TambahItems(
	ctx context.Context,
	id uint64,
	req dto.TransaksiAddItemsRequest,
	salesID uint64,
	meta AuditMeta,
) (*dto.TransaksiResponse, error) {
	itemsBaru, err := req.NormalisasiItems()
	if err != nil {
		return nil, fieldErr("items", err.Error())
	}

	db := s.db.WithContext(ctx)
	trx, err := s.trx.FindByID(db, id)
	if err != nil {
		return nil, err
	}
	if trx.SalesID == nil || *trx.SalesID != salesID {
		return nil, domain.ErrTidakDitemukan
	}
	if trx.StatusApproval != domain.ApprovalPending {
		return nil, domain.ErrStatusTidakValid
	}

	itemsGabungan := make([]dto.TransaksiItemRequest, 0, len(trx.Details)+len(itemsBaru))
	for _, d := range trx.Details {
		itemsGabungan = append(itemsGabungan, detailKeItemRequest(d))
	}
	itemsGabungan = append(itemsGabungan, itemsBaru...)

	tanggalStr := time.Time(trx.Tanggal).UTC().Format("2006-01-02")
	k, err := s.hitungKeranjang(ctx, trx.KodePelanggan, tanggalStr, itemsGabungan,
		dto.FormatUang(trx.Disc1Persen), dto.FormatUang(trx.Disc2Persen), dto.FormatUang(trx.Disc3Persen),
		dto.FormatUang(trx.PPNPersen),
		hitungBarisOpts{StrictStock: true, StrictHarga: true},
	)
	if err != nil {
		return nil, err
	}

	details := make([]domain.TransaksiDetail, 0, len(k.baris))
	for i, b := range k.baris {
		urutan := uint16(i + 1)
		d := domain.TransaksiDetail{
			Urutan:         urutan,
			KodeItem:       b.brg.KodeBarang,
			NamaItem:       b.brg.NamaItem,
			Satuan:         b.brg.Satuan,
			Qty:            b.Jumlah,
			QtyPromo:       b.qtyPromo,
			TotalQtyKeluar: b.TotalQtyKeluar,
			Harga:          money.RoundMoney(b.hargaDec),
			Subtotal:       money.RoundMoney(k.hasil.Items[i].Subtotal),
			Disc1Persen:    b.disc1,
			Disc2Persen:    b.disc2,
			Disc3Persen:    b.disc3,
			TotalAfterDisc: money.RoundMoney(k.hasil.Items[i].TotalAfterDisc),
			HPPSnapshot:    money.RoundMoney(b.hppSnapshot),
		}
		if len(b.alokasi) > 0 {
			bid := b.alokasi[0].BarangMasukID
			bn := b.alokasi[0].NoBatch
			exp := datatypes.Date(b.alokasi[0].Exp)
			d.BarangMasukID = &bid
			d.BatchNumber = &bn
			d.ExpiryDate = &exp
		}
		for _, p := range b.promoTerapan {
			d.Promos = append(d.Promos, domain.TransaksiDetailPromo{
				PromoID:     p.Promo.ID,
				KodePromo:   p.Promo.KodePromo,
				NamaPromo:   p.Promo.NamaPromo,
				TipePromo:   p.Promo.TipePromo,
				QtyBonus:    p.QtyBonus,
				NilaiDiskon: money.RoundMoney(p.NilaiDiskon),
			})
		}
		details = append(details, d)
	}

	trx.IsMultiItem = len(details) > 1
	trx.Total = k.hasil.Total
	trx.PPNNominal = k.hasil.PPNNominal
	trx.JumlahItem = len(details)
	trx.TotalQtyDitagih = k.totalQtyDitagih
	trx.TotalQtyKeluar = k.totalQtyKeluar
	hasilBayar := trx.SesuaikanPembayaran(k.hasil.TotalAkhir)

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := s.trx.ReplaceDetailsAndHeader(tx, trx, details); err != nil {
			return err
		}
		if hasilBayar.TulisRiwayat && s.bayar != nil {
			now := s.clock.Now().UTC()
			ket := domain.KeteranganPenyesuaianTotal
			oldSt := hasilBayar.OldStatus
			newSt := hasilBayar.NewStatus
			uid := salesID
			row := domain.RiwayatPembayaran{
				TransaksiPenjualanID: trx.ID,
				OldJumlahDibayar:     hasilBayar.OldJumlahDibayar,
				NewJumlahDibayar:     hasilBayar.NewJumlahDibayar,
				OldSisaHutang:        hasilBayar.OldSisaHutang,
				NewSisaHutang:        hasilBayar.NewSisaHutang,
				OldStatus:            &oldSt,
				NewStatus:            &newSt,
				NominalPembayaran:    hasilBayar.NominalDelta,
				Keterangan:           &ket,
				ChangedBy:            &uid,
				ChangedAt:            now,
				CreatedAt:            now,
			}
			if err := s.bayar.Create(tx, &row); err != nil {
				return err
			}
		}
		ringkas := fmt.Sprintf("tambah item transaksi pending #%d → %d baris, total_akhir=%s",
			trx.ID, len(details), dto.FormatUang(trx.TotalAkhir))
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "transaksi.tambah_items", EntityType: "transaksi_penjualan",
			EntityID: &trx.ID, Ringkasan: &ringkas, DataSesudah: map[string]any{
				"id": trx.ID, "jumlah_item": trx.JumlahItem,
				"total": dto.FormatUang(trx.Total), "total_akhir": dto.FormatUang(trx.TotalAkhir),
			},
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
	if err != nil {
		return nil, err
	}

	loaded, err := s.trx.FindByID(s.db.WithContext(ctx), trx.ID)
	if err != nil {
		return nil, err
	}
	resp := mapTransaksiResponse(loaded)
	return &resp, nil
}

func detailKeItemRequest(d domain.TransaksiDetail) dto.TransaksiItemRequest {
	kodePromos := make([]string, 0, len(d.Promos))
	for _, p := range d.Promos {
		kodePromos = append(kodePromos, p.KodePromo)
	}
	return dto.TransaksiItemRequest{
		KodeItem:      d.KodeItem,
		Qty:           d.Qty,
		Harga:         dto.FormatUang(d.Harga),
		Disc1Persen:   dto.FormatUang(d.Disc1Persen),
		Disc2Persen:   dto.FormatUang(d.Disc2Persen),
		Disc3Persen:   dto.FormatUang(d.Disc3Persen),
		KodePromos:    kodePromos,
		BarangMasukID: d.BarangMasukID,
	}
}

// Detail mengembalikan transaksi; sales hanya miliknya (SC-03 / SC-07).
func (s *TransaksiService) Detail(ctx context.Context, id, userID uint64, role domain.Role) (*dto.TransaksiResponse, error) {
	trx, err := s.trx.FindByID(s.db.WithContext(ctx), id)
	if err != nil {
		return nil, err
	}
	if role.Punya(domain.PermTransaksiLihatSemua) {
		resp := mapTransaksiResponse(trx)
		return &resp, nil
	}
	if role.Punya(domain.PermTransaksiLihatMilik) {
		if trx.SalesID == nil || *trx.SalesID != userID {
			return nil, domain.ErrTidakDitemukan
		}
		resp := mapTransaksiResponse(trx)
		return &resp, nil
	}
	return nil, domain.ErrTidakDiizinkan
}

// Daftar transaksi dengan isolasi sales (SC-07).
func (s *TransaksiService) Daftar(
	ctx context.Context,
	q dto.TransaksiListQuery,
	userID uint64,
	role domain.Role,
) ([]dto.TransaksiListItem, dto.PageMeta, error) {
	if role.Punya(domain.PermTransaksiLihatSemua) {
		// optional sales_id filter from query
	} else if role.Punya(domain.PermTransaksiLihatMilik) {
		sid := userID
		q.SalesID = &sid
	} else {
		return nil, dto.PageMeta{}, domain.ErrTidakDiizinkan
	}

	db := s.db.WithContext(ctx)
	hasil, err := s.trx.List(db, q)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	out := make([]dto.TransaksiListItem, 0, len(hasil.Items))
	pendingIDs := make([]uint64, 0)
	for i := range hasil.Items {
		item := mapTransaksiListItem(&hasil.Items[i])
		if hasil.Items[i].StatusApproval == domain.ApprovalPending {
			pendingIDs = append(pendingIDs, hasil.Items[i].ID)
		}
		out = append(out, item)
	}
	if len(pendingIDs) > 0 {
		cukupMap, err := s.trx.HitungKecukupanStok(db, pendingIDs)
		if err != nil {
			return nil, dto.PageMeta{}, err
		}
		for i := range out {
			if v, ok := cukupMap[out[i].ID]; ok {
				b := v
				out[i].StokCukup = &b
			}
		}
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), nil
}

func mapTransaksiListItem(trx *domain.TransaksiPenjualan) dto.TransaksiListItem {
	return dto.TransaksiListItem{
		ID: trx.ID, NoTransaksi: trx.NoTransaksi,
		Tanggal:       time.Time(trx.Tanggal).UTC().Format("2006-01-02"),
		KodePelanggan: trx.KodePelanggan, NamaPelanggan: trx.NamaPelanggan,
		ChannelOutlet: string(trx.ChannelOutlet), Area: trx.Area,
		IsMultiItem: trx.IsMultiItem, JumlahItem: trx.JumlahItem,
		TotalQtyDitagih: trx.TotalQtyDitagih, TotalQtyKeluar: trx.TotalQtyKeluar,
		Total: dto.FormatUang(trx.Total), PPNNominal: dto.FormatUang(trx.PPNNominal),
		TotalAkhir: dto.FormatUang(trx.TotalAkhir),
		StatusApproval: string(trx.StatusApproval), StatusPembayaran: string(trx.StatusPembayaran),
		SalesID: trx.SalesID, CreatedAt: trx.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *TransaksiService) hitungKeranjang(
	ctx context.Context,
	kodePelanggan, tanggalStr string,
	itemsReq []dto.TransaksiItemRequest,
	disc1S, disc2S, disc3S, ppnS string,
	opts hitungBarisOpts,
) (*keranjangHasil, error) {
	if strings.TrimSpace(kodePelanggan) == "" {
		return nil, fieldErr("kode_pelanggan", "wajib diisi")
	}
	tanggal, err := parseTanggalWajib(tanggalStr, "tanggal")
	if err != nil {
		return nil, err
	}

	db := s.db.WithContext(ctx)
	plg, err := s.pelanggan.FindByKode(db, strings.TrimSpace(kodePelanggan))
	if err != nil {
		return nil, err
	}
	if !plg.IsActive {
		return nil, fieldErr("kode_pelanggan", "pelanggan tidak aktif")
	}

	disc1G, err := parsePersenOpsional(disc1S, "disc1_persen")
	if err != nil {
		return nil, err
	}
	disc2G, err := parsePersenOpsional(disc2S, "disc2_persen")
	if err != nil {
		return nil, err
	}
	disc3G, err := parsePersenOpsional(disc3S, "disc3_persen")
	if err != nil {
		return nil, err
	}
	ppn := s.ppnDefault
	if strings.TrimSpace(ppnS) != "" {
		ppn, err = parsePersenOpsional(ppnS, "ppn_persen")
		if err != nil {
			return nil, err
		}
	}

	hariIni := s.clock.Now().UTC()
	peringatan := make([]string, 0)
	baris := make([]barisPratinjauHasil, 0, len(itemsReq))
	calcItems := make([]domain.ItemInput, 0, len(itemsReq))
	semuaCukup := true
	totalQtyDitagih := 0
	totalQtyKeluar := 0

	for i, it := range itemsReq {
		prefix := fmt.Sprintf("items[%d]", i)
		hasilBaris, warn, err := s.hitungBarisPratinjau(db, it, plg.ChannelOutlet, tanggal, hariIni, prefix, opts)
		if err != nil {
			return nil, err
		}
		peringatan = append(peringatan, warn...)
		if !hasilBaris.StokCukup {
			semuaCukup = false
			if opts.StrictStock {
				return nil, domain.ErrStokTidakCukup
			}
		}
		totalQtyDitagih += hasilBaris.Jumlah
		totalQtyKeluar += hasilBaris.TotalQtyKeluar
		calcItems = append(calcItems, domain.ItemInput{
			Qty:         hasilBaris.Jumlah,
			Harga:       hasilBaris.hargaDec,
			Disc1:       hasilBaris.disc1,
			Disc2:       hasilBaris.disc2,
			Disc3:       hasilBaris.disc3,
			DiskonPromo: hasilBaris.diskonPromoDec,
		})
		baris = append(baris, hasilBaris)
	}

	hasil := domain.BulatkanHasilTransaksi(domain.HitungTotalTransaksi(domain.TransaksiInput{
		Items:       calcItems,
		Disc1Global: disc1G,
		Disc2Global: disc2G,
		Disc3Global: disc3G,
		PPNPersen:   ppn,
	}))

	return &keranjangHasil{
		plg: plg, tanggal: tanggal,
		disc1G: disc1G, disc2G: disc2G, disc3G: disc3G, ppn: ppn,
		baris: baris, calcItems: calcItems, hasil: hasil,
		totalQtyDitagih: totalQtyDitagih, totalQtyKeluar: totalQtyKeluar,
		semuaCukup: semuaCukup, peringatan: peringatan,
	}, nil
}

type barisPratinjauHasil struct {
	resp                dto.PratinjauItemResponse
	brg                 *domain.Barang
	hargaDec            decimal.Decimal
	hppSnapshot         decimal.Decimal
	disc1, disc2, disc3 decimal.Decimal
	diskonPromoDec      decimal.Decimal
	qtyPromo            int
	Jumlah              int
	TotalQtyKeluar      int
	StokCukup           bool
	alokasi             []domain.AlokasiBatchHasil
	promoTerapan        []domain.PromoTerapan
}

func (s *TransaksiService) hitungBarisPratinjau(
	db *gorm.DB,
	it dto.TransaksiItemRequest,
	channel domain.ChannelOutlet,
	tanggalTrx, hariIni time.Time,
	prefix string,
	opts hitungBarisOpts,
) (barisPratinjauHasil, []string, error) {
	warn := make([]string, 0)
	kode := strings.TrimSpace(it.KodeItem)
	brg, err := s.barang.FindByKode(db, kode)
	if err != nil {
		return barisPratinjauHasil{}, nil, err
	}
	if !brg.IsActive {
		return barisPratinjauHasil{}, nil, fieldErr(prefix+".kode_item", "barang tidak aktif")
	}

	disc1, err := parsePersenOpsional(it.Disc1Persen, prefix+".disc1_persen")
	if err != nil {
		return barisPratinjauHasil{}, nil, err
	}
	disc2, err := parsePersenOpsional(it.Disc2Persen, prefix+".disc2_persen")
	if err != nil {
		return barisPratinjauHasil{}, nil, err
	}
	disc3, err := parsePersenOpsional(it.Disc3Persen, prefix+".disc3_persen")
	if err != nil {
		return barisPratinjauHasil{}, nil, err
	}

	kandidat, stokSKU, err := s.kandidatBatch(db, brg, it.BarangMasukID, hariIni)
	if err != nil {
		return barisPratinjauHasil{}, nil, err
	}

	hargaServer, hppSnapshot, err := hargaDariKandidat(kandidat, channel)
	if err != nil {
		return barisPratinjauHasil{}, nil, fieldErr(prefix+".kode_item", err.Error())
	}

	if strings.TrimSpace(it.Harga) != "" {
		hargaClient, perr := dto.ParseUang(it.Harga, prefix+".harga")
		if perr != nil {
			return barisPratinjauHasil{}, nil, fieldErr(prefix+".harga", perr.Error())
		}
		if !hargaClient.Equal(hargaServer) {
			if opts.StrictHarga {
				return barisPratinjauHasil{}, nil, domain.ErrHargaTidakSesuai
			}
			warn = append(warn, fmt.Sprintf(
				"%s: harga client %s diganti harga channel server %s",
				prefix, dto.FormatUang(hargaClient), dto.FormatUang(hargaServer),
			))
		}
	} else if opts.StrictHarga {
		return barisPratinjauHasil{}, nil, fieldErr(prefix+".harga", "wajib diisi")
	}

	promos, err := s.muatPromoByKode(db, it.KodePromos, prefix)
	if err != nil {
		return barisPratinjauHasil{}, nil, err
	}

	subtotalSementara := decimal.NewFromInt(int64(it.Qty)).Mul(hargaServer)
	afterDisc := domain.DiskonBerjenjang(subtotalSementara, disc1, disc2, disc3)
	promoHasil := domain.HitungPromoBaris(promos, kode, it.Qty, afterDisc, tanggalTrx)
	qtyPromo := promoHasil.QtyBonus
	totalKeluar := it.Qty + qtyPromo

	alokasi, stokCukup := alokasiSoft(kandidat, totalKeluar, brg.MetodeAlokasi, channel, hariIni)
	alokasiResp := make([]dto.AlokasiBatchResponse, 0, len(alokasi))
	for _, a := range alokasi {
		exp := a.Exp.UTC().Format("2006-01-02")
		alokasiResp = append(alokasiResp, dto.AlokasiBatchResponse{
			BarangMasukID: a.BarangMasukID,
			NoBatch:       a.NoBatch,
			Exp:           &exp,
			Qty:           a.Qty,
		})
	}
	if len(alokasi) > 0 {
		hppSnapshot = alokasi[0].HPPSnapshot
	}

	promoResp := make([]dto.PromoTerapanResponse, 0, len(promoHasil.Diterapkan))
	for _, d := range promoHasil.Diterapkan {
		id := d.Promo.ID
		promoResp = append(promoResp, dto.PromoTerapanResponse{
			PromoID:     &id,
			KodePromo:   d.Promo.KodePromo,
			NamaPromo:   d.Promo.NamaPromo,
			TipePromo:   string(d.Promo.TipePromo),
			QtyBonus:    d.QtyBonus,
			NilaiDiskon: dto.FormatUang(d.NilaiDiskon),
		})
	}

	itemResp := dto.TransaksiItemResponse{
		Urutan:          1,
		KodeItem:        brg.KodeBarang,
		NamaItem:        brg.NamaItem,
		Satuan:          brg.Satuan,
		Jumlah:          it.Qty,
		QtyPromo:        qtyPromo,
		TotalQtyKeluar:  totalKeluar,
		Harga:           dto.FormatUang(hargaServer),
		Disc1Persen:     dto.FormatUang(disc1),
		Disc2Persen:     dto.FormatUang(disc2),
		Disc3Persen:     dto.FormatUang(disc3),
		HPPSnapshot:     dto.FormatUang(hppSnapshot),
		PromoDiterapkan: promoResp,
	}
	if len(alokasi) == 1 {
		id := alokasi[0].BarangMasukID
		bn := alokasi[0].NoBatch
		exp := alokasi[0].Exp.UTC().Format("2006-01-02")
		itemResp.BarangMasukID = &id
		itemResp.BatchNumber = &bn
		itemResp.ExpiryDate = &exp
	}

	return barisPratinjauHasil{
		resp: dto.PratinjauItemResponse{
			TransaksiItemResponse: itemResp,
			DiskonPromo:           dto.FormatUang(promoHasil.NilaiDiskon),
			AlokasiBatch:          alokasiResp,
			StokCukup:             stokCukup,
			StokTersedia:          stokSKU,
		},
		brg:            brg,
		hargaDec:       hargaServer,
		hppSnapshot:    hppSnapshot,
		disc1:          disc1,
		disc2:          disc2,
		disc3:          disc3,
		diskonPromoDec: promoHasil.NilaiDiskon,
		qtyPromo:       qtyPromo,
		Jumlah:         it.Qty,
		TotalQtyKeluar: totalKeluar,
		StokCukup:      stokCukup,
		alokasi:        alokasi,
		promoTerapan:   promoHasil.Diterapkan,
	}, warn, nil
}

func mapTransaksiResponse(trx *domain.TransaksiPenjualan) dto.TransaksiResponse {
	items := make([]dto.TransaksiItemResponse, 0, len(trx.Details))
	for _, d := range trx.Details {
		promos := make([]dto.PromoTerapanResponse, 0, len(d.Promos))
		for _, p := range d.Promos {
			id := p.PromoID
			promos = append(promos, dto.PromoTerapanResponse{
				PromoID:     &id,
				KodePromo:   p.KodePromo,
				NamaPromo:   p.NamaPromo,
				TipePromo:   string(p.TipePromo),
				QtyBonus:    p.QtyBonus,
				NilaiDiskon: dto.FormatUang(p.NilaiDiskon),
			})
		}
		var exp *string
		if d.ExpiryDate != nil {
			s := time.Time(*d.ExpiryDate).UTC().Format("2006-01-02")
			exp = &s
		}
		items = append(items, dto.TransaksiItemResponse{
			ID: d.ID, Urutan: d.Urutan,
			KodeItem: d.KodeItem, NamaItem: d.NamaItem, Satuan: d.Satuan,
			BarangMasukID: d.BarangMasukID, BatchNumber: d.BatchNumber, ExpiryDate: exp,
			Jumlah: d.Qty, QtyPromo: d.QtyPromo, TotalQtyKeluar: d.TotalQtyKeluar,
			Harga: dto.FormatUang(d.Harga), Subtotal: dto.FormatUang(d.Subtotal),
			Disc1Persen: dto.FormatUang(d.Disc1Persen), Disc2Persen: dto.FormatUang(d.Disc2Persen),
			Disc3Persen: dto.FormatUang(d.Disc3Persen), TotalAfterDisc: dto.FormatUang(d.TotalAfterDisc),
			HPPSnapshot: dto.FormatUang(d.HPPSnapshot), PromoDiterapkan: promos,
		})
	}

	var tjt *string
	if trx.TanggalJatuhTempo != nil {
		s := time.Time(*trx.TanggalJatuhTempo).UTC().Format("2006-01-02")
		tjt = &s
	}
	var sales *dto.UserRingkasResponse
	if trx.SalesID != nil {
		sales = &dto.UserRingkasResponse{ID: *trx.SalesID}
	}

	return dto.TransaksiResponse{
		ID: trx.ID, NoTransaksi: trx.NoTransaksi,
		Tanggal:       time.Time(trx.Tanggal).UTC().Format("2006-01-02"),
		Periode:       trx.Periode,
		KodePelanggan: trx.KodePelanggan, NamaPelanggan: trx.NamaPelanggan,
		Alamat: trx.Alamat, ChannelOutlet: string(trx.ChannelOutlet), Area: trx.Area,
		IsMultiItem: trx.IsMultiItem,
		Disc1Persen: dto.FormatUang(trx.Disc1Persen), Disc2Persen: dto.FormatUang(trx.Disc2Persen),
		Disc3Persen: dto.FormatUang(trx.Disc3Persen), PPNPersen: dto.FormatUang(trx.PPNPersen),
		Total: dto.FormatUang(trx.Total), PPNNominal: dto.FormatUang(trx.PPNNominal),
		TotalAkhir: dto.FormatUang(trx.TotalAkhir),
		JumlahItem: trx.JumlahItem, TotalQtyDitagih: trx.TotalQtyDitagih, TotalQtyKeluar: trx.TotalQtyKeluar,
		StatusApproval: string(trx.StatusApproval), FulfillmentStatus: string(trx.FulfillmentStatus),
		StatusPembayaran: string(trx.StatusPembayaran),
		JumlahDibayar:    dto.FormatUang(trx.JumlahDibayar), SisaHutang: dto.FormatUang(trx.SisaHutang),
		TanggalJatuhTempo: tjt, KeteranganPembayaran: trx.KeteranganPembayaran,
		Sales: sales, Items: items,
		CreatedAt: trx.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: trx.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *TransaksiService) kandidatBatch(
	db *gorm.DB,
	brg *domain.Barang,
	paksaID *uint64,
	hariIni time.Time,
) ([]domain.BatchKandidat, int, error) {
	saldo, err := s.stock.ListBatchSaldo(db, brg.ID)
	if err != nil {
		return nil, 0, err
	}
	hari := dayUTC(hariIni)
	kandidat := make([]domain.BatchKandidat, 0)
	stokSKU := 0
	for _, row := range saldo {
		if row.QtyTersedia <= 0 {
			continue
		}
		exp := time.Time(row.Exp)
		if dayUTC(exp).Before(hari) {
			continue
		}
		if paksaID != nil && row.ID != *paksaID {
			continue
		}
		stokSKU += row.QtyTersedia
		kandidat = append(kandidat, domain.BatchKandidat{
			BarangMasukID: row.ID,
			NoBatch:       row.NoBatch,
			Exp:           exp,
			TanggalMasuk:  time.Time(row.TanggalMasuk),
			QtyTersedia:   row.QtyTersedia,
			HPPDenganPPN:  row.HPPDenganPPN,
			HargaMT:       row.HargaMT,
			HargaGT:       row.HargaGT,
		})
	}
	if paksaID != nil && len(kandidat) == 0 {
		return nil, 0, fieldErr("barang_masuk_id", "batch tidak ditemukan, kedaluwarsa, atau tanpa saldo")
	}
	return kandidat, stokSKU, nil
}

func (s *TransaksiService) muatPromoByKode(db *gorm.DB, kode []string, prefix string) ([]domain.Promo, error) {
	if len(kode) == 0 {
		return nil, nil
	}
	out := make([]domain.Promo, 0, len(kode))
	seen := map[string]struct{}{}
	for i, k := range kode {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		p, err := s.promo.FindByKode(db, k)
		if err != nil {
			if errors.Is(err, domain.ErrTidakDitemukan) {
				return nil, fieldErr(fmt.Sprintf("%s.kode_promos[%d]", prefix, i), "kode promo tidak ditemukan")
			}
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

func hargaDariKandidat(kandidat []domain.BatchKandidat, channel domain.ChannelOutlet) (harga, hpp decimal.Decimal, err error) {
	if len(kandidat) == 0 {
		return decimal.Zero, decimal.Zero, fmt.Errorf("tidak ada batch tersedia untuk menentukan harga channel")
	}
	best := kandidat[0]
	for _, k := range kandidat[1:] {
		if k.Exp.Before(best.Exp) {
			best = k
		}
	}
	return domain.PilihHarga(best.HargaMT, best.HargaGT, channel), best.HPPDenganPPN, nil
}

func alokasiSoft(
	kandidat []domain.BatchKandidat,
	qty int,
	metode domain.MetodeAlokasi,
	channel domain.ChannelOutlet,
	hariIni time.Time,
) ([]domain.AlokasiBatchHasil, bool) {
	alokasi, err := domain.AlokasiBatch(kandidat, qty, metode, channel, hariIni)
	if err != nil {
		return nil, false
	}
	return alokasi, true
}

func parseTanggalWajib(s, field string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fieldErr(field, "wajib diisi")
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return time.Time{}, fieldErr(field, "format tanggal harus YYYY-MM-DD")
	}
	return t, nil
}

func parsePersenOpsional(s, field string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fieldErr(field, "tidak valid")
	}
	if d.IsNegative() || d.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero, fieldErr(field, "harus antara 0 dan 100")
	}
	return d, nil
}

func fieldErr(field, msg string) error {
	return &appvalidator.Errors{Fields: []appvalidator.FieldError{{Field: field, Message: msg}}}
}
