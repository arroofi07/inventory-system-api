package service

import (
	"fmt"

	"app/internal/config"
	"app/internal/domain"
	"app/internal/pkg/password"
	"app/internal/repository"
	"gorm.io/gorm"
)

// SeedSuperAdmin membuat atau memperbarui akun super_admin dari env.
func SeedSuperAdmin(db *gorm.DB, seed config.SeedConfig) error {
	if seed.SuperAdminEmail == "" || seed.SuperAdminPassword == "" {
		return fmt.Errorf("SEED_SUPER_ADMIN_EMAIL dan SEED_SUPER_ADMIN_PASSWORD wajib diisi")
	}

	hash, err := password.Hash(seed.SuperAdminPassword)
	if err != nil {
		return err
	}

	userRepo := repository.NewUserRepo()
	existing, err := userRepo.FindByEmail(db, seed.SuperAdminEmail)
	if err == domain.ErrTidakDitemukan {
		u := &domain.User{
			Name:     "Super Admin",
			Email:    seed.SuperAdminEmail,
			Password: hash,
			Role:     domain.RoleSuperAdmin,
			IsActive: true,
		}
		return userRepo.Create(db, u)
	}
	if err != nil {
		return err
	}

	return db.Model(existing).Updates(map[string]any{
		"password":  hash,
		"role":      domain.RoleSuperAdmin,
		"is_active": true,
		"name":      "Super Admin",
	}).Error
}
