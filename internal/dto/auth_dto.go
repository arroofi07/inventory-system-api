package dto

// LoginRequest body POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"admin@pkb.test"`
	Password string `json:"password" binding:"required,min=1" example:"rahasia123"`
}

// UserPublic data pengguna yang dikembalikan setelah autentikasi.
type UserPublic struct {
	ID       uint64 `json:"id" example:"1"`
	Name     string `json:"name" example:"Super Admin"`
	Email    string `json:"email" example:"admin@pkb.test"`
	Role     string `json:"role" example:"super_admin" enums:"super_admin,admin,afiliasi,sales"`
	IsActive bool   `json:"is_active" example:"true"`
}

// LoginData isi data respons login/refresh.
type LoginData struct {
	AccessToken string     `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType   string     `json:"token_type" example:"Bearer"`
	ExpiresIn   int64      `json:"expires_in" example:"900"`
	User        UserPublic `json:"user"`
}

// LoginResponse envelope sukses login/refresh.
type LoginResponse struct {
	Data LoginData `json:"data"`
}

// MeData profil dari GET /me.
type MeData struct {
	ID    uint64 `json:"id" example:"1"`
	Name  string `json:"name" example:"Super Admin"`
	Email string `json:"email" example:"admin@pkb.test"`
	Role  string `json:"role" example:"super_admin" enums:"super_admin,admin,afiliasi,sales"`
}

// MeResponse envelope GET /me.
type MeResponse struct {
	Data MeData `json:"data"`
}
