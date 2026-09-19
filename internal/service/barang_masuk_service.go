package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/money"
	appvalidator "app/internal/pkg/validator"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type BarangMasukService struct {
	db     *gorm.DB
	bm     *repository.BarangMasukRepo
	barang *repository.BarangRepo
	stock  *repository.StockRepo
	harga  *repository.PriceChangeRepo
	audit  *repository.AuditRepo
	ppn    decimal.Decimal
}

func NewBarangMasukService(
	db *gorm.DB,
	bm *repository.BarangMasukRepo,
	barang *repository.BarangRepo,
	stock *repository.StockRepo,
	harga *repository.PriceChangeRepo,
	audit *repository.AuditRepo,
	ppnPersen decimal.Decimal,
) *BarangMasukService {
	return &BarangMasukService{db: db, bm: bm, barang: barang, stock: stock, harga: harga, audit: audit, ppn: ppnPersen}
}

func (s *BarangMasukService) Daftar(ctx context.Context, q dto.BarangMasukListQuery) ([]dto.BarangMasukResponse, dto.PageMeta, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, dto.PageMeta{}, err
	}
	q.Normalize()
	hasil, err := s.bm.List(s.db.WithContext(ctx), q)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	out := make([]dto.BarangMasukResponse, 0, len(hasil.Items))
	for i := range hasil.Items {
		resp, err := s.mapResponse(s.db.WithContext(ctx), &hasil.Items[i])
		if err != nil {
			return nil, dto.PageMeta{}, err
		}
		out = append(out, resp)
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), nil
}

func (s *BarangMasukService) Detail(ctx context.Context, id uint64) (*dto.BarangMasukResponse, error) {
	bm, err := s.bm.FindByID(s.db.WithContext(ctx), id)
	if err != nil {
		return nil, err
	}
	resp, err := s.mapResponse(s.db.WithContext(ctx), bm)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *BarangMasukService) Buat(ctx context.Context, req dto.BarangMasukCreateRequest, meta AuditMeta) (*dto.BarangMasukResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, err := parsePricingInput(
		req.Harga, req.DiscHPP1, req.DiscHPP2, req.DiscHPP3,
		req.MarkupMTType, req.MarkupMTAmount, req.MarkupGTType, req.MarkupGTAmount,
	)
	if err != nil {
		return nil, err
	}
	exp, err := parseDateField(req.Exp, "exp")
	if err != nil {
		return nil, err
	}
	tglMasuk, err := parseDateField(req.TanggalMasuk, "tanggal_masuk")
	if err != nil {
		return nil, err
	}
	aging := 0
	if req.AgingMonth != nil {
		aging = *req.AgingMonth
	}

	var out *dto.BarangMasukResponse
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		barang, err := s.resolveOrCreateBarang(tx, req, meta)
		if err != nil {
			return err
		}
		locked, err := s.bm.LockBarangForUpdate(tx, barang.ID)
		if err != nil {
			return err
		}

		bm := domain.BarangMasuk{
			BarangID:       locked.ID,
			NoFaktur:       strings.TrimSpace(req.NoFaktur),
			NoBatch:        strings.TrimSpace(req.NoBatch),
			Exp:            exp,
			TanggalMasuk:   tglMasuk,
			Qty:            req.Qty,
			Harga:          harga,
			DiscHPP1:       disc1,
			DiscHPP2:       disc2,
			DiscHPP3:       disc3,
			MarkupMTType:   mtType,
			MarkupMTAmount: mtAmt,
			MarkupGTType:   gtType,
			MarkupGTAmount: gtAmt,
			AgingMonth:     aging,
			CreatedBy:      meta.UserID,
		}
		hitungHargaBatch(&bm, s.ppn)

		if err := s.bm.Create(tx, &bm); err != nil {
			return err
		}
		if err := s.stock.CatatPenerimaan(tx, locked, &bm, meta.UserID); err != nil {
			return err
		}
		bm.Barang = locked
		ringkas := fmt.Sprintf("penerimaan %s batch %s qty %d", locked.KodeBarang, bm.NoBatch, bm.Qty)
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "barang_masuk.buat", EntityType: "barang_masuk",
			EntityID: &bm.ID, Ringkasan: &ringkas, DataSesudah: bm,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp, err := s.mapResponse(tx, &bm)
		if err != nil {
			return err
		}
		out = &resp
		return nil
	})
	return out, err
}

func (s *BarangMasukService) Ubah(ctx context.Context, id uint64, req dto.BarangMasukUpdateRequest, meta AuditMeta) (*dto.BarangMasukResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	var out *dto.BarangMasukResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bm, err := s.bm.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := *bm
		qtyLama := bm.Qty

		if req.NoFaktur != nil {
			bm.NoFaktur = strings.TrimSpace(*req.NoFaktur)
		}
		if req.NoBatch != nil {
			bm.NoBatch = strings.TrimSpace(*req.NoBatch)
		}
		if req.Exp != nil {
			d, err := parseDateField(*req.Exp, "exp")
			if err != nil {
				return err
			}
			bm.Exp = d
		}
		if req.TanggalMasuk != nil {
			d, err := parseDateField(*req.TanggalMasuk, "tanggal_masuk")
			if err != nil {
				return err
			}
			bm.TanggalMasuk = d
		}
		if req.AgingMonth != nil {
			bm.AgingMonth = *req.AgingMonth
		}
		if req.Qty != nil {
			bm.Qty = *req.Qty
		}

		harga := bm.Harga
		disc1, disc2, disc3 := bm.DiscHPP1, bm.DiscHPP2, bm.DiscHPP3
		mtType, mtAmt := bm.MarkupMTType, bm.MarkupMTAmount
		gtType, gtAmt := bm.MarkupGTType, bm.MarkupGTAmount

		if req.Harga != nil {
			harga, err = dto.ParseUang(*req.Harga, "harga")
			if err != nil {
				return validationField("harga", err.Error())
			}
		}
		if req.DiscHPP1 != nil {
			disc1, err = dto.ParseUangOpsional(*req.DiscHPP1, "disc_hpp_1")
			if err != nil {
				return validationField("disc_hpp_1", err.Error())
			}
		}
		if req.DiscHPP2 != nil {
			disc2, err = dto.ParseUangOpsional(*req.DiscHPP2, "disc_hpp_2")
			if err != nil {
				return validationField("disc_hpp_2", err.Error())
			}
		}
		if req.DiscHPP3 != nil {
			disc3, err = dto.ParseUangOpsional(*req.DiscHPP3, "disc_hpp_3")
			if err != nil {
				return validationField("disc_hpp_3", err.Error())
			}
		}
		if req.MarkupMTType != nil && *req.MarkupMTType != "" {
			mtType = domain.MarkupType(*req.MarkupMTType)
		}
		if req.MarkupMTAmount != nil {
			mtAmt, err = dto.ParseUangOpsional(*req.MarkupMTAmount, "markup_mt_amount")
			if err != nil {
				return validationField("markup_mt_amount", err.Error())
			}
		}
		if req.MarkupGTType != nil && *req.MarkupGTType != "" {
			gtType = domain.MarkupType(*req.MarkupGTType)
		}
		if req.MarkupGTAmount != nil {
			gtAmt, err = dto.ParseUangOpsional(*req.MarkupGTAmount, "markup_gt_amount")
			if err != nil {
				return validationField("markup_gt_amount", err.Error())
			}
		}
		if err := validateDiscRange(disc1, disc2, disc3); err != nil {
			return err
		}

		bm.Harga = harga
		bm.DiscHPP1 = disc1
		bm.DiscHPP2 = disc2
		bm.DiscHPP3 = disc3
		bm.MarkupMTType = mtType
		bm.MarkupMTAmount = mtAmt
		bm.MarkupGTType = gtType
		bm.MarkupGTAmount = gtAmt
		hitungHargaBatch(bm, s.ppn)

		if bm.Qty != qtyLama {
			locked, err := s.bm.LockBarangForUpdate(tx, bm.BarangID)
			if err != nil {
				return err
			}
			if err := s.stock.SesuaikanQtyPenerimaan(tx, locked, bm, qtyLama, bm.Qty); err != nil {
				return err
			}
		}

		if err := s.bm.Update(tx, bm); err != nil {
			return err
		}
		if hargaPricingBerubah(&snapshot, bm) {
			ket := "ubah penerimaan"
			log := buatPriceChangeLog(&snapshot, bm, uuid.NewString(), meta.UserID, &ket, time.Now().UTC())
			if err := s.harga.Create(tx, &log); err != nil {
				return err
			}
		}
		ringkas := fmt.Sprintf("ubah penerimaan #%d", bm.ID)
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "barang_masuk.ubah", EntityType: "barang_masuk",
			EntityID: &bm.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: *bm,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp, err := s.mapResponse(tx, bm)
		if err != nil {
			return err
		}
		out = &resp
		return nil
	})
	return out, err
}

func (s *BarangMasukService) Hapus(ctx context.Context, id uint64, meta AuditMeta) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bm, err := s.bm.FindByID(tx, id)
		if err != nil {
			return err
		}
		locked, err := s.bm.LockBarangForUpdate(tx, bm.BarangID)
		if err != nil {
			return err
		}
		if err := s.stock.HapusPenerimaan(tx, locked, bm); err != nil {
			return err
		}
		if err := s.bm.Delete(tx, id); err != nil {
			return err
		}
		ringkas := fmt.Sprintf("hapus penerimaan #%d", id)
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "barang_masuk.hapus", EntityType: "barang_masuk",
			EntityID: &id, Ringkasan: &ringkas, DataSebelum: *bm,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
}

func (s *BarangMasukService) resolveOrCreateBarang(tx *gorm.DB, req dto.BarangMasukCreateRequest, meta AuditMeta) (*domain.Barang, error) {
	kode := strings.TrimSpace(req.KodeBarang)
	existing, err := s.barang.FindByKode(tx, kode)
	if err == nil {
		if !existing.IsActive {
			return nil, validationField("kode_barang", "barang nonaktif")
		}
		return existing, nil
	}
	if err != domain.ErrTidakDitemukan {
		return nil, err
	}
	if !req.BuatBarangBaru {
		return nil, validationField("kode_barang", "barang tidak ditemukan; set buat_barang_baru=true untuk membuat master")
	}
	if strings.TrimSpace(req.NamaItem) == "" {
		return nil, validationField("nama_item", "wajib bila membuat barang baru")
	}
	if strings.TrimSpace(req.Brand) == "" {
		return nil, validationField("brand", "wajib bila membuat barang baru")
	}
	satuan := strings.TrimSpace(req.Satuan)
	if satuan == "" {
		satuan = "PCS"
	}
	b := domain.Barang{
		KodeBarang:      kode,
		NamaItem:        strings.TrimSpace(req.NamaItem),
		Brand:           strings.TrimSpace(req.Brand),
		Satuan:          satuan,
		MetodeAlokasi:   domain.AlokasiFEFO,
		ExpiryAlertDays: 30,
		IsActive:        true,
	}
	if err := s.barang.Create(tx, &b); err != nil {
		return nil, err
	}
	ringkas := "buat master barang " + b.KodeBarang + " via penerimaan"
	_ = s.audit.CatatLengkap(tx, repository.AuditTulis{
		UserID: meta.UserID, Aksi: "barang.buat", EntityType: "barang",
		EntityID: &b.ID, Ringkasan: &ringkas, DataSesudah: b,
		IPAddress: meta.IPAddress, RequestID: meta.RequestID,
	})
	return &b, nil
}

func hitungHargaBatch(bm *domain.BarangMasuk, ppn decimal.Decimal) {
	hpp := domain.HitungHPP(bm.Harga, bm.DiscHPP1, bm.DiscHPP2, bm.DiscHPP3)
	bm.HPP = money.RoundMoney(hpp)
	bm.HPPDenganPPN = money.RoundMoney(domain.HitungHPPDenganPPN(hpp, ppn))
	bm.HargaMT = money.RoundMoney(domain.HitungHargaChannel(bm.Harga, bm.MarkupMTAmount, bm.MarkupMTType))
	bm.HargaGT = money.RoundMoney(domain.HitungHargaChannel(bm.Harga, bm.MarkupGTAmount, bm.MarkupGTType))
}

func (s *BarangMasukService) mapResponse(db *gorm.DB, bm *domain.BarangMasuk) (dto.BarangMasukResponse, error) {
	qtyTersedia, err := s.stock.QtyTersediaBatch(db, bm)
	if err != nil {
		return dto.BarangMasukResponse{}, err
	}
	kode, nama, brand := "", "", ""
	if bm.Barang != nil {
		kode = bm.Barang.KodeBarang
		nama = bm.Barang.NamaItem
		brand = bm.Barang.Brand
	}
	return dto.BarangMasukResponse{
		ID:             bm.ID,
		BarangID:       bm.BarangID,
		KodeBarang:     kode,
		NamaItem:       nama,
		Brand:          brand,
		NoFaktur:       bm.NoFaktur,
		NoBatch:        bm.NoBatch,
		Exp:            formatDate(bm.Exp),
		TanggalMasuk:   formatDate(bm.TanggalMasuk),
		Qty:            bm.Qty,
		QtyTersedia:    qtyTersedia,
		Harga:          dto.FormatUang(bm.Harga),
		DiscHPP1:       dto.FormatUang(bm.DiscHPP1),
		DiscHPP2:       dto.FormatUang(bm.DiscHPP2),
		DiscHPP3:       dto.FormatUang(bm.DiscHPP3),
		HPP:            dto.FormatUang(bm.HPP),
		HPPDenganPPN:   dto.FormatUang(bm.HPPDenganPPN),
		MarkupMTType:   string(bm.MarkupMTType),
		MarkupMTAmount: dto.FormatUang(bm.MarkupMTAmount),
		MarkupGTType:   string(bm.MarkupGTType),
		MarkupGTAmount: dto.FormatUang(bm.MarkupGTAmount),
		HargaMT:        dto.FormatUang(bm.HargaMT),
		HargaGT:        dto.FormatUang(bm.HargaGT),
		AgingMonth:     bm.AgingMonth,
		CreatedAt:      bm.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func parsePricingInput(
	hargaS, d1, d2, d3, mtTypeS, mtAmtS, gtTypeS, gtAmtS string,
) (harga, disc1, disc2, disc3 decimal.Decimal, mtType, gtType domain.MarkupType, mtAmt, gtAmt decimal.Decimal, err error) {
	harga, err = dto.ParseUang(hargaS, "harga")
	if err != nil {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, validationField("harga", err.Error())
	}
	if harga.IsNegative() {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, validationField("harga", "harga tidak boleh negatif")
	}
	disc1, err = dto.ParseUangOpsional(d1, "disc_hpp_1")
	if err != nil {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, validationField("disc_hpp_1", err.Error())
	}
	disc2, err = dto.ParseUangOpsional(d2, "disc_hpp_2")
	if err != nil {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, validationField("disc_hpp_2", err.Error())
	}
	disc3, err = dto.ParseUangOpsional(d3, "disc_hpp_3")
	if err != nil {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, validationField("disc_hpp_3", err.Error())
	}
	if err := validateDiscRange(disc1, disc2, disc3); err != nil {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, err
	}
	mtType = domain.MarkupPercent
	if strings.TrimSpace(mtTypeS) != "" {
		mtType = domain.MarkupType(mtTypeS)
	}
	gtType = domain.MarkupPercent
	if strings.TrimSpace(gtTypeS) != "" {
		gtType = domain.MarkupType(gtTypeS)
	}
	mtAmt, err = dto.ParseUangOpsional(mtAmtS, "markup_mt_amount")
	if err != nil {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, validationField("markup_mt_amount", err.Error())
	}
	gtAmt, err = dto.ParseUangOpsional(gtAmtS, "markup_gt_amount")
	if err != nil {
		return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, validationField("markup_gt_amount", err.Error())
	}
	return harga, disc1, disc2, disc3, mtType, gtType, mtAmt, gtAmt, nil
}

func validateDiscRange(d1, d2, d3 decimal.Decimal) error {
	for _, p := range []struct {
		v     decimal.Decimal
		field string
	}{
		{d1, "disc_hpp_1"}, {d2, "disc_hpp_2"}, {d3, "disc_hpp_3"},
	} {
		if p.v.IsNegative() || p.v.GreaterThan(decimal.NewFromInt(100)) {
			return validationField(p.field, "harus antara 0 dan 100")
		}
	}
	return nil
}

func parseDateField(s, field string) (datatypes.Date, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return datatypes.Date{}, validationField(field, "format tanggal harus YYYY-MM-DD")
	}
	return datatypes.Date(t), nil
}

func formatDate(d datatypes.Date) string {
	t := time.Time(d)
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func validationField(field, msg string) error {
	return &appvalidator.Errors{Fields: []appvalidator.FieldError{{Field: field, Message: msg}}}
}
