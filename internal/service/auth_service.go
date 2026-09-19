package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"app/internal/config"
	"app/internal/domain"
	"app/internal/pkg/clock"
	"app/internal/pkg/password"
	"app/internal/repository"
	"gorm.io/gorm"
)

type AuthService struct {
	db        *gorm.DB
	cfg       config.JWTConfig
	clock     clock.Clock
	userRepo  *repository.UserRepo
	tokenRepo *repository.TokenRepo
	auditRepo *repository.AuditRepo
}

func NewAuthService(
	db *gorm.DB,
	cfg config.JWTConfig,
	clk clock.Clock,
	userRepo *repository.UserRepo,
	tokenRepo *repository.TokenRepo,
	auditRepo *repository.AuditRepo,
) *AuthService {
	return &AuthService{
		db:        db,
		cfg:       cfg,
		clock:     clk,
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		auditRepo: auditRepo,
	}
}

type LoginMeta struct {
	UserAgent string
	IPAddress string
}

func (s *AuthService) Login(ctx context.Context, email, plain string, meta LoginMeta) (*domain.TokenPair, error) {
	user, err := s.userRepo.FindByEmail(s.db.WithContext(ctx), email)
	if err != nil {
		if err == domain.ErrTidakDitemukan {
			return nil, domain.ErrKredensialSalah
		}
		return nil, err
	}

	if !password.Verifikasi(user.Password, plain) {
		return nil, domain.ErrKredensialSalah
	}

	if !user.IsActive {
		return nil, domain.ErrAkunNonaktif
	}

	var pair *domain.TokenPair
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var terr error
		pair, terr = s.terbitkanPasanganToken(tx, user, meta)
		if terr != nil {
			return terr
		}
		uid := user.ID
		ringkasan := "login berhasil"
		return s.auditRepo.Catat(tx, &uid, "auth.login", "user", &uid, &ringkasan)
	})
	return pair, err
}

func (s *AuthService) Refresh(ctx context.Context, tokenMentah string, meta LoginMeta) (*domain.TokenPair, error) {
	hash := sha256Hex(tokenMentah)

	var pair *domain.TokenPair
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rt, err := s.tokenRepo.FindByHash(tx, hash)
		if err != nil {
			return domain.ErrRefreshTokenTidakValid
		}

		now := s.clock.Now()
		if rt.RevokedAt != nil {
			_ = s.tokenRepo.RevokeAllForUser(tx, rt.UserID, now)
			uid := rt.UserID
			ringkasan := "refresh token dipakai ulang"
			_ = s.auditRepo.Catat(tx, &uid, "auth.refresh_token_dipakai_ulang", "user", &uid, &ringkasan)
			return domain.ErrRefreshTokenTidakValid
		}

		if rt.ExpiresAt.Before(now) {
			return domain.ErrRefreshTokenKedaluwarsa
		}

		user, err := s.userRepo.FindByID(tx, rt.UserID)
		if err != nil || !user.IsActive {
			return domain.ErrAkunNonaktif
		}

		if err := s.tokenRepo.Revoke(tx, rt.ID, now); err != nil {
			return err
		}

		pair, err = s.terbitkanPasanganToken(tx, user, meta)
		return err
	})

	return pair, err
}

func (s *AuthService) Logout(ctx context.Context, tokenMentah string) error {
	if tokenMentah == "" {
		return nil
	}
	hash := sha256Hex(tokenMentah)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rt, err := s.tokenRepo.FindByHash(tx, hash)
		if err != nil {
			return nil // idempotent
		}
		if rt.RevokedAt != nil {
			return nil
		}
		now := s.clock.Now()
		if err := s.tokenRepo.Revoke(tx, rt.ID, now); err != nil {
			return err
		}
		uid := rt.UserID
		ringkasan := "logout"
		return s.auditRepo.Catat(tx, &uid, "auth.logout", "user", &uid, &ringkasan)
	})
}

func (s *AuthService) VerifikasiAccessToken(tokenStr string) (*domain.AccessClaims, error) {
	claims := &domain.AccessClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("algoritma tidak didukung")
		}
		return []byte(s.cfg.AccessSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrTidakDiizinkan
	}
	return claims, nil
}

func (s *AuthService) terbitkanPasanganToken(tx *gorm.DB, user *domain.User, meta LoginMeta) (*domain.TokenPair, error) {
	now := s.clock.Now()
	expiresAt := now.Add(s.cfg.AccessTTL)

	claims := domain.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(user.ID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
			Issuer:    "pkb-api",
			Audience:  []string{"pkb-web"},
		},
		Name:  user.Name,
		Email: user.Email,
		Role:  user.Role,
	}

	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(s.cfg.AccessSecret))
	if err != nil {
		return nil, err
	}

	rawRefresh, err := randomToken(32)
	if err != nil {
		return nil, err
	}

	ua := meta.UserAgent
	ip := meta.IPAddress
	rt := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: sha256Hex(rawRefresh),
		ExpiresAt: now.Add(s.cfg.RefreshTTL),
		UserAgent: strPtrOrNil(ua),
		IPAddress: strPtrOrNil(ip),
		CreatedAt: now,
	}
	if err := s.tokenRepo.Create(tx, rt); err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  access,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
		User:         user.Public(),
	}, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
