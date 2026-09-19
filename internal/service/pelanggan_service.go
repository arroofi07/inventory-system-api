package service

import (
	"context"
	"strconv"
	"strings"

	"app/internal/domain"
	"app/internal/dto"
	appvalidator "app/internal/pkg/validator"
	"app/internal/repository"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PelangganService master pelanggan (SB-06).
type PelangganService struct {
	db   *gorm.DB
	repo *repository.PelangganRepo
	audit *repository.AuditRepo
}

func NewPelangganService(db *gorm.DB, repo *repository.PelangganRepo, audit *repository.AuditRepo) *PelangganService {
	return &PelangganService{db: db, repo: repo, audit: audit}
}

func (s *PelangganService) Daftar(ctx context.Context, q dto.PelangganListQuery) ([]dto.PelangganResponse, dto.PageMeta, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, dto.PageMeta{}, err
	}
	q.Normalize()
	hasil, err := s.repo.List(s.db.WithContext(ctx), q)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	out := make([]dto.PelangganResponse, 0, len(hasil.Items))
	for i := range hasil.Items {
		out = append(out, mapPelanggan(&hasil.Items[i], false))
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), nil
}

// CariSelect2 mengembalikan format Select2 (hanya pelanggan aktif).
func (s *PelangganService) CariSelect2(ctx context.Context, q dto.PelangganCariQuery) (*dto.Select2Response, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, err
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PerPage < 1 {
		q.PerPage = 20
	}
	if q.PerPage > 50 {
		q.PerPage = 50
	}
	aktif := true
	hasil, err := s.repo.List(s.db.WithContext(ctx), dto.PelangganListQuery{
		ListQuery: dto.ListQuery{Q: q.Q, Page: q.Page, PerPage: q.PerPage, Sort: "nama_pelanggan"},
		IsActive:  &aktif,
	})
	if err != nil {
		return nil, err
	}
	results := make([]dto.Select2Result, 0, len(hasil.Items))
	for i := range hasil.Items {
		p := &hasil.Items[i]
		results = append(results, dto.Select2Result{
			ID:   p.KodePelanggan,
			Text: p.KodePelanggan + " — " + p.NamaPelanggan,
		})
	}
	lebih := int64(q.Page*q.PerPage) < hasil.Total
	return &dto.Select2Response{
		Results:    results,
		Pagination: dto.Select2Pagination{More: lebih},
	}, nil
}

func (s *PelangganService) Detail(ctx context.Context, id uint64) (*dto.PelangganResponse, error) {
	db := s.db.WithContext(ctx)
	p, err := s.repo.FindByID(db, id)
	if err != nil {
		return nil, err
	}
	punya, err := s.repo.PunyaTransaksi(db, p.KodePelanggan)
	if err != nil {
		return nil, err
	}
	resp := mapPelanggan(p, punya)
	return &resp, nil
}

func (s *PelangganService) DetailByKode(ctx context.Context, kode string) (*dto.PelangganResponse, error) {
	db := s.db.WithContext(ctx)
	p, err := s.repo.FindByKode(db, kode)
	if err != nil {
		return nil, err
	}
	punya, err := s.repo.PunyaTransaksi(db, p.KodePelanggan)
	if err != nil {
		return nil, err
	}
	resp := mapPelanggan(p, punya)
	return &resp, nil
}

// RiwayatTransaksi mengembalikan ringkasan transaksi outlet (maks 20).
func (s *PelangganService) RiwayatTransaksi(ctx context.Context, idOrKode string) ([]dto.RiwayatTransaksiItem, error) {
	db := s.db.WithContext(ctx)
	kode := strings.TrimSpace(idOrKode)
	if id, err := strconv.ParseUint(idOrKode, 10, 64); err == nil {
		p, err := s.repo.FindByID(db, id)
		if err != nil {
			return nil, err
		}
		kode = p.KodePelanggan
	} else {
		if _, err := s.repo.FindByKode(db, kode); err != nil {
			return nil, err
		}
	}

	type row struct {
		ID             uint64
		Tanggal        string
		TotalAkhir     string
		StatusApproval string
		NoTransaksi    *string
	}
	var rows []row
	err := db.Raw(`
		SELECT id,
			DATE_FORMAT(tanggal, '%Y-%m-%d') AS tanggal,
			CAST(total_akhir AS CHAR) AS total_akhir,
			status_approval,
			no_transaksi
		FROM transaksi_penjualan
		WHERE kode_pelanggan = ?
		ORDER BY tanggal DESC, id DESC
		LIMIT 20
	`, kode).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.RiwayatTransaksiItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.RiwayatTransaksiItem{
			ID: r.ID, Tanggal: r.Tanggal, TotalAkhir: r.TotalAkhir,
			StatusApproval: r.StatusApproval, NoTransaksi: r.NoTransaksi,
		})
	}
	return out, nil
}

func (s *PelangganService) Buat(ctx context.Context, req dto.PelangganCreateRequest, meta AuditMeta) (*dto.PelangganResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	if !domain.ChannelOutletValid(domain.ChannelOutlet(req.ChannelOutlet)) {
		return nil, validationField("channel_outlet", "harus salah satu dari 5 channel outlet")
	}
	tgl, err := parseDateField(req.TglRegistrasi, "tgl_registrasi")
	if err != nil {
		return nil, err
	}
	nominal, err := parseUangNonNegatif(req.NominalPengambilanPertama, "nominal_pengambilan_pertama")
	if err != nil {
		return nil, err
	}
	kredit, err := parseUangNonNegatif(req.EstimasiBatasKredit, "estimasi_batas_kredit")
	if err != nil {
		return nil, err
	}

	p := domain.Pelanggan{
		KodePelanggan:             strings.TrimSpace(req.KodePelanggan),
		NamaPelanggan:             strings.TrimSpace(req.NamaPelanggan),
		TglRegistrasi:             tgl,
		Phone:                     strings.TrimSpace(req.Phone),
		NPWPNIK:                   trimPtr(req.NPWPNIK),
		NamaPemilikNPWPNIK:        trimPtr(req.NamaPemilikNPWPNIK),
		AlamatNPWPNIK:             trimPtr(req.AlamatNPWPNIK),
		Territory:                 strings.TrimSpace(req.Territory),
		Distrik:                   strings.TrimSpace(req.Distrik),
		AlamatToko:                strings.TrimSpace(req.AlamatToko),
		RTRW:                      trimPtr(req.RTRW),
		Provinsi:                  strings.TrimSpace(req.Provinsi),
		Kabupaten:                 strings.TrimSpace(req.Kabupaten),
		Kecamatan:                 strings.TrimSpace(req.Kecamatan),
		Kelurahan:                 strings.TrimSpace(req.Kelurahan),
		KodePos:                   trimPtr(req.KodePos),
		ChannelOutlet:             domain.ChannelOutlet(req.ChannelOutlet),
		AlamatPengantaranBarang:   trimPtr(req.AlamatPengantaranBarang),
		JenisBangunan:             trimPtr(req.JenisBangunan),
		StatusBangunan:            trimPtr(req.StatusBangunan),
		NominalPengambilanPertama: nominal,
		EstimasiBatasKredit:       kredit,
		IsActive:                  true,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.repo.Create(tx, &p); err != nil {
			return err
		}
		ringkas := "buat pelanggan " + p.KodePelanggan
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "pelanggan.buat", EntityType: "pelanggan",
			EntityID: &p.ID, Ringkasan: &ringkas, DataSesudah: p,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
	if err != nil {
		return nil, err
	}
	resp := mapPelanggan(&p, false)
	return &resp, nil
}

func (s *PelangganService) Ubah(ctx context.Context, id uint64, req dto.PelangganUpdateRequest, meta AuditMeta) (*dto.PelangganResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	var out *dto.PelangganResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.repo.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := *sebelum

		if req.KodePelanggan != nil {
			kodeBaru := strings.TrimSpace(*req.KodePelanggan)
			if kodeBaru != sebelum.KodePelanggan {
				punya, err := s.repo.PunyaTransaksi(tx, sebelum.KodePelanggan)
				if err != nil {
					return err
				}
				if punya {
					return validationField("kode_pelanggan", "kode pelanggan tidak dapat diubah karena sudah punya transaksi")
				}
				sebelum.KodePelanggan = kodeBaru
			}
		}
		if req.NamaPelanggan != nil {
			sebelum.NamaPelanggan = strings.TrimSpace(*req.NamaPelanggan)
		}
		if req.TglRegistrasi != nil {
			tgl, err := parseDateField(*req.TglRegistrasi, "tgl_registrasi")
			if err != nil {
				return err
			}
			sebelum.TglRegistrasi = tgl
		}
		if req.Phone != nil {
			sebelum.Phone = strings.TrimSpace(*req.Phone)
		}
		if req.NPWPNIK != nil {
			sebelum.NPWPNIK = trimPtr(req.NPWPNIK)
		}
		if req.NamaPemilikNPWPNIK != nil {
			sebelum.NamaPemilikNPWPNIK = trimPtr(req.NamaPemilikNPWPNIK)
		}
		if req.AlamatNPWPNIK != nil {
			sebelum.AlamatNPWPNIK = trimPtr(req.AlamatNPWPNIK)
		}
		if req.Territory != nil {
			sebelum.Territory = strings.TrimSpace(*req.Territory)
		}
		if req.Distrik != nil {
			sebelum.Distrik = strings.TrimSpace(*req.Distrik)
		}
		if req.AlamatToko != nil {
			sebelum.AlamatToko = strings.TrimSpace(*req.AlamatToko)
		}
		if req.RTRW != nil {
			sebelum.RTRW = trimPtr(req.RTRW)
		}
		if req.Provinsi != nil {
			sebelum.Provinsi = strings.TrimSpace(*req.Provinsi)
		}
		if req.Kabupaten != nil {
			sebelum.Kabupaten = strings.TrimSpace(*req.Kabupaten)
		}
		if req.Kecamatan != nil {
			sebelum.Kecamatan = strings.TrimSpace(*req.Kecamatan)
		}
		if req.Kelurahan != nil {
			sebelum.Kelurahan = strings.TrimSpace(*req.Kelurahan)
		}
		if req.KodePos != nil {
			sebelum.KodePos = trimPtr(req.KodePos)
		}
		if req.ChannelOutlet != nil {
			ch := domain.ChannelOutlet(*req.ChannelOutlet)
			if !domain.ChannelOutletValid(ch) {
				return validationField("channel_outlet", "harus salah satu dari 5 channel outlet")
			}
			sebelum.ChannelOutlet = ch
		}
		if req.AlamatPengantaranBarang != nil {
			sebelum.AlamatPengantaranBarang = trimPtr(req.AlamatPengantaranBarang)
		}
		if req.JenisBangunan != nil {
			sebelum.JenisBangunan = trimPtr(req.JenisBangunan)
		}
		if req.StatusBangunan != nil {
			sebelum.StatusBangunan = trimPtr(req.StatusBangunan)
		}
		if req.NominalPengambilanPertama != nil {
			nominal, err := parseUangNonNegatif(req.NominalPengambilanPertama, "nominal_pengambilan_pertama")
			if err != nil {
				return err
			}
			sebelum.NominalPengambilanPertama = nominal
		}
		if req.EstimasiBatasKredit != nil {
			kredit, err := parseUangNonNegatif(req.EstimasiBatasKredit, "estimasi_batas_kredit")
			if err != nil {
				return err
			}
			sebelum.EstimasiBatasKredit = kredit
		}

		if err := s.repo.Update(tx, sebelum); err != nil {
			return err
		}
		ringkas := "ubah pelanggan " + sebelum.KodePelanggan
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "pelanggan.ubah", EntityType: "pelanggan",
			EntityID: &sebelum.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: *sebelum,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		punya, err := s.repo.PunyaTransaksi(tx, sebelum.KodePelanggan)
		if err != nil {
			return err
		}
		resp := mapPelanggan(sebelum, punya)
		out = &resp
		return nil
	})
	return out, err
}

func (s *PelangganService) SetStatus(ctx context.Context, id uint64, aktif bool, meta AuditMeta) (*dto.PelangganResponse, error) {
	var out *dto.PelangganResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.repo.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := *sebelum
		if err := s.repo.SetActive(tx, id, aktif); err != nil {
			return err
		}
		sebelum.IsActive = aktif
		aksi := "pelanggan.nonaktifkan"
		if aktif {
			aksi = "pelanggan.aktifkan"
		}
		ringkas := aksi + " " + sebelum.KodePelanggan
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: aksi, EntityType: "pelanggan",
			EntityID: &sebelum.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: *sebelum,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		punya, err := s.repo.PunyaTransaksi(tx, sebelum.KodePelanggan)
		if err != nil {
			return err
		}
		resp := mapPelanggan(sebelum, punya)
		out = &resp
		return nil
	})
	return out, err
}

func mapPelanggan(p *domain.Pelanggan, punyaTransaksi bool) dto.PelangganResponse {
	return dto.PelangganResponse{
		ID:                        p.ID,
		KodePelanggan:             p.KodePelanggan,
		NamaPelanggan:             p.NamaPelanggan,
		TglRegistrasi:             formatDate(p.TglRegistrasi),
		Phone:                     p.Phone,
		NPWPNIK:                   p.NPWPNIK,
		NamaPemilikNPWPNIK:        p.NamaPemilikNPWPNIK,
		AlamatNPWPNIK:             p.AlamatNPWPNIK,
		Territory:                 p.Territory,
		Distrik:                   p.Distrik,
		AlamatToko:                p.AlamatToko,
		RTRW:                      p.RTRW,
		Provinsi:                  p.Provinsi,
		Kabupaten:                 p.Kabupaten,
		Kecamatan:                 p.Kecamatan,
		Kelurahan:                 p.Kelurahan,
		KodePos:                   p.KodePos,
		ChannelOutlet:             string(p.ChannelOutlet),
		AlamatPengantaranBarang:   p.AlamatPengantaranBarang,
		JenisBangunan:             p.JenisBangunan,
		StatusBangunan:            p.StatusBangunan,
		NominalPengambilanPertama: dto.FormatUang(p.NominalPengambilanPertama),
		EstimasiBatasKredit:       dto.FormatUang(p.EstimasiBatasKredit),
		IsActive:                  p.IsActive,
		PunyaTransaksi:            punyaTransaksi,
	}
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func parseUangNonNegatif(raw *string, field string) (decimal.Decimal, error) {
	s := ""
	if raw != nil {
		s = *raw
	}
	d, err := dto.ParseUangOpsional(s, field)
	if err != nil {
		return decimal.Zero, validationField(field, err.Error())
	}
	if d.IsNegative() {
		return decimal.Zero, validationField(field, "tidak boleh negatif")
	}
	return d, nil
}
