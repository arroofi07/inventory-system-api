package service

import (
	"context"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	appvalidator "app/internal/pkg/validator"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PromoService master promo (SB-08).
type PromoService struct {
	db     *gorm.DB
	promo  *repository.PromoRepo
	barang *repository.BarangRepo
	audit  *repository.AuditRepo
	clock  clock.Clock
}

func NewPromoService(
	db *gorm.DB,
	promo *repository.PromoRepo,
	barang *repository.BarangRepo,
	audit *repository.AuditRepo,
	clk clock.Clock,
) *PromoService {
	if clk == nil {
		clk = clock.Real{}
	}
	return &PromoService{db: db, promo: promo, barang: barang, audit: audit, clock: clk}
}

func (s *PromoService) Daftar(ctx context.Context, q dto.PromoListQuery) ([]dto.PromoResponse, dto.PageMeta, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, dto.PageMeta{}, err
	}
	q.Normalize()
	hasil, err := s.promo.List(s.db.WithContext(ctx), q, s.clock.Now())
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	out := make([]dto.PromoResponse, 0, len(hasil.Items))
	for i := range hasil.Items {
		out = append(out, mapPromo(&hasil.Items[i]))
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), nil
}

func (s *PromoService) Detail(ctx context.Context, id uint64) (*dto.PromoResponse, error) {
	p, err := s.promo.FindByID(s.db.WithContext(ctx), id)
	if err != nil {
		return nil, err
	}
	resp := mapPromo(p)
	return &resp, nil
}

func (s *PromoService) Buat(ctx context.Context, req dto.PromoCreateRequest, meta AuditMeta) (*dto.PromoResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	fields, err := s.parsePromoFields(promoFieldInput{
		TipePromo:          req.TipePromo,
		BuyQty:             req.BuyQty,
		GetQty:             req.GetQty,
		BonusQty:           req.BonusQty,
		DiscountPercentage: req.DiscountPercentage,
		DiscountAmount:     req.DiscountAmount,
		MinQty:             req.MinQty,
		MinAmount:          req.MinAmount,
		MaxApplications:    req.MaxApplications,
		KodeBarang:         req.KodeBarang,
		TanggalMulai:       &req.TanggalMulai,
		TanggalBerakhir:    &req.TanggalBerakhir,
		wajibTipe:          true,
		wajibTanggal:       true,
	})
	if err != nil {
		return nil, err
	}

	aktif := true
	if req.IsActive != nil {
		aktif = *req.IsActive
	}

	var out *dto.PromoResponse
	const maxCoba = 3
	for coba := 0; coba < maxCoba; coba++ {
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			kode := ""
			if req.KodePromo != nil && strings.TrimSpace(*req.KodePromo) != "" {
				kode = strings.TrimSpace(*req.KodePromo)
			} else {
				generated, err := s.promo.NextKodePromo(tx, s.clock.Now())
				if err != nil {
					return err
				}
				kode = generated
			}
			if err := s.pastikanKodeBarang(tx, fields.kodeBarang); err != nil {
				return err
			}

			p := domain.Promo{
				KodePromo:          kode,
				NamaPromo:          strings.TrimSpace(req.NamaPromo),
				Deskripsi:          trimPtr(req.Deskripsi),
				TipePromo:          fields.tipe,
				BuyQty:             fields.buyQty,
				GetQty:             fields.getQty,
				BonusQty:           fields.bonusQty,
				DiscountPercentage: fields.discPct,
				DiscountAmount:     fields.discAmt,
				MinQty:             fields.minQty,
				MinAmount:          fields.minAmt,
				MaxApplications:    fields.maxApp,
				KodeBarang:         fields.kodeBarang,
				TanggalMulai:       fields.mulai,
				TanggalBerakhir:    fields.akhir,
				IsActive:           aktif,
				SyaratKetentuan:    trimPtr(req.SyaratKetentuan),
				CreatedBy:          meta.UserID,
			}
			if err := s.promo.Create(tx, &p); err != nil {
				return err
			}
			ringkas := "buat promo " + p.KodePromo
			if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
				UserID: meta.UserID, Aksi: "promo.buat", EntityType: "promo",
				EntityID: &p.ID, Ringkasan: &ringkas, DataSesudah: mapPromo(&p),
				IPAddress: meta.IPAddress, RequestID: meta.RequestID,
			}); err != nil {
				return err
			}
			resp := mapPromo(&p)
			out = &resp
			return nil
		})
		if err == nil {
			return out, nil
		}
		if err == domain.ErrDuplikat && (req.KodePromo == nil || strings.TrimSpace(*req.KodePromo) == "") {
			continue
		}
		return nil, err
	}
	return nil, domain.ErrDuplikat
}

func (s *PromoService) Ubah(ctx context.Context, id uint64, req dto.PromoUpdateRequest, meta AuditMeta) (*dto.PromoResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	var out *dto.PromoResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.promo.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := mapPromo(sebelum)

		tipe := string(sebelum.TipePromo)
		if req.TipePromo != nil {
			tipe = *req.TipePromo
		}
		buy := sebelum.BuyQty
		if req.BuyQty != nil {
			buy = req.BuyQty
		}
		get := sebelum.GetQty
		if req.GetQty != nil {
			get = req.GetQty
		}
		bonus := &sebelum.BonusQty
		if req.BonusQty != nil {
			bonus = req.BonusQty
		}
		minQty := &sebelum.MinQty
		if req.MinQty != nil {
			minQty = req.MinQty
		}
		maxApp := sebelum.MaxApplications
		if req.MaxApplications != nil {
			maxApp = req.MaxApplications
		}
		kodeBarang := sebelum.KodeBarang
		if req.KodeBarang != nil {
			kodeBarang = trimPtr(req.KodeBarang)
		}

		discPctS := dto.FormatUang(sebelum.DiscountPercentage)
		if req.DiscountPercentage != nil {
			discPctS = *req.DiscountPercentage
		}
		discAmtS := dto.FormatUang(sebelum.DiscountAmount)
		if req.DiscountAmount != nil {
			discAmtS = *req.DiscountAmount
		}
		minAmtS := dto.FormatUang(sebelum.MinAmount)
		if req.MinAmount != nil {
			minAmtS = *req.MinAmount
		}
		mulaiS := formatDate(sebelum.TanggalMulai)
		if req.TanggalMulai != nil {
			mulaiS = *req.TanggalMulai
		}
		akhirS := formatDate(sebelum.TanggalBerakhir)
		if req.TanggalBerakhir != nil {
			akhirS = *req.TanggalBerakhir
		}

		fields, err := s.parsePromoFields(promoFieldInput{
			TipePromo:          tipe,
			BuyQty:             buy,
			GetQty:             get,
			BonusQty:           bonus,
			DiscountPercentage: &discPctS,
			DiscountAmount:     &discAmtS,
			MinQty:             minQty,
			MinAmount:          &minAmtS,
			MaxApplications:    maxApp,
			KodeBarang:         kodeBarang,
			TanggalMulai:       &mulaiS,
			TanggalBerakhir:    &akhirS,
			wajibTipe:          true,
			wajibTanggal:       true,
		})
		if err != nil {
			return err
		}
		if err := s.pastikanKodeBarang(tx, fields.kodeBarang); err != nil {
			return err
		}

		if req.NamaPromo != nil {
			sebelum.NamaPromo = strings.TrimSpace(*req.NamaPromo)
		}
		if req.Deskripsi != nil {
			sebelum.Deskripsi = trimPtr(req.Deskripsi)
		}
		if req.SyaratKetentuan != nil {
			sebelum.SyaratKetentuan = trimPtr(req.SyaratKetentuan)
		}
		sebelum.TipePromo = fields.tipe
		sebelum.BuyQty = fields.buyQty
		sebelum.GetQty = fields.getQty
		sebelum.BonusQty = fields.bonusQty
		sebelum.DiscountPercentage = fields.discPct
		sebelum.DiscountAmount = fields.discAmt
		sebelum.MinQty = fields.minQty
		sebelum.MinAmount = fields.minAmt
		sebelum.MaxApplications = fields.maxApp
		sebelum.KodeBarang = fields.kodeBarang
		sebelum.TanggalMulai = fields.mulai
		sebelum.TanggalBerakhir = fields.akhir

		if err := s.promo.Update(tx, sebelum); err != nil {
			return err
		}
		ringkas := "ubah promo " + sebelum.KodePromo
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "promo.ubah", EntityType: "promo",
			EntityID: &sebelum.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: mapPromo(sebelum),
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp := mapPromo(sebelum)
		out = &resp
		return nil
	})
	return out, err
}

func (s *PromoService) SetStatus(ctx context.Context, id uint64, aktif bool, meta AuditMeta) (*dto.PromoResponse, error) {
	var out *dto.PromoResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.promo.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := mapPromo(sebelum)
		if err := s.promo.SetActive(tx, id, aktif); err != nil {
			return err
		}
		sebelum.IsActive = aktif
		aksi := "promo.nonaktifkan"
		if aktif {
			aksi = "promo.aktifkan"
		}
		ringkas := aksi + " " + sebelum.KodePromo
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: aksi, EntityType: "promo",
			EntityID: &sebelum.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: mapPromo(sebelum),
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp := mapPromo(sebelum)
		out = &resp
		return nil
	})
	return out, err
}

func (s *PromoService) Hapus(ctx context.Context, id uint64, meta AuditMeta) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		p, err := s.promo.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := mapPromo(p)
		if err := s.promo.Delete(tx, id); err != nil {
			return err
		}
		ringkas := "hapus promo " + p.KodePromo
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "promo.hapus", EntityType: "promo",
			EntityID: &id, Ringkasan: &ringkas, DataSebelum: snapshot,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
}

type promoFieldInput struct {
	TipePromo          string
	BuyQty             *int
	GetQty             *int
	BonusQty           *int
	DiscountPercentage *string
	DiscountAmount     *string
	MinQty             *int
	MinAmount          *string
	MaxApplications    *int
	KodeBarang         *string
	TanggalMulai       *string
	TanggalBerakhir    *string
	wajibTipe          bool
	wajibTanggal       bool
}

type promoFields struct {
	tipe       domain.TipePromo
	buyQty     *int
	getQty     *int
	bonusQty   int
	discPct    decimal.Decimal
	discAmt    decimal.Decimal
	minQty     int
	minAmt     decimal.Decimal
	maxApp     *int
	kodeBarang *string
	mulai      datatypes.Date
	akhir      datatypes.Date
}

func (s *PromoService) parsePromoFields(in promoFieldInput) (promoFields, error) {
	var out promoFields
	tipe := domain.TipePromo(strings.TrimSpace(in.TipePromo))
	if !tipePromoValid(tipe) {
		return out, validationField("tipe_promo", "harus salah satu dari 4 tipe promo")
	}
	out.tipe = tipe

	out.minQty = 1
	if in.MinQty != nil {
		if *in.MinQty < 1 {
			return out, validationField("min_qty", "minimal 1")
		}
		out.minQty = *in.MinQty
	}
	minAmt, err := dto.ParseUangOpsional(ptrStr(in.MinAmount), "min_amount")
	if err != nil {
		return out, validationField("min_amount", err.Error())
	}
	if minAmt.IsNegative() {
		return out, validationField("min_amount", "tidak boleh negatif")
	}
	out.minAmt = minAmt

	if in.MaxApplications != nil {
		if *in.MaxApplications < 1 {
			return out, validationField("max_applications", "minimal 1")
		}
		out.maxApp = in.MaxApplications
	}
	out.kodeBarang = trimPtr(in.KodeBarang)

	switch tipe {
	case domain.PromoBuyXGetY:
		if in.BuyQty == nil || *in.BuyQty < 1 {
			return out, validationField("buy_qty", "wajib dan minimal 1 untuk buy_x_get_y")
		}
		if in.GetQty == nil || *in.GetQty < 1 {
			return out, validationField("get_qty", "wajib dan minimal 1 untuk buy_x_get_y")
		}
		out.buyQty = in.BuyQty
		out.getQty = in.GetQty
	case domain.PromoBonusQty:
		if in.BonusQty == nil || *in.BonusQty < 1 {
			return out, validationField("bonus_qty", "wajib dan minimal 1 untuk bonus_qty")
		}
		out.bonusQty = *in.BonusQty
	case domain.PromoPercentageDiscount:
		pct, err := dto.ParseUang(ptrStr(in.DiscountPercentage), "discount_percentage")
		if err != nil {
			return out, validationField("discount_percentage", err.Error())
		}
		// Server wajib: 0.01–100 (perbaikan bug browser-only).
		if pct.LessThanOrEqual(decimal.Zero) || pct.GreaterThan(decimal.NewFromInt(100)) {
			return out, validationField("discount_percentage", "harus antara 0,01 dan 100")
		}
		out.discPct = pct
	case domain.PromoFixedDiscount:
		amt, err := dto.ParseUang(ptrStr(in.DiscountAmount), "discount_amount")
		if err != nil {
			return out, validationField("discount_amount", err.Error())
		}
		if amt.LessThan(decimal.NewFromInt(1)) {
			return out, validationField("discount_amount", "minimal 1")
		}
		out.discAmt = amt
	default:
		return out, validationField("tipe_promo", "tipe tidak dikenal")
	}

	if in.wajibTanggal {
		if in.TanggalMulai == nil || in.TanggalBerakhir == nil {
			return out, validationField("tanggal_mulai", "periode wajib")
		}
		mulai, err := parseDateField(*in.TanggalMulai, "tanggal_mulai")
		if err != nil {
			return out, err
		}
		akhir, err := parseDateField(*in.TanggalBerakhir, "tanggal_berakhir")
		if err != nil {
			return out, err
		}
		if time.Time(akhir).Before(time.Time(mulai)) {
			return out, validationField("tanggal_berakhir", "harus sama atau setelah tanggal_mulai")
		}
		out.mulai = mulai
		out.akhir = akhir
	}
	return out, nil
}

func (s *PromoService) pastikanKodeBarang(tx *gorm.DB, kode *string) error {
	if kode == nil || *kode == "" {
		return nil
	}
	_, err := s.barang.FindByKode(tx, *kode)
	if err == domain.ErrTidakDitemukan {
		return validationField("kode_barang", "barang tidak ditemukan")
	}
	return err
}

func mapPromo(p *domain.Promo) dto.PromoResponse {
	return dto.PromoResponse{
		ID:                 p.ID,
		KodePromo:          p.KodePromo,
		NamaPromo:          p.NamaPromo,
		Deskripsi:          p.Deskripsi,
		TipePromo:          string(p.TipePromo),
		BuyQty:             p.BuyQty,
		GetQty:             p.GetQty,
		BonusQty:           p.BonusQty,
		DiscountPercentage: dto.FormatUang(p.DiscountPercentage),
		DiscountAmount:     dto.FormatUang(p.DiscountAmount),
		MinQty:             p.MinQty,
		MinAmount:          dto.FormatUang(p.MinAmount),
		MaxApplications:    p.MaxApplications,
		KodeBarang:         p.KodeBarang,
		TanggalMulai:       formatDate(p.TanggalMulai),
		TanggalBerakhir:    formatDate(p.TanggalBerakhir),
		IsActive:           p.IsActive,
		SyaratKetentuan:    p.SyaratKetentuan,
	}
}

func tipePromoValid(t domain.TipePromo) bool {
	switch t {
	case domain.PromoBuyXGetY, domain.PromoBonusQty, domain.PromoPercentageDiscount, domain.PromoFixedDiscount:
		return true
	default:
		return false
	}
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
