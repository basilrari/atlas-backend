package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyUser_Nil(t *testing.T) {
	u, err := VerifyUser(nil)
	assert.Nil(t, u)
	assert.Equal(t, ErrNotAuthenticated, err)
}

func TestVerifyUser_EmptyMap(t *testing.T) {
	u, err := VerifyUser(map[string]interface{}{})
	assert.Nil(t, u)
	assert.Equal(t, ErrNotAuthenticated, err)
}

func TestVerifyUser_NoUserID(t *testing.T) {
	u, err := VerifyUser(map[string]interface{}{
		"fullname": "Test",
		"email":    "a@b.com",
	})
	assert.Nil(t, u)
	assert.Equal(t, ErrNotAuthenticated, err)
}

func TestVerifyUser_Valid(t *testing.T) {
	u, err := VerifyUser(map[string]interface{}{
		"user_id":  "550e8400-e29b-41d4-a716-446655440000",
		"fullname": "Test User",
		"email":    "test@example.com",
		"role":     "viewer",
		"org_id":   "660e8400-e29b-41d4-a716-446655440000",
	})
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", u.UserID)
	assert.Equal(t, "Test User", u.Fullname)
	assert.Equal(t, "test@example.com", u.Email)
	assert.Equal(t, "viewer", u.Role)
	require.NotNil(t, u.OrgID)
	assert.Equal(t, "660e8400-e29b-41d4-a716-446655440000", *u.OrgID)
}

func TestVerifyUser_NilOrgID(t *testing.T) {
	u, err := VerifyUser(map[string]interface{}{
		"user_id":  "550e8400-e29b-41d4-a716-446655440000",
		"fullname": "Test",
		"email":    "a@b.com",
		"role":     "viewer",
	})
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.Nil(t, u.OrgID)
}
