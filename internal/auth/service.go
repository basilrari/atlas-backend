package auth

import (
	"troo-backend/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// LoginInput for login request body.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SessionUserShape is the object stored in session and returned by /me (Express parity).
type SessionUserShape struct {
	UserID   string  `json:"user_id"`
	Fullname string  `json:"fullname"`
	Email    string  `json:"email"`
	Role     string  `json:"role"`
	OrgID    *string `json:"org_id"`
}

// UserFinder abstracts user lookup by email+password (for production GORM or test doubles).
type UserFinder interface {
	FindByEmailAndPassword(email, password string) (*models.User, error)
}

// GormUserFinder implements UserFinder using GORM and bcrypt.
type GormUserFinder struct{ DB *gorm.DB }

func (g *GormUserFinder) FindByEmailAndPassword(email, password string) (*models.User, error) {
	return LoginUser(g.DB, LoginInput{Email: email, Password: password})
}

// LoginUser finds user by email and verifies password. Returns user for session or error.
func LoginUser(db *gorm.DB, input LoginInput) (*models.User, error) {
	if input.Email == "" || input.Password == "" {
		return nil, ErrEmailPasswordRequired
	}
	var u models.User
	if err := db.Where("email = ?", input.Email).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInvalidEmail
		}
		return nil, err
	}
	if u.PasswordHash == "" {
		return nil, ErrInvalidEmail
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrIncorrectPassword
	}
	return &u, nil
}

// VerifyUser validates session user and returns the shape for /me (Express verifyUserService).
func VerifyUser(sessionUser interface{}) (*SessionUserShape, error) {
	if sessionUser == nil {
		return nil, ErrNotAuthenticated
	}
	m, ok := sessionUser.(map[string]interface{})
	if !ok {
		return nil, ErrNotAuthenticated
	}
	userID, _ := m["user_id"].(string)
	if userID == "" {
		return nil, ErrNotAuthenticated
	}
	out := &SessionUserShape{
		UserID:   userID,
		Fullname: str(m["fullname"]),
		Email:    str(m["email"]),
		Role:     str(m["role"]),
	}
	if o, ok := m["org_id"]; ok && o != nil {
		if s, ok := o.(string); ok {
			out.OrgID = &s
		}
	}
	return out, nil
}

func str(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
