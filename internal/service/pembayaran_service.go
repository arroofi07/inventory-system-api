package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const idemScopePembayaran = "pembayaran"

// PembayaranService catat nominal & riwayat (SD-02, SD-04).
type PembayaranService struct {
	db    *gorm.DB
	trx   *repository.TransaksiRepo
	bayar *repository.PembayaranRepo
	idem  *repository.IdempotencyRepo
	audit *repository.AuditRepo
	clock clock.Clock
}

func NewPembayaranService(
	db *gorm.DB,
	trx *repository.TransaksiRepo,
	bayar *repository.PembayaranRepo,
	idem *repository.IdempotencyRepo,
	audit *repository.AuditRepo,
	clk clock.Clock,
) *PembayaranService {
	return &PembayaranService{
		db: db, trx: trx, bayar: bayar, idem: idem, audit: audit, clock: clk,
	}
}

// HasilIdempoten respons tersimpan dari request sebelumnya.
type HasilIdempoten struct {
	StatusCode int
	Body       []byte
}

// Catat menambahkan nominal_pembayaran ke transaksi approved (04 §5.4).
func (s *PembayaranService) Catat(
	ctx context.Context,
	id uint64,
	req dto.PembayaranCreateRequest,
	userID uint64,
	idemKey string,
	rawBody []byte,
	meta AuditMeta,
) (*dto.PembayaranCreateResponse, *HasilIdempoten, error) {
	reqHash := hashBody(rawBody)
	if idemKey != "" {
		existing, err := s.idem.Find(s.db.WithContext(ctx), idemScopePembayaran, userID, idemKey)
		if err != nil {
			return nil, nil, err
		}
		if existing != nil {
			if existing.RequestHash != reqHash {
				return nil, nil, fieldErr("Idempotency-Key", "sudah dipakai dengan body berbeda")
			}
			return nil, &HasilIdempoten{StatusCode: existing.StatusCode, Body: existing.ResponseJSON}, nil
		}
	}

	nominal, err := dto.ParseUang(req.NominalPembayaran, "nominal_pembayaran")
	if err != nil {
		return nil, nil, fieldErr("nominal_pembayaran", err.Error())
	}
	if !nominal.GreaterThan(decimal.Zero) {
		return nil, nil, fieldErr("nominal_pembayaran", "harus lebih dari 0")
	}

	var tglBayar *datatypes.Date
	if strings.TrimSpace(req.TanggalPembayaran) != "" {
		d, err := parseDateField(req.TanggalPembayaran, "tanggal_pembayaran")
		if err != nil {
			return nil, nil, err
		}
		tglBayar = &d
	} else {
		now := s.clock.Now().UTC()
		d := datatypes.Date(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC))
		tglBayar = &d
	}

	var out *dto.PembayaranCreateResponse
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		trx, err := s.trx.LockByID(tx, id)
		if err != nil {
			return err
		}
		if trx.StatusApproval != domain.ApprovalApproved {
			return domain.ErrStatusTidakValid
		}

		baru := trx.JumlahDibayar.Add(nominal)
		if baru.GreaterThan(trx.TotalAkhir) {
			return domain.ErrKelebihanBayar
		}

		oldStatus := trx.StatusPembayaran
		oldDibayar := trx.JumlahDibayar
		oldSisa := trx.SisaHutang

		status, sisa, dibayar := domain.TurunkanStatusPembayaran(trx.TotalAkhir, baru)
		trx.StatusPembayaran = status
		trx.JumlahDibayar = dibayar
		trx.SisaHutang = sisa
		now := s.clock.Now().UTC()
		trx.TanggalPembayaranTerakhir = &now
		if status == domain.PembayaranLunas {
			trx.TanggalJatuhTempo = nil
			trx.JumlahDibayar = trx.TotalAkhir
			trx.SisaHutang = decimal.Zero
		}
		if ket := strings.TrimSpace(req.Keterangan); ket != "" {
			trx.KeteranganPembayaran = &ket
		}
		if err := s.trx.UpdatePembayaran(tx, trx); err != nil {
			return err
		}

		uid := userID
		oldSt := oldStatus
		newSt := status
		var metode *string
		if m := strings.TrimSpace(req.MetodePembayaran); m != "" {
			metode = &m
		}
		var ketPtr *string
		if k := strings.TrimSpace(req.Keterangan); k != "" {
			ketPtr = &k
		}
		row := domain.RiwayatPembayaran{
			TransaksiPenjualanID: trx.ID,
			OldJumlahDibayar:     oldDibayar,
			NewJumlahDibayar:     trx.JumlahDibayar,
			OldSisaHutang:        oldSisa,
			NewSisaHutang:        trx.SisaHutang,
			OldStatus:            &oldSt,
			NewStatus:            &newSt,
			NominalPembayaran:    nominal,
			MetodePembayaran:     metode,
			TanggalPembayaran:    tglBayar,
			Keterangan:           ketPtr,
			ChangedBy:            &uid,
			ChangedAt:            now,
			CreatedAt:            now,
		}
		if err := s.bayar.Create(tx, &row); err != nil {
			return err
		}

		ringkas := fmt.Sprintf("pembayaran trx #%d nominal %s → %s", trx.ID, dto.FormatUang(nominal), string(status))
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "pembayaran.catat", EntityType: "transaksi_penjualan",
			EntityID: &trx.ID, Ringkasan: &ringkas,
			DataSesudah: map[string]any{
				"nominal_pembayaran": dto.FormatUang(nominal),
				"jumlah_dibayar":     dto.FormatUang(trx.JumlahDibayar),
				"sisa_hutang":        dto.FormatUang(trx.SisaHutang),
				"status_pembayaran":  string(status),
				"riwayat_id":         row.ID,
			},
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}

		var tpt *string
		if trx.TanggalPembayaranTerakhir != nil {
			s := trx.TanggalPembayaranTerakhir.UTC().Format(time.RFC3339)
			tpt = &s
		}
		out = &dto.PembayaranCreateResponse{
			TransaksiID:               trx.ID,
			NoTransaksi:               trx.NoTransaksi,
			TotalAkhir:                dto.FormatUang(trx.TotalAkhir),
			JumlahDibayar:             dto.FormatUang(trx.JumlahDibayar),
			SisaHutang:                dto.FormatUang(trx.SisaHutang),
			StatusPembayaran:          string(trx.StatusPembayaran),
			TanggalPembayaranTerakhir: tpt,
			RiwayatID:                 row.ID,
		}

		if idemKey != "" {
			payload, _ := json.Marshal(map[string]any{"data": out})
			rid := trx.ID
			rec := &repository.IdempotencyRecord{
				Scope: idemScopePembayaran, IdemKey: idemKey, UserID: userID,
				ResourceID: &rid, RequestHash: reqHash, ResponseJSON: payload,
				StatusCode: 200, CreatedAt: now,
			}
			if err := s.idem.Save(tx, rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return out, nil, nil
}

// RiwayatByTransaksi daftar kronologi pembayaran (SD-04).
func (s *PembayaranService) RiwayatByTransaksi(ctx context.Context, id uint64) ([]dto.RiwayatPembayaranItem, error) {
	db := s.db.WithContext(ctx)
	if _, err := s.trx.FindByID(db, id); err != nil {
		return nil, err
	}
	rows, err := s.bayar.ListByTransaksi(db, id)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RiwayatPembayaranItem, 0, len(rows))
	for _, row := range rows {
		item := dto.RiwayatPembayaranItem{
			ID:                row.ID,
			NominalPembayaran: dto.FormatUang(row.NominalPembayaran),
			OldJumlahDibayar:  dto.FormatUang(row.OldJumlahDibayar),
			NewJumlahDibayar:  dto.FormatUang(row.NewJumlahDibayar),
			OldSisaHutang:     dto.FormatUang(row.OldSisaHutang),
			NewSisaHutang:     dto.FormatUang(row.NewSisaHutang),
			MetodePembayaran:  row.MetodePembayaran,
			Keterangan:        row.Keterangan,
			ChangedBy:         row.ChangedBy,
			ChangedAt:         row.ChangedAt.UTC().Format(time.RFC3339),
		}
		if row.OldStatus != nil {
			s := string(*row.OldStatus)
			item.OldStatus = &s
		}
		if row.NewStatus != nil {
			s := string(*row.NewStatus)
			item.NewStatus = &s
		}
		if row.TanggalPembayaran != nil {
			s := time.Time(*row.TanggalPembayaran).UTC().Format("2006-01-02")
			item.TanggalPembayaran = &s
		}
		out = append(out, item)
	}
	return out, nil
}

func hashBody(b []byte) string {
	if len(b) == 0 {
		b = []byte("{}")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
