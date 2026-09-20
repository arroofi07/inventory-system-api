package service

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	appvalidator "app/internal/pkg/validator"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// BarangService adalah contoh pola CRUD SB-01 (list/detail/create/update/status + audit)
// plus batch FEFO (SB-04) dan harga massal (SB-05).
type BarangService struct {
	db     *gorm.DB
	barang *repository.BarangRepo
	bm     *repository.BarangMasukRepo
	stock  *repository.StockRepo
	harga  *repository.PriceChangeRepo
	audit  *repository.AuditRepo
	clock  clock.Clock
	ppn    decimal.Decimal
}

func NewBarangService(
	db *gorm.DB,
	barang *repository.BarangRepo,
	bm *repository.BarangMasukRepo,
	stock *repository.StockRepo,
	harga *repository.PriceChangeRepo,
	audit *repository.AuditRepo,
	clk clock.Clock,
	ppnPersen decimal.Decimal,
) *BarangService {
	if clk == nil {
		clk = clock.Real{}
	}
	return &BarangService{
		db: db, barang: barang, bm: bm, stock: stock, harga: harga,
		audit: audit, clock: clk, ppn: ppnPersen,
	}
}

type AuditMeta struct {
	UserID    *uint64
	IPAddress *string
	RequestID *string
}

func (s *BarangService) Daftar(ctx context.Context, q dto.BarangListQuery) ([]dto.BarangResponse, dto.PageMeta, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, dto.PageMeta{}, err
	}
	q.Normalize()
	hasil, err := s.barang.List(s.db.WithContext(ctx), q)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	out := make([]dto.BarangResponse, 0, len(hasil.Items))
	for i := range hasil.Items {
		out = append(out, mapBarang(&hasil.Items[i]))
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), nil
}

func (s *BarangService) Detail(ctx context.Context, id uint64) (*dto.BarangResponse, error) {
	b, err := s.barang.FindByID(s.db.WithContext(ctx), id)
	if err != nil {
		return nil, err
	}
	resp := mapBarang(b)
	return &resp, nil
}

func (s *BarangService) DetailByKode(ctx context.Context, kode string) (*dto.BarangResponse, error) {
	b, err := s.barang.FindByKode(s.db.WithContext(ctx), kode)
	if err != nil {
		return nil, err
	}
	resp := mapBarang(b)
	return &resp, nil
}

func (s *BarangService) Buat(ctx context.Context, req dto.BarangCreateRequest, meta AuditMeta) (*dto.BarangResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	b := domain.Barang{
		KodeBarang:      strings.TrimSpace(req.KodeBarang),
		NamaItem:        strings.TrimSpace(req.NamaItem),
		Brand:           strings.TrimSpace(req.Brand),
		Satuan:          "PCS",
		MetodeAlokasi:   domain.AlokasiFEFO,
		ExpiryAlertDays: 30,
		IsActive:        true,
	}
	if req.Satuan != "" {
		b.Satuan = strings.TrimSpace(req.Satuan)
	}
	if req.MinStock != nil {
		b.MinStock = *req.MinStock
	}
	if req.ReorderPoint != nil {
		b.ReorderPoint = *req.ReorderPoint
	}
	if req.ExpiryAlertDays != nil {
		b.ExpiryAlertDays = *req.ExpiryAlertDays
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.barang.Create(tx, &b); err != nil {
			return err
		}
		ringkas := "buat master barang " + b.KodeBarang
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "barang.buat", EntityType: "barang",
			EntityID: &b.ID, Ringkasan: &ringkas, DataSesudah: b,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
	if err != nil {
		return nil, err
	}
	resp := mapBarang(&b)
	return &resp, nil
}

func (s *BarangService) Ubah(ctx context.Context, id uint64, req dto.BarangUpdateRequest, meta AuditMeta) (*dto.BarangResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	var out *dto.BarangResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.barang.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := *sebelum
		if req.NamaItem != nil {
			sebelum.NamaItem = strings.TrimSpace(*req.NamaItem)
		}
		if req.Brand != nil {
			sebelum.Brand = strings.TrimSpace(*req.Brand)
		}
		if req.Satuan != nil {
			sebelum.Satuan = strings.TrimSpace(*req.Satuan)
		}
		if req.MinStock != nil {
			sebelum.MinStock = *req.MinStock
		}
		if req.ReorderPoint != nil {
			sebelum.ReorderPoint = *req.ReorderPoint
		}
		if req.MetodeAlokasi != nil {
			sebelum.MetodeAlokasi = domain.AlokasiFEFO
		}
		if req.ExpiryAlertDays != nil {
			sebelum.ExpiryAlertDays = *req.ExpiryAlertDays
		}
		if err := s.barang.Update(tx, sebelum); err != nil {
			return err
		}
		ringkas := "ubah master barang " + sebelum.KodeBarang
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "barang.ubah", EntityType: "barang",
			EntityID: &sebelum.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: *sebelum,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp := mapBarang(sebelum)
		out = &resp
		return nil
	})
	return out, err
}

func (s *BarangService) SetStatus(ctx context.Context, id uint64, aktif bool, meta AuditMeta) (*dto.BarangResponse, error) {
	var out *dto.BarangResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.barang.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := *sebelum
		if err := s.barang.SetActive(tx, id, aktif); err != nil {
			return err
		}
		sebelum.IsActive = aktif
		aksi := "barang.nonaktif"
		if aktif {
			aksi = "barang.aktif"
		}
		ringkas := aksi + " " + sebelum.KodeBarang
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: aksi, EntityType: "barang",
			EntityID: &id, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: *sebelum,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp := mapBarang(sebelum)
		out = &resp
		return nil
	})
	return out, err
}

// BatchTersedia mengembalikan batch bersaldo urut FEFO/FIFO + opsional rencana_alokasi.
func (s *BarangService) BatchTersedia(ctx context.Context, kode string, q dto.BatchTersediaQuery) (*dto.BatchTersediaResponse, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	b, err := s.barang.FindByKode(db, kode)
	if err != nil {
		return nil, err
	}
	saldo, err := s.stock.ListBatchSaldo(db, b.ID)
	if err != nil {
		return nil, err
	}

	hariIni := s.clock.Now().UTC()
	channel := domain.ChannelOutlet(q.Channel)
	if channel == "" {
		channel = domain.ChannelGeneralTrade
	}

	items := make([]dto.BatchTersediaItem, 0)
	kandidat := make([]domain.BatchKandidat, 0)
	for _, row := range saldo {
		if row.QtyTersedia <= 0 {
			continue
		}
		exp := time.Time(row.Exp)
		if dayUTC(exp).Before(dayUTC(hariIni)) {
			continue
		}
		sisa := int(dayUTC(exp).Sub(dayUTC(hariIni)).Hours() / 24)
		mendekati := sisa <= b.ExpiryAlertDays
		hargaJual := domain.HargaUntukChannel(row.BarangMasuk, channel)
		items = append(items, dto.BatchTersediaItem{
			BarangMasukID: row.ID,
			NoBatch:       row.NoBatch,
			NoFaktur:      row.NoFaktur,
			Exp:           formatDate(row.Exp),
			TanggalMasuk:  formatDate(row.TanggalMasuk),
			SisaHari:      sisa,
			QtyMasuk:      row.Qty,
			QtyTersedia:   row.QtyTersedia,
			HargaJual:     dto.FormatUang(hargaJual),
			HargaMT:       dto.FormatUang(row.HargaMT),
			HargaGT:       dto.FormatUang(row.HargaGT),
			HPPDenganPPN:  dto.FormatUang(row.HPPDenganPPN),
			MendekatiExp:  mendekati,
		})
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

	sortBatchTersedia(items, b.MetodeAlokasi)

	resp := &dto.BatchTersediaResponse{
		KodeBarang:    b.KodeBarang,
		NamaItem:      b.NamaItem,
		Satuan:        b.Satuan,
		MetodeAlokasi: string(b.MetodeAlokasi),
		StokTersedia:  b.StokTersedia,
		Batch:         items,
	}

	if q.Qty != nil {
		alokasi, err := domain.AlokasiBatch(kandidat, *q.Qty, b.MetodeAlokasi, channel, hariIni)
		if err != nil {
			return nil, err
		}
		rencana := make([]dto.RencanaAlokasiItem, 0, len(alokasi))
		for _, a := range alokasi {
			rencana = append(rencana, dto.RencanaAlokasiItem{
				BarangMasukID: a.BarangMasukID,
				NoBatch:       a.NoBatch,
				Exp:           a.Exp.UTC().Format("2006-01-02"),
				Qty:           a.Qty,
			})
		}
		resp.RencanaAlokasi = rencana
	}
	return resp, nil
}

// DaftarBatch semua batch SKU; limit=1 → batch terbaru (tanggal_masuk DESC).
func (s *BarangService) DaftarBatch(ctx context.Context, kode string, q dto.BatchListQuery) ([]dto.BatchListItem, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	b, err := s.barang.FindByKode(db, kode)
	if err != nil {
		return nil, err
	}
	saldo, err := s.stock.ListBatchSaldo(db, b.ID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.BatchListItem, 0, len(saldo))
	for _, row := range saldo {
		out = append(out, dto.BatchListItem{
			BarangMasukID: row.ID,
			NoBatch:       row.NoBatch,
			NoFaktur:      row.NoFaktur,
			Exp:           formatDate(row.Exp),
			TanggalMasuk:  formatDate(row.TanggalMasuk),
			QtyMasuk:      row.Qty,
			QtyTersedia:   row.QtyTersedia,
			Harga:         dto.FormatUang(row.Harga),
			HargaMT:       dto.FormatUang(row.HargaMT),
			HargaGT:       dto.FormatUang(row.HargaGT),
			HPP:           dto.FormatUang(row.HPP),
			HPPDenganPPN:  dto.FormatUang(row.HPPDenganPPN),
		})
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func sortBatchTersedia(items []dto.BatchTersediaItem, metode domain.MetodeAlokasi) {
	slices.SortFunc(items, func(a, b dto.BatchTersediaItem) int {
		switch metode {
		case domain.AlokasiFIFO:
			if c := cmp.Compare(a.TanggalMasuk, b.TanggalMasuk); c != 0 {
				return c
			}
			return cmp.Compare(a.BarangMasukID, b.BarangMasukID)
		default:
			if c := cmp.Compare(a.Exp, b.Exp); c != 0 {
				return c
			}
			if c := cmp.Compare(a.TanggalMasuk, b.TanggalMasuk); c != 0 {
				return c
			}
			return cmp.Compare(a.BarangMasukID, b.BarangMasukID)
		}
	})
}

func dayUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func mapBarang(b *domain.Barang) dto.BarangResponse {
	return dto.BarangResponse{
		ID:              b.ID,
		KodeBarang:      b.KodeBarang,
		NamaItem:        b.NamaItem,
		Brand:           b.Brand,
		Satuan:          b.Satuan,
		StokTersedia:    b.StokTersedia,
		MinStock:        b.MinStock,
		ReorderPoint:    b.ReorderPoint,
		StatusStok:      b.StatusStok(),
		MetodeAlokasi:   string(b.MetodeAlokasi),
		ExpiryAlertDays: b.ExpiryAlertDays,
		IsActive:        b.IsActive,
	}
}
