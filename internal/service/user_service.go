package service

import (
	"context"
	"strings"
	"time"

	"app/internal/domain"
	"app/internal/dto"
	"app/internal/pkg/clock"
	"app/internal/pkg/password"
	appvalidator "app/internal/pkg/validator"
	"app/internal/repository"
	"gorm.io/gorm"
)

// UserService administrasi pengguna (SB-07).
type UserService struct {
	db     *gorm.DB
	users  *repository.UserRepo
	tokens *repository.TokenRepo
	audit  *repository.AuditRepo
	clock  clock.Clock
}

func NewUserService(
	db *gorm.DB,
	users *repository.UserRepo,
	tokens *repository.TokenRepo,
	audit *repository.AuditRepo,
	clk clock.Clock,
) *UserService {
	if clk == nil {
		clk = clock.Real{}
	}
	return &UserService{db: db, users: users, tokens: tokens, audit: audit, clock: clk}
}

func (s *UserService) Daftar(ctx context.Context, q dto.UserListQuery) ([]dto.UserResponse, dto.PageMeta, error) {
	if err := appvalidator.Struct(q); err != nil {
		return nil, dto.PageMeta{}, err
	}
	if q.Role != "" && !roleDikenal(domain.Role(q.Role)) {
		return nil, dto.PageMeta{}, validationField("role", "role tidak valid")
	}
	q.Normalize()
	hasil, err := s.users.List(s.db.WithContext(ctx), q)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	out := make([]dto.UserResponse, 0, len(hasil.Items))
	for i := range hasil.Items {
		out = append(out, mapUser(&hasil.Items[i]))
	}
	return out, dto.NewPageMeta(q.Page, q.PerPage, hasil.Total), nil
}

func (s *UserService) Detail(ctx context.Context, id uint64) (*dto.UserResponse, error) {
	u, err := s.users.FindByID(s.db.WithContext(ctx), id)
	if err != nil {
		return nil, err
	}
	resp := mapUser(u)
	return &resp, nil
}

func (s *UserService) Buat(ctx context.Context, req dto.UserCreateRequest, meta AuditMeta) (*dto.UserResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	role := domain.Role(req.Role)
	if !roleFormDiizinkan(role) {
		return nil, validationField("role", "hanya sales atau afiliasi")
	}
	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, err
	}
	aktif := true
	if req.IsActive != nil {
		aktif = *req.IsActive
	}
	jk, err := parseJenisKelamin(req.JenisKelamin)
	if err != nil {
		return nil, err
	}

	u := domain.User{
		Name:         strings.TrimSpace(req.Name),
		Email:        strings.ToLower(strings.TrimSpace(req.Email)),
		Password:     hash,
		Role:         role,
		NoHP:         trimPtr(req.NoHP),
		NoKTP:        trimPtr(req.NoKTP),
		Alamat:       trimPtr(req.Alamat),
		JenisKelamin: jk,
		IsActive:     aktif,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.users.CreateMapped(tx, &u); err != nil {
			return err
		}
		ringkas := "buat user " + u.Email
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "user.buat", EntityType: "user",
			EntityID: &u.ID, Ringkasan: &ringkas, DataSesudah: mapUser(&u),
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
	if err != nil {
		return nil, err
	}
	resp := mapUser(&u)
	return &resp, nil
}

func (s *UserService) Ubah(ctx context.Context, id uint64, req dto.UserUpdateRequest, aktorUserID uint64, meta AuditMeta) (*dto.UserResponse, error) {
	if err := appvalidator.Struct(req); err != nil {
		return nil, err
	}
	var out *dto.UserResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.users.FindByID(tx, id)
		if err != nil {
			return err
		}
		snapshot := mapUser(sebelum)
		cabutSesi := false

		if req.Name != nil {
			sebelum.Name = strings.TrimSpace(*req.Name)
		}
		if req.Email != nil {
			sebelum.Email = strings.ToLower(strings.TrimSpace(*req.Email))
		}
		if req.Password != nil && strings.TrimSpace(*req.Password) != "" {
			hash, err := password.Hash(*req.Password)
			if err != nil {
				return err
			}
			sebelum.Password = hash
			cabutSesi = true
		}
		if req.Role != nil {
			roleBaru := domain.Role(*req.Role)
			if !roleFormDiizinkan(roleBaru) {
				return validationField("role", "hanya sales atau afiliasi")
			}
			if sebelum.Role == domain.RoleAdmin || sebelum.Role == domain.RoleSuperAdmin {
				return validationField("role", "role admin/super_admin tidak diubah lewat form ini")
			}
			if id == aktorUserID && roleBaru != sebelum.Role {
				return validationField("role", "tidak boleh mengubah role diri sendiri")
			}
			if roleBaru != sebelum.Role {
				sebelum.Role = roleBaru
				cabutSesi = true
			}
		}
		if req.NoHP != nil {
			sebelum.NoHP = trimPtr(req.NoHP)
		}
		if req.NoKTP != nil {
			sebelum.NoKTP = trimPtr(req.NoKTP)
		}
		if req.Alamat != nil {
			sebelum.Alamat = trimPtr(req.Alamat)
		}
		if req.JenisKelamin != nil {
			jk, err := parseJenisKelamin(req.JenisKelamin)
			if err != nil {
				return err
			}
			sebelum.JenisKelamin = jk
		}

		if err := s.users.Update(tx, sebelum); err != nil {
			return err
		}
		if cabutSesi {
			if err := s.tokens.RevokeAllForUser(tx, sebelum.ID, s.clock.Now()); err != nil {
				return err
			}
		}
		ringkas := "ubah user " + sebelum.Email
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "user.ubah", EntityType: "user",
			EntityID: &sebelum.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: mapUser(sebelum),
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp := mapUser(sebelum)
		out = &resp
		return nil
	})
	return out, err
}

func (s *UserService) SetStatus(ctx context.Context, id uint64, aktif bool, aktorUserID uint64, meta AuditMeta) (*dto.UserResponse, error) {
	var out *dto.UserResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sebelum, err := s.users.FindByID(tx, id)
		if err != nil {
			return err
		}
		if id == aktorUserID && !aktif {
			return validationField("is_active", "tidak boleh menonaktifkan diri sendiri")
		}
		if sebelum.Role == domain.RoleSuperAdmin && sebelum.IsActive && !aktif {
			n, err := s.users.CountActiveByRole(tx, domain.RoleSuperAdmin)
			if err != nil {
				return err
			}
			if n <= 1 {
				return validationField("is_active", "tidak boleh menonaktifkan super_admin aktif terakhir")
			}
		}
		snapshot := mapUser(sebelum)
		if err := s.users.SetActive(tx, id, aktif); err != nil {
			return err
		}
		sebelum.IsActive = aktif
		if !aktif {
			if err := s.tokens.RevokeAllForUser(tx, id, s.clock.Now()); err != nil {
				return err
			}
		}
		aksi := "user.nonaktifkan"
		if aktif {
			aksi = "user.aktifkan"
		}
		ringkas := aksi + " " + sebelum.Email
		if err := s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: aksi, EntityType: "user",
			EntityID: &sebelum.ID, Ringkasan: &ringkas,
			DataSebelum: snapshot, DataSesudah: mapUser(sebelum),
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		}); err != nil {
			return err
		}
		resp := mapUser(sebelum)
		out = &resp
		return nil
	})
	return out, err
}

func (s *UserService) Hapus(ctx context.Context, id uint64, aktorUserID uint64, meta AuditMeta) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		u, err := s.users.FindByID(tx, id)
		if err != nil {
			return err
		}
		if id == aktorUserID {
			return validationField("id", "tidak boleh menghapus diri sendiri")
		}
		if u.Role == domain.RoleAdmin {
			return validationField("id", "user admin tidak dapat dihapus")
		}
		if u.Role == domain.RoleSuperAdmin {
			n, err := s.users.CountActiveByRole(tx, domain.RoleSuperAdmin)
			if err != nil {
				return err
			}
			if u.IsActive && n <= 1 {
				return validationField("id", "tidak boleh menghapus super_admin aktif terakhir")
			}
		}
		snapshot := mapUser(u)
		if err := s.tokens.RevokeAllForUser(tx, id, s.clock.Now()); err != nil {
			return err
		}
		if err := s.users.Delete(tx, id); err != nil {
			return err
		}
		ringkas := "hapus user " + u.Email
		return s.audit.CatatLengkap(tx, repository.AuditTulis{
			UserID: meta.UserID, Aksi: "user.hapus", EntityType: "user",
			EntityID: &id, Ringkasan: &ringkas, DataSebelum: snapshot,
			IPAddress: meta.IPAddress, RequestID: meta.RequestID,
		})
	})
}

func mapUser(u *domain.User) dto.UserResponse {
	resp := dto.UserResponse{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Role:     string(u.Role),
		NoHP:     u.NoHP,
		NoKTP:    u.NoKTP,
		Alamat:   u.Alamat,
		IsActive: u.IsActive,
	}
	if u.JenisKelamin != nil {
		s := string(*u.JenisKelamin)
		resp.JenisKelamin = &s
	}
	if !u.CreatedAt.IsZero() {
		resp.CreatedAt = u.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !u.UpdatedAt.IsZero() {
		resp.UpdatedAt = u.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func roleFormDiizinkan(r domain.Role) bool {
	return r == domain.RoleSales || r == domain.RoleAfiliasi
}

func roleDikenal(r domain.Role) bool {
	switch r {
	case domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleAfiliasi, domain.RoleSales:
		return true
	default:
		return false
	}
}

func parseJenisKelamin(raw *string) (*domain.JenisKelamin, error) {
	if raw == nil {
		return nil, nil
	}
	v := strings.TrimSpace(*raw)
	if v == "" {
		return nil, nil
	}
	jk := domain.JenisKelamin(v)
	switch jk {
	case domain.JenisKelaminLaki, domain.JenisKelaminPerempuan:
		return &jk, nil
	default:
		return nil, validationField("jenis_kelamin", "harus L atau P")
	}
}
