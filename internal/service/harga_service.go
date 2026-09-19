package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"app/internal/domain"
	"app/internal/dto"
	appvalidator "app/internal/pkg/validator"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// HargaMassal memperbarui harga banyak batch satu SKU dalam satu bulk_operation_id.
func (s *BarangService) HargaMassal(
	ctx context.Context,
	barangID uint64,
	req dto.HargaMassalRequest,
	meta AuditMeta,
) (*dto.HargaMassalResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	if _, err := s.barang.FindByID(s.db.WithContext(ctx), barangID); err != nil {
		return nil, err
	}

	bulkID := uuid.NewString()
	var updated, skipped int
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows, err := s.bm.FindByIDsForBarang(tx, barangID, req.BatchIDs)
		if err != nil {
			return err
		}
		if len(rows) != len(uniqueUint64(req.BatchIDs)) {
			return validationField("batch_ids", "satu atau lebih batch tidak ditemukan untuk SKU ini")
		}
		now := s.clock.Now().UTC()
		for i := range rows {
			sebelum := rows[i]
			sesudah := sebelum
			if err := applyHargaFields(&sesudah, req); err != nil {
				return err
			}
			hitungHargaBatch(&sesudah, s.ppn)
			if !hargaPricingBerubah(&sebelum, &sesudah) {
				skipped++
				continue
			}
			if err := s.bm.Update(tx, &sesudah); err != nil {
				return err
			}
			log := buatPriceChangeLog(&sebelum, &sesudah, bulkID, meta.UserID, req.Keterangan, now)
			if err := s.harga.Create(tx, &log); err != nil {
				return err
			}
			updated++
		}
		if updated > 0 {
			ringkas := fmt.Sprintf("harga massal %d batch (bulk %s)", updated, bulkID)
			if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
				UserID: meta.UserID, Aksi: "barang.harga_massal", EntityType: "barang",
				EntityID: &barangID, Ringkasan: &ringkas,
				IPAddress: meta.IPAddress, RequestID: meta.RequestID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &dto.HargaMassalResponse{
		BulkOperationID:       bulkID,
		JumlahBatchDiperbarui: updated,
		JumlahDilewati:        skipped,
	}, nil
}

// RiwayatHarga daftar price_change_logs per SKU.
func (s *BarangService) RiwayatHarga(
	ctx context.Context,
	barangID uint64,
	q dto.RiwayatHargaQuery,
) ([]dto.PriceChangeLogResponse, dto.PageMeta, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, dto.PageMeta{}, err
	}
	q.Normalize()
	if _, err := s.barang.FindByID(s.db.WithContext(ctx), barangID); err != nil {
		return nil, dto.PageMeta{}, err
	}
	hasil, err := s.harga.ListByBarangID(s.db.WithContext(ctx), barangID, q.Page, q.PerPage)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}

	batchIDs := make([]uint64, 0, len(hasil.Items))
	for _, it := range hasil.Items {
		batchIDs = append(batchIDs, it.BarangMasukID)
	}
	noBatch := map[uint64]string{}
	if len(batchIDs) > 0 {
		var bms []domain.BarangMasuk
		_ = s.db.WithContext(ctx).Select("id", "no_batch").Where("id IN ?", batchIDs).Find(&bms)
		for _, b := range bms {
			noBatch[b.ID] = b.NoBatch
		}
	}

	out := make([]dto.PriceChangeLogResponse, 0, len(hasil.Items))
	for _, it := range hasil.Items {
		out = append(out, mapPriceChangeLog(&it, noBatch[it.BarangMasukID]))
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), nil
}

func applyHargaFields(bm *domain.BarangMasuk, req dto.HargaMassalRequest) error {
	var err error
	if req.Harga != nil {
		bm.Harga, err = dto.ParseUang(*req.Harga, "harga")
		if err != nil {
			return validationField("harga", err.Error())
		}
		if bm.Harga.IsNegative() {
			return validationField("harga", "harga tidak boleh negatif")
		}
	}
	if req.DiscHPP1 != nil {
		bm.DiscHPP1, err = dto.ParseUangOpsional(*req.DiscHPP1, "disc_hpp_1")
		if err != nil {
			return validationField("disc_hpp_1", err.Error())
		}
	}
	if req.DiscHPP2 != nil {
		bm.DiscHPP2, err = dto.ParseUangOpsional(*req.DiscHPP2, "disc_hpp_2")
		if err != nil {
			return validationField("disc_hpp_2", err.Error())
		}
	}
	if req.DiscHPP3 != nil {
		bm.DiscHPP3, err = dto.ParseUangOpsional(*req.DiscHPP3, "disc_hpp_3")
		if err != nil {
			return validationField("disc_hpp_3", err.Error())
		}
	}
	if err := validateDiscRange(bm.DiscHPP1, bm.DiscHPP2, bm.DiscHPP3); err != nil {
		return err
	}
	if req.MarkupMTType != nil && *req.MarkupMTType != "" {
		bm.MarkupMTType = domain.MarkupType(*req.MarkupMTType)
	}
	if req.MarkupMTAmount != nil {
		bm.MarkupMTAmount, err = dto.ParseUangOpsional(*req.MarkupMTAmount, "markup_mt_amount")
		if err != nil {
			return validationField("markup_mt_amount", err.Error())
		}
	}
	if req.MarkupGTType != nil && *req.MarkupGTType != "" {
		bm.MarkupGTType = domain.MarkupType(*req.MarkupGTType)
	}
	if req.MarkupGTAmount != nil {
		bm.MarkupGTAmount, err = dto.ParseUangOpsional(*req.MarkupGTAmount, "markup_gt_amount")
		if err != nil {
			return validationField("markup_gt_amount", err.Error())
		}
	}
	return nil
}

func uniqueUint64(ids []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func mapPriceChangeLog(l *domain.PriceChangeLog, noBatch string) dto.PriceChangeLogResponse {
	return dto.PriceChangeLogResponse{
		ID:              l.ID,
		BarangMasukID:   l.BarangMasukID,
		NoBatch:         noBatch,
		BulkOperationID: l.BulkOperationID,
		OldHarga:        formatDecPtr(l.OldHarga),
		NewHarga:        formatDecPtr(l.NewHarga),
		OldHPP:          formatDecPtr(l.OldHPP),
		NewHPP:          formatDecPtr(l.NewHPP),
		OldHargaMT:      formatDecPtr(l.OldHargaMT),
		NewHargaMT:      formatDecPtr(l.NewHargaMT),
		OldHargaGT:      formatDecPtr(l.OldHargaGT),
		NewHargaGT:      formatDecPtr(l.NewHargaGT),
		Keterangan:      l.Keterangan,
		ChangedAt:       l.ChangedAt.UTC().Format(time.RFC3339),
	}
}

func formatDecPtr(d *decimal.Decimal) *string {
	if d == nil {
		return nil
	}
	s := dto.FormatUang(*d)
	return &s
}
