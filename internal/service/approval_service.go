package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/repository"
	"gorm.io/gorm"
)

// KetersediaanStok menghitung kecukupan per SKU + pending bersaing (SC-09).
func (s *TransaksiService) KetersediaanStok(ctx context.Context, id uint64) (*dto.KetersediaanStokResponse, error) {
	db := s.db.WithContext(ctx)
	trx, err := s.trx.FindByID(db, id)
	if err != nil {
		return nil, err
	}

	agg := agregatQtyKeluar(trx.Details)
	kodes := make([]string, 0, len(agg))
	for k := range agg {
		kodes = append(kodes, k)
	}
	sort.Strings(kodes)

	barangs, err := s.barang.FindByKodes(db, kodes)
	if err != nil {
		return nil, err
	}
	byKode := map[string]domain.Barang{}
	for _, b := range barangs {
		byKode[b.KodeBarang] = b
	}

	bersaing, err := s.trx.ListPendingBersaing(db, kodes, id, 30)
	if err != nil {
		return nil, err
	}
	bersaingBySKU := map[string][]uint64{}
	bersaingOut := make([]dto.TransaksiBersaingItem, 0, len(bersaing))
	for _, b := range bersaing {
		kodeSet := map[string]struct{}{}
		for _, d := range b.Details {
			if _, ok := agg[d.KodeItem]; ok {
				kodeSet[d.KodeItem] = struct{}{}
				bersaingBySKU[d.KodeItem] = appendUnique(bersaingBySKU[d.KodeItem], b.ID)
			}
		}
		kodeList := make([]string, 0, len(kodeSet))
		for k := range kodeSet {
			kodeList = append(kodeList, k)
		}
		sort.Strings(kodeList)
		bersaingOut = append(bersaingOut, dto.TransaksiBersaingItem{
			ID: b.ID, KodePelanggan: b.KodePelanggan, NamaPelanggan: b.NamaPelanggan,
			TotalAkhir: dto.FormatUang(b.TotalAkhir),
			Tanggal:    time.Time(b.Tanggal).UTC().Format("2006-01-02"),
			KodeItems:  kodeList,
		})
	}

	skuRows := make([]dto.KetersediaanSKUBaris, 0, len(kodes))
	semuaCukup := true
	for _, kode := range kodes {
		nama := kode
		tersedia := 0
		if b, ok := byKode[kode]; ok {
			nama = b.NamaItem
			tersedia = b.StokTersedia
		}
		diminta := agg[kode]
		cukup := tersedia >= diminta
		if !cukup {
			semuaCukup = false
		}
		skuRows = append(skuRows, dto.KetersediaanSKUBaris{
			KodeItem: kode, NamaItem: nama, Diminta: diminta, Tersedia: tersedia,
			StokCukup: cukup, BersaingIDs: bersaingBySKU[kode],
		})
	}

	return &dto.KetersediaanStokResponse{
		TransaksiID: id, SemuaStokCukup: semuaCukup, SKU: skuRows, Bersaing: bersaingOut,
	}, nil
}

// Approve menyetujui pending: lock stok, ledger, no_transaksi (SC-10).
func (s *TransaksiService) Approve(
	ctx context.Context,
	id uint64,
	req dto.ApproveRequest,
	approverID uint64,
	meta AuditMeta,
) (*dto.HasilApprovalResponse, error) {
	var hasil *dto.HasilApprovalResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		trx, err := s.trx.FindByID(tx, id)
		if err != nil {
			return err
		}
		if trx.StatusApproval != domain.ApprovalPending {
			return domain.ErrStatusTidakValid
		}
		if len(trx.Details) == 0 {
			return fieldErr("items", "transaksi tanpa detail")
		}

		agg := agregatQtyKeluar(trx.Details)
		kodes := keysSorted(agg)
		barangs, err := s.barang.FindByKodes(tx, kodes)
		if err != nil {
			return err
		}
		if len(barangs) != len(kodes) {
			return domain.ErrTidakDitemukan
		}
		ids := make([]uint64, 0, len(barangs))
		byKode := map[string]*domain.Barang{}
		for i := range barangs {
			ids = append(ids, barangs[i].ID)
			byKode[barangs[i].KodeBarang] = &barangs[i]
		}
		locked, err := s.barang.LockByIDs(tx, ids)
		if err != nil {
			return err
		}
		byID := map[uint64]*domain.Barang{}
		for i := range locked {
			byID[locked[i].ID] = &locked[i]
			byKode[locked[i].KodeBarang] = &locked[i]
		}

		var kurang []domain.StokKurangBaris
		for _, kode := range kodes {
			b := byKode[kode]
			diminta := agg[kode]
			if b == nil || b.StokTersedia < diminta {
				tersedia := 0
				nama := kode
				if b != nil {
					tersedia = b.StokTersedia
					nama = b.NamaItem
				}
				kurang = append(kurang, domain.StokKurangBaris{
					KodeItem: kode, NamaItem: nama, Diminta: diminta, Tersedia: tersedia,
				})
			}
		}
		if len(kurang) > 0 {
			return &domain.ErrStokApproval{Details: kurang}
		}

		noTrx, err := s.trx.NextNoTransaksi(tx)
		if err != nil {
			return err
		}

		hariIni := s.clock.Now().UTC()
		uid := approverID
		pergerakan := make([]dto.PergerakanStokItem, 0)
		for _, d := range trx.Details {
			b := byKode[d.KodeItem]
			if b == nil {
				return domain.ErrTidakDitemukan
			}
			kandidat, _, err := s.kandidatBatch(tx, b, nil, hariIni)
			if err != nil {
				return err
			}
			alokasi, err := domain.AlokasiBatch(kandidat, d.TotalQtyKeluar, b.MetodeAlokasi, trx.ChannelOutlet, hariIni)
			if err != nil {
				if errors.Is(err, domain.ErrStokTidakCukup) {
					return &domain.ErrStokApproval{Details: []domain.StokKurangBaris{{
						KodeItem: d.KodeItem, NamaItem: d.NamaItem,
						Diminta: d.TotalQtyKeluar, Tersedia: b.StokTersedia,
					}}}
				}
				return err
			}
			for _, a := range alokasi {
				saldo, err := s.stock.CatatPenjualan(tx, b, a.BarangMasukID, a.Qty, trx.ID, noTrx, &uid)
				if err != nil {
					return err
				}
				pergerakan = append(pergerakan, dto.PergerakanStokItem{
					KodeItem: d.KodeItem, NoBatch: a.NoBatch, Qty: -a.Qty, SaldoSetelah: saldo,
				})
			}
		}

		now := s.clock.Now().UTC()
		notes := strings.TrimSpace(req.ApprovalNotes)
		var notesPtr *string
		if notes != "" {
			notesPtr = &notes
		}
		trx.NoTransaksi = &noTrx
		trx.StatusApproval = domain.ApprovalApproved
		trx.ApprovedAt = &now
		trx.ApprovedBy = &uid
		trx.ApprovalNotes = notesPtr
		trx.FulfillmentStatus = domain.FulfillmentFull
		if err := s.trx.MarkApproved(tx, trx); err != nil {
			return err
		}

		ringkas := fmt.Sprintf("approve transaksi #%d → %s", trx.ID, noTrx)
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "transaksi.approve", EntityType: "transaksi_penjualan",
			EntityID: &trx.ID, Ringkasan: &ringkas,
			DataSesudah: map[string]any{"id": trx.ID, "no_transaksi": noTrx, "status_approval": "approved"},
			IPAddress:   meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}

		hasil = &dto.HasilApprovalResponse{
			ID: trx.ID, NoTransaksi: noTrx, StatusApproval: string(domain.ApprovalApproved),
			ApprovedAt: now.Format(time.RFC3339), ApprovedBy: approverID,
			TotalAkhir: dto.FormatUang(trx.TotalAkhir), PergerakanStok: pergerakan,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hasil, nil
}

// Reject menolak pending dengan catatan wajib (SC-11).
func (s *TransaksiService) Reject(
	ctx context.Context,
	id uint64,
	req dto.RejectRequest,
	approverID uint64,
	meta AuditMeta,
) (*dto.TransaksiResponse, error) {
	notes := strings.TrimSpace(req.ApprovalNotes)
	if notes == "" {
		return nil, fieldErr("approval_notes", "wajib diisi")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		trx, err := s.trx.FindByID(tx, id)
		if err != nil {
			return err
		}
		if trx.StatusApproval != domain.ApprovalPending {
			return domain.ErrStatusTidakValid
		}
		now := s.clock.Now().UTC()
		uid := approverID
		trx.StatusApproval = domain.ApprovalRejected
		trx.ApprovedAt = &now
		trx.ApprovedBy = &uid
		trx.ApprovalNotes = &notes
		trx.NoTransaksi = nil
		if err := s.trx.MarkRejected(tx, trx); err != nil {
			return err
		}
		ringkas := fmt.Sprintf("reject transaksi #%d", trx.ID)
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "transaksi.reject", EntityType: "transaksi_penjualan",
			EntityID: &trx.ID, Ringkasan: &ringkas,
			DataSesudah: map[string]any{"id": trx.ID, "status_approval": "rejected", "approval_notes": notes},
			IPAddress:   meta.IPAddress, RequestID: meta.RequestID,
		})
	})
	if err != nil {
		return nil, err
	}
	loaded, err := s.trx.FindByID(s.db.WithContext(ctx), id)
	if err != nil {
		return nil, err
	}
	resp := mapTransaksiResponse(loaded)
	return &resp, nil
}

// BulkApprove memproses tiap ID terpisah (SC-12).
func (s *TransaksiService) BulkApprove(
	ctx context.Context,
	req dto.BulkApproveRequest,
	approverID uint64,
	meta AuditMeta,
) (*dto.BulkApproveResponse, error) {
	if len(req.TransaksiIDs) == 0 {
		return nil, fieldErr("transaksi_ids", "wajib diisi")
	}
	ids := append([]uint64(nil), req.TransaksiIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := &dto.BulkApproveResponse{}
	out.Ringkasan.Total = len(ids)
	for _, id := range ids {
		res, err := s.Approve(ctx, id, dto.ApproveRequest{ApprovalNotes: req.ApprovalNotes}, approverID, meta)
		if err != nil {
			item := dto.BulkApproveItemHasil{ID: id, OK: false, Message: err.Error()}
			var stokErr *domain.ErrStokApproval
			switch {
			case errors.As(err, &stokErr):
				item.Code = "STOK_TIDAK_CUKUP"
				item.Message = "Stok tidak cukup untuk menyetujui transaksi"
				for _, d := range stokErr.Details {
					item.Details = append(item.Details, dto.StokKurangItem{
						KodeItem: d.KodeItem, NamaItem: d.NamaItem, Diminta: d.Diminta, Tersedia: d.Tersedia,
					})
				}
			case errors.Is(err, domain.ErrStatusTidakValid):
				item.Code = "STATUS_TIDAK_VALID"
				item.Message = "Operasi tidak diizinkan pada status saat ini"
			case errors.Is(err, domain.ErrTidakDitemukan):
				item.Code = "TIDAK_DITEMUKAN"
				item.Message = "Data tidak ditemukan"
			default:
				item.Code = "KESALAHAN_INTERNAL"
			}
			out.Gagal = append(out.Gagal, item)
			out.Ringkasan.Gagal++
			continue
		}
		no := res.NoTransaksi
		out.Berhasil = append(out.Berhasil, dto.BulkApproveItemHasil{
			ID: id, OK: true, NoTransaksi: &no,
		})
		out.Ringkasan.Berhasil++
	}
	if out.Berhasil == nil {
		out.Berhasil = []dto.BulkApproveItemHasil{}
	}
	if out.Gagal == nil {
		out.Gagal = []dto.BulkApproveItemHasil{}
	}
	return out, nil
}

func agregatQtyKeluar(details []domain.TransaksiDetail) map[string]int {
	agg := map[string]int{}
	for _, d := range details {
		agg[d.KodeItem] += d.TotalQtyKeluar
	}
	return agg
}

func keysSorted(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func appendUnique(xs []uint64, v uint64) []uint64 {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}