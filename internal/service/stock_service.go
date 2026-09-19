package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/repository"
	"gorm.io/gorm"
)

// StockService ledger pergerakan & penyesuaian (SC-13).
type StockService struct {
	db     *gorm.DB
	barang *repository.BarangRepo
	stock  *repository.StockRepo
	audit  *repository.AuditRepo
	clock  clock.Clock
}

func NewStockService(
	db *gorm.DB,
	barang *repository.BarangRepo,
	stock *repository.StockRepo,
	audit *repository.AuditRepo,
	clk clock.Clock,
) *StockService {
	return &StockService{db: db, barang: barang, stock: stock, audit: audit, clock: clk}
}

// DaftarPergerakan kartu stok dari ledger.
func (s *StockService) DaftarPergerakan(
	ctx context.Context,
	q dto.PergerakanStokListQuery,
) ([]dto.PergerakanStokItemResponse, dto.PageMeta, dto.PergerakanStokRingkasan, error) {
	kode := strings.TrimSpace(q.KodeBarang)
	if kode == "" {
		return nil, dto.PageMeta{}, dto.PergerakanStokRingkasan{}, fieldErr("kode_barang", "wajib diisi")
	}
	db := s.db.WithContext(ctx)
	b, err := s.barang.FindByKode(db, kode)
	if err != nil {
		return nil, dto.PageMeta{}, dto.PergerakanStokRingkasan{}, err
	}
	hasil, err := s.stock.ListPergerakan(db, b.ID, q)
	if err != nil {
		return nil, dto.PageMeta{}, dto.PergerakanStokRingkasan{}, err
	}
	out := make([]dto.PergerakanStokItemResponse, 0, len(hasil.Items))
	for _, it := range hasil.Items {
		out = append(out, dto.PergerakanStokItemResponse{
			ID: it.ID, CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339),
			MovementType: it.MovementType, NoBatch: it.NoBatch, Qty: it.Qty,
			SaldoSetelah: it.SaldoSetelah, ReferenceType: it.ReferenceType,
			ReferenceNo: it.ReferenceNo, Keterangan: it.Keterangan, Oleh: it.Oleh,
		})
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), hasil.Ringkasan, nil
}

// Penyesuaian manual (super_admin / stok.sesuaikan).
func (s *StockService) Penyesuaian(
	ctx context.Context,
	req dto.PenyesuaianStokRequest,
	userID uint64,
	meta AuditMeta,
) (*dto.PenyesuaianStokResponse, error) {
	kode := strings.TrimSpace(req.KodeBarang)
	if kode == "" {
		return nil, fieldErr("kode_barang", "wajib diisi")
	}
	if req.Qty == 0 {
		return nil, fieldErr("qty", "tidak boleh 0")
	}
	alasan := strings.TrimSpace(req.Alasan)
	if len(alasan) < 3 {
		return nil, fieldErr("alasan", "minimal 3 karakter")
	}

	var out *dto.PenyesuaianStokResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		b, err := s.barang.FindByKode(tx, kode)
		if err != nil {
			return err
		}
		locked, err := s.barang.LockByIDs(tx, []uint64{b.ID})
		if err != nil {
			return err
		}
		if len(locked) == 0 {
			return domain.ErrTidakDitemukan
		}
		barang := &locked[0]
		if req.BarangMasukID != nil {
			var n int64
			if err := tx.Model(&domain.BarangMasuk{}).
				Where("id = ? AND barang_id = ?", *req.BarangMasukID, barang.ID).
				Count(&n).Error; err != nil {
				return err
			}
			if n == 0 {
				return fieldErr("barang_masuk_id", "batch tidak cocok dengan SKU")
			}
		}
		uid := userID
		mv, err := s.stock.CatatPenyesuaian(tx, barang, req.Qty, req.BarangMasukID, alasan, &uid)
		if err != nil {
			return err
		}
		ringkas := fmt.Sprintf("penyesuaian stok %s qty=%d → %d", kode, req.Qty, barang.StokTersedia)
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "stok.penyesuaian", EntityType: "barang",
			EntityID: &barang.ID, Ringkasan: &ringkas,
			DataSesudah: map[string]any{
				"kode_barang": kode, "qty": req.Qty, "saldo_setelah": barang.StokTersedia, "alasan": alasan,
			},
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		out = &dto.PenyesuaianStokResponse{
			ID: mv.ID, KodeBarang: kode, Qty: mv.Qty, SaldoSetelah: mv.SaldoSetelah,
			MovementType: string(mv.MovementType), CreatedAt: mv.CreatedAt.UTC().Format(time.RFC3339),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Rekonsiliasi membandingkan SUM(ledger) vs stok_tersedia (SC-13).
func (s *StockService) Rekonsiliasi(ctx context.Context) (*dto.RekonsiliasiHasil, error) {
	selisih, total, err := s.stock.RekonsiliasiSelisih(s.db.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if selisih == nil {
		selisih = []dto.RekonsiliasiBaris{}
	}
	return &dto.RekonsiliasiHasil{
		Diperiksa: total, Menyimpang: len(selisih), Penyimpangan: selisih,
	}, nil
}