package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"app/internal/dto"
	"app/internal/httpx"
	"app/internal/service"
)

const RefreshCookieName = "refresh_token"
const RefreshCookiePath = "/api/v1/auth"

type AuthHandler struct {
	auth       *service.AuthService
	secure     bool
	refreshTTL time.Duration
}

func NewAuthHandler(auth *service.AuthService, secureCookie bool, refreshTTL time.Duration) *AuthHandler {
	return &AuthHandler{auth: auth, secure: secureCookie, refreshTTL: refreshTTL}
}

// Login mengautentikasi pengguna dan menerbitkan pasangan token.
//
// @Summary      Login
// @Description  Memverifikasi email/password. Access token dikembalikan di body;
// @Description  refresh token dikirim sebagai cookie HttpOnly (Path=/api/v1/auth).
// @Description  Kode error: KREDENSIAL_SALAH, AKUN_NONAKTIF, VALIDASI_GAGAL, TERLALU_BANYAK_PERCOBAAN.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      dto.LoginRequest  true  "Kredensial"
// @Success      200   {object}  dto.LoginResponse
// @Failure      401   {object}  dto.ErrorResponse  "KREDENSIAL_SALAH"
// @Failure      403   {object}  dto.ErrorResponse  "AKUN_NONAKTIF"
// @Failure      422   {object}  dto.ErrorResponse  "VALIDASI_GAGAL"
// @Failure      429   {object}  dto.ErrorResponse  "TERLALU_BANYAK_PERCOBAAN"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BalasError(c, http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Format email tidak valid atau field kosong")
		return
	}

	pair, err := h.auth.Login(c.Request.Context(), strings.TrimSpace(req.Email), req.Password, service.LoginMeta{
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		httpx.MapDomainError(c, err)
		return
	}

	h.setRefreshCookie(c, pair.RefreshToken)
	httpx.BalasOK(c, dto.LoginData{
		AccessToken: pair.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   pair.ExpiresIn,
		User: dto.UserPublic{
			ID:       pair.User.ID,
			Name:     pair.User.Name,
			Email:    pair.User.Email,
			Role:     string(pair.User.Role),
			IsActive: pair.User.IsActive,
		},
	})
}

// Refresh menerbitkan pasangan token baru dari cookie refresh.
//
// @Summary      Refresh token
// @Description  Memakai cookie refresh_token. Token lama dicabut (rotasi).
// @Description  Pemakaian ulang token yang sudah dicabut mencabut seluruh sesi pengguna.
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  dto.LoginResponse
// @Failure      401  {object}  dto.ErrorResponse  "TIDAK_TERAUTENTIKASI"
// @Failure      403  {object}  dto.ErrorResponse  "AKUN_NONAKTIF"
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	raw, err := c.Cookie(RefreshCookieName)
	if err != nil || raw == "" {
		httpx.BalasError(c, http.StatusUnauthorized, "TIDAK_TERAUTENTIKASI", "Refresh token tidak ditemukan")
		return
	}

	pair, err := h.auth.Refresh(c.Request.Context(), raw, service.LoginMeta{
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		h.clearRefreshCookie(c)
		httpx.MapDomainError(c, err)
		return
	}

	h.setRefreshCookie(c, pair.RefreshToken)
	httpx.BalasOK(c, dto.LoginData{
		AccessToken: pair.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   pair.ExpiresIn,
		User: dto.UserPublic{
			ID:       pair.User.ID,
			Name:     pair.User.Name,
			Email:    pair.User.Email,
			Role:     string(pair.User.Role),
			IsActive: pair.User.IsActive,
		},
	})
}

// Logout mencabut refresh token dan menghapus cookie.
//
// @Summary      Logout
// @Description  Mencabut refresh token aktif (idempotent).
// @Tags         Auth
// @Success      204  "Sesi diakhiri"
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(RefreshCookieName)
	_ = h.auth.Logout(c.Request.Context(), raw)
	h.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, value string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(RefreshCookieName, value, int(h.refreshTTL.Seconds()), RefreshCookiePath, "", h.secure, true)
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(RefreshCookieName, "", -1, RefreshCookiePath, "", h.secure, true)
}
