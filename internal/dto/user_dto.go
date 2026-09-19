package dto

// UserResponse tanpa password.
type UserResponse struct {
	ID           uint64  `json:"id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	Role         string  `json:"role"`
	NoHP         *string `json:"no_hp,omitempty"`
	NoKTP        *string `json:"no_ktp,omitempty"`
	Alamat       *string `json:"alamat,omitempty"`
	JenisKelamin *string `json:"jenis_kelamin,omitempty"`
	IsActive     bool    `json:"is_active"`
	CreatedAt    string  `json:"created_at,omitempty"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
}

// UserCreateRequest body POST /users — role hanya sales|afiliasi (SB-07).
type UserCreateRequest struct {
	Name         string  `json:"name" validate:"required,max=255"`
	Email        string  `json:"email" validate:"required,email,max=255"`
	Password     string  `json:"password" validate:"required,min=8,max=72"`
	Role         string  `json:"role" validate:"required,oneof=sales afiliasi"`
	NoHP         *string `json:"no_hp" validate:"omitempty,max=30"`
	NoKTP        *string `json:"no_ktp" validate:"omitempty,max=32"`
	Alamat       *string `json:"alamat"`
	JenisKelamin *string `json:"jenis_kelamin" validate:"omitempty,oneof=L P"`
	IsActive     *bool   `json:"is_active"`
}

// UserUpdateRequest body PATCH /users/{id}.
type UserUpdateRequest struct {
	Name         *string `json:"name" validate:"omitempty,max=255"`
	Email        *string `json:"email" validate:"omitempty,email,max=255"`
	Password     *string `json:"password" validate:"omitempty,min=8,max=72"`
	Role         *string `json:"role" validate:"omitempty,oneof=sales afiliasi"`
	NoHP         *string `json:"no_hp" validate:"omitempty,max=30"`
	NoKTP        *string `json:"no_ktp" validate:"omitempty,max=32"`
	Alamat       *string `json:"alamat"`
	JenisKelamin *string `json:"jenis_kelamin" validate:"omitempty,oneof=L P"`
}

// UserStatusRequest soft nonaktif / aktifkan.
type UserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UserListQuery filter daftar user.
type UserListQuery struct {
	ListQuery
	Role            string `form:"role"`
	IsActive        *bool  `form:"is_active"`
	IncludeInactive bool   `form:"include_inactive"`
}
