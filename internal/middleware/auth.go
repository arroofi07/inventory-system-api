package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"app/internal/domain"
	"app/internal/httpx"
)

const (
	CtxKeyUserID = "user_id"
	CtxKeyRole   = "role"
	CtxKeyName   = "name"
	CtxKeyEmail  = "email"
)

type TokenVerifier interface {
	VerifikasiAccessToken(token string) (*domain.AccessClaims, error)
}

func Auth(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := ambilBearer(header)
		if !ok {
			httpx.BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI",
				"Token tidak ditemukan pada header Authorization")
			return
		}

		claims, err := verifier.VerifikasiAccessToken(token)
		if err != nil {
			httpx.BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI",
				"Token tidak valid atau sudah kedaluwarsa")
			return
		}

		userID, err := strconv.ParseUint(claims.Subject, 10, 64)
		if err != nil {
			httpx.BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI",
				"Token tidak valid atau sudah kedaluwarsa")
			return
		}

		c.Set(CtxKeyUserID, userID)
		c.Set(CtxKeyRole, claims.Role)
		c.Set(CtxKeyName, claims.Name)
		c.Set(CtxKeyEmail, claims.Email)
		c.Next()
	}
}

func RequirePermission(p domain.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, ok := c.Get(CtxKeyRole)
		if !ok {
			httpx.BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI", "Sesi tidak ditemukan")
			return
		}
		role, ok := roleVal.(domain.Role)
		if !ok || !role.Punya(p) {
			httpx.BalasError(c, http.StatusForbidden, "TIDAK_BERWENANG",
				"Role Anda tidak berwenang melakukan tindakan ini")
			return
		}
		c.Next()
	}
}

func RequireAnyPermission(ps ...domain.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, ok := c.Get(CtxKeyRole)
		if !ok {
			httpx.BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI", "Sesi tidak ditemukan")
			return
		}
		role, ok := roleVal.(domain.Role)
		if !ok || !role.PunyaSalahSatu(ps...) {
			httpx.BalasError(c, http.StatusForbidden, "TIDAK_BERWENANG",
				"Role Anda tidak berwenang melakukan tindakan ini")
			return
		}
		c.Next()
	}
}

func ambilBearer(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
