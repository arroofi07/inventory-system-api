package domain

import "time"

type User struct {
	ID              uint64        `gorm:"primaryKey" json:"id"`
	Name            string        `gorm:"size:255;not null" json:"name"`
	Email           string        `gorm:"size:255;not null;uniqueIndex" json:"email"`
	EmailVerifiedAt *time.Time    `json:"email_verified_at,omitempty"`
	Password        string        `gorm:"size:255;not null" json:"-"`
	Role            Role          `gorm:"type:enum('super_admin','admin','afiliasi','sales');default:sales" json:"role"`
	NoHP            *string       `gorm:"column:no_hp;size:30" json:"no_hp,omitempty"`
	NoKTP           *string       `gorm:"column:no_ktp;size:32;uniqueIndex" json:"no_ktp,omitempty"`
	Alamat          *string       `gorm:"type:text" json:"alamat,omitempty"`
	JenisKelamin    *JenisKelamin `gorm:"type:enum('L','P')" json:"jenis_kelamin,omitempty"`
	IsActive        bool          `gorm:"not null;default:true" json:"is_active"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

func (User) TableName() string { return "users" }

type RefreshToken struct {
	ID        uint64     `gorm:"primaryKey" json:"id"`
	UserID    uint64     `gorm:"column:user_id;not null;index" json:"user_id"`
	TokenHash string     `gorm:"column:token_hash;size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	RevokedAt *time.Time `gorm:"column:revoked_at" json:"revoked_at,omitempty"`
	UserAgent *string    `gorm:"column:user_agent;size:255" json:"user_agent,omitempty"`
	IPAddress *string    `gorm:"column:ip_address;size:45" json:"ip_address,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
