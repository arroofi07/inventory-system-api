package service

import (
	"context"
	"fmt"
	"time"

	"app/internal/config"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/pkg/fakturpdf"
	"app/internal/pkg/terbilang"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// FakturService builder JSON faktur + lock cetak (SD-09, SD-10).
type FakturService struct {
	db         *gorm.DB
	trx        *repository.TransaksiRepo
	users      *repository.UserRepo
	audit      *repository.AuditRepo
	clock      clock.Clock
	cfg        config.BisnisConfig
	perusahaan config.PerusahaanConfig
}

func NewFakturService(
	db *gorm.DB,
	trx *repository.TransaksiRepo,
	users *repository.UserRepo,
	audit *repository.AuditRepo,
	clk clock.Clock,
	bisnis config.BisnisConfig,
	perusahaan config.PerusahaanConfig,
) *FakturService {
	return &FakturService{
		db: db, trx: trx, users: users, audit: audit, clock: clk,
		cfg: bisnis, perusahaan: perusahaan,
	}
}

// Ambil membangun data faktur, menerapkan lock sekali cetak (04 §6.1–6.2).
func (s *FakturService) Ambil(
	ctx context.Context,
	id uint64,
	userID uint64,
	role domain.Role,
	meta AuditMeta,
) (*dto.FakturResponse, error) {
	var out *dto.FakturResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		locked, err := s.trx.LockByID(tx, id)
		if err != nil {
			return err
		}
		if locked.StatusApproval != domain.ApprovalApproved {
			return domain.ErrStatusTidakValid
		}

		full, err := s.trx.FindByID(tx, id)
		if err != nil {
			return err
		}

		cetakUlang := false
		now := s.clock.Now()

		if s.cfg.FakturLockAktif {
			if full.FakturDicetakAt != nil {
				if !role.Punya(domain.PermFakturCetakUlang) {
					return domain.ErrFakturTerkunci
				}
				cetakUlang = true
				// SA cetak ulang: jangan ubah timestamp.
			} else {
				t := now
				full.FakturDicetakAt = &t
				full.FakturDicetakBy = &userID
				if err := tx.Model(full).Select(
					"faktur_dicetak_at", "faktur_dicetak_by", "updated_at",
				).Updates(map[string]any{
					"faktur_dicetak_at": t,
					"faktur_dicetak_by": userID,
					"updated_at":        t,
				}).Error; err != nil {
					return err
				}
			}

			ringkas := "cetak faktur"
			if cetakUlang {
				ringkas = "cetak ulang faktur"
			}
			uid := userID
			if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
				UserID:     &uid,
				Aksi:       "faktur.cetak",
				EntityType: "transaksi_penjualan",
				EntityID:   &id,
				Ringkasan:  &ringkas,
				DataSesudah: map[string]any{
					"cetak_ulang":       cetakUlang,
					"no_transaksi":      full.NoTransaksi,
					"faktur_dicetak_at": full.FakturDicetakAt,
				},
				IPAddress: meta.IPAddress,
				RequestID: meta.RequestID,
			}); err != nil {
				return err
			}
		}

		built, err := s.buildResponse(tx, full, cetakUlang)
		if err != nil {
			return err
		}
		out = built
		return nil
	})
	return out, err
}

// HasilPDF adalah berkas PDF faktur beserta nama unduhan (SD-11).
type HasilPDF struct {
	Body     []byte
	Filename string
}

// AmbilPDF memakai data + lock yang sama dengan Ambil, lalu merender PDF.
func (s *FakturService) AmbilPDF(
	ctx context.Context,
	id uint64,
	userID uint64,
	role domain.Role,
	meta AuditMeta,
) (*HasilPDF, error) {
	data, err := s.Ambil(ctx, id, userID, role, meta)
	if err != nil {
		return nil, err
	}
	body, err := fakturpdf.Bangun(data, s.cfg.FakturItemsPerPage)
	if err != nil {
		return nil, err
	}
	return &HasilPDF{
		Body:     body,
		Filename: fakturpdf.NamaBerkas(data.NoTransaksi, id),
	}, nil
}

func (s *FakturService) buildResponse(
	tx *gorm.DB,
	trx *domain.TransaksiPenjualan,
	cetakUlang bool,
) (*dto.FakturResponse, error) {
	alokasi := domain.AlokasiProporsionalFaktur(trx.Total, trx.PPNNominal, trx.Details)
	items := make([]dto.FakturItem, 0, domain.HitungJumlahBarisCetakFaktur(trx.Details))
	urutanCetak := uint16(0)

	for i, d := range trx.Details {
		urutanCetak++
		var exp *string
		if d.ExpiryDate != nil {
			s := time.Time(*d.ExpiryDate).UTC().Format("2006-01-02")
			exp = &s
		}
		var al domain.BarisFaktur
		if i < len(alokasi) {
			al = alokasi[i]
		}
		items = append(items, dto.FakturItem{
			Urutan:             urutanCetak,
			KodeItem:           d.KodeItem,
			NamaItem:           d.NamaItem,
			NoBatch:            d.BatchNumber,
			Exp:                exp,
			Qty:                d.Qty,
			Satuan:             d.Satuan,
			Harga:              dto.FormatUang(d.Harga),
			Disc1Persen:        dto.FormatUang(d.Disc1Persen),
			Disc2Persen:        dto.FormatUang(d.Disc2Persen),
			Disc3Persen:        dto.FormatUang(d.Disc3Persen),
			NilaiSetelahGlobal: dto.FormatUang(al.NilaiSetelahGlobal),
			PPNBaris:           dto.FormatUang(al.PPNBaris),
			TotalFinalBaris:    dto.FormatUang(al.TotalFinalBaris),
			IsBonus:            false,
		})
		if d.QtyPromo > 0 {
			urutanCetak++
			nol := dto.FormatUang(decimal.Zero)
			items = append(items, dto.FakturItem{
				Urutan:          urutanCetak,
				KodeItem:        d.KodeItem,
				NamaItem:        fmt.Sprintf("BONUS - %s", d.NamaItem),
				NoBatch:         d.BatchNumber,
				Exp:             exp,
				Qty:             d.QtyPromo,
				Satuan:          d.Satuan,
				Harga:           nol,
				TotalFinalBaris: nol,
				IsBonus:         true,
			})
		}
	}

	layout, totalHalaman := domain.PilihLayoutFaktur(
		len(items),
		s.cfg.FakturHalfPageMax,
		s.cfg.FakturFullPageMax,
		s.cfg.FakturItemsPerPage,
	)

	var dicetakAt *string
	if trx.FakturDicetakAt != nil {
		s := trx.FakturDicetakAt.Format(time.RFC3339)
		dicetakAt = &s
	}

	sales := s.userRingkas(tx, trx.SalesID)
	approver := s.userRingkas(tx, trx.ApprovedBy)

	return &dto.FakturResponse{
		NoTransaksi:  trx.NoTransaksi,
		Tanggal:      time.Time(trx.Tanggal).UTC().Format("2006-01-02"),
		Layout:       string(layout),
		TotalHalaman: totalHalaman,
		Perusahaan: dto.FakturPerusahaan{
			Nama:    s.perusahaan.Nama,
			Alamat:  s.perusahaan.Alamat,
			Telepon: s.perusahaan.Telepon,
			NPWP:    s.perusahaan.NPWP,
		},
		Pelanggan: dto.FakturPelanggan{
			KodePelanggan: trx.KodePelanggan,
			NamaPelanggan: trx.NamaPelanggan,
			Alamat:        trx.Alamat,
			ChannelOutlet: string(trx.ChannelOutlet),
		},
		Items: items,
		Ringkasan: dto.FakturRingkasan{
			Total:       dto.FormatUang(trx.Total),
			Disc1Persen: dto.FormatUang(trx.Disc1Persen),
			Disc2Persen: dto.FormatUang(trx.Disc2Persen),
			Disc3Persen: dto.FormatUang(trx.Disc3Persen),
			PPNPersen:   dto.FormatUang(trx.PPNPersen),
			PPNNominal:  dto.FormatUang(trx.PPNNominal),
			TotalAkhir:  dto.FormatUang(trx.TotalAkhir),
			Terbilang:   terbilang.Rupiah(trx.TotalAkhir),
		},
		Sales:           sales,
		Approver:        approver,
		FakturDicetakAt: dicetakAt,
		CetakUlang:      cetakUlang,
	}, nil
}

func (s *FakturService) userRingkas(tx *gorm.DB, id *uint64) *dto.UserRingkasResponse {
	if id == nil {
		return nil
	}
	u, err := s.users.FindByID(tx, *id)
	if err != nil || u == nil {
		return &dto.UserRingkasResponse{ID: *id}
	}
	return &dto.UserRingkasResponse{ID: u.ID, Name: u.Name}
}
