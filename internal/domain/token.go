package domain

import (
	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	jwt.RegisteredClaims
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  Role   `json:"role"`
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // detik
	User         UserPublic
}

type UserPublic struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     Role   `json:"role"`
	IsActive bool   `json:"is_active"`
}

func (u User) Public() UserPublic {
	return UserPublic{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Role:     u.Role,
		IsActive: u.IsActive,
	}
}
