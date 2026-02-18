package user

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	usersvc "troo-backend/internal/application/user"
	"troo-backend/internal/domain"
	"troo-backend/internal/middleware"
	"troo-backend/internal/pkg/constants"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserTest(t *testing.T) (*Handlers, *usersvc.Service, *redis.Client, *gorm.DB) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		rdb.Close()
		mr.Close()
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.User{}))
	svc := &usersvc.Service{DB: db, Rdb: rdb}
	handlers := &Handlers{
		Service: svc,
		Config:  middleware.SessionConfig{AllowCrossSiteDev: false, IsProduction: false},
	}
	return handlers, svc, rdb, db
}

func setSessionUser(c *fiber.Ctx, userID, role string, orgID *string) {
	middleware.SetSessionUser(c, middleware.SessionUser{
		UserID: userID, Fullname: "Test", Email: "test@test.com", Role: role, OrgID: orgID,
	})
}

// Test CreateUser requires auth (401 without session).
func TestCreateUser_RequiresAuth(t *testing.T) {
	h, _, _, _ := setupUserTest(t)
	app := fiber.New()
	app.Use(middleware.RequireAuth())
	app.Post("/create-user", h.CreateUser)

	body, _ := json.Marshal(map[string]string{
		"user_name": "u1", "email": "u1@test.com", "password": "Pass1!word", "fullname": "User One",
	})
	req := httptest.NewRequest("POST", "/create-user", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// Test UpdateRole returns 403 for role without ASSIGN_ROLE (e.g. viewer).
func TestUpdateRole_ForbiddenForViewer(t *testing.T) {
	h, svc, rdb, db := setupUserTest(t)
	// Create two users: viewer (actor) and manager (target)
	uid1 := uuid.New()
	uid2 := uuid.New()
	require.NoError(t, db.Create(&domain.User{
		UserID: uid1, UserName: "viewer1", Email: "v@test.com", PasswordHash: "x", Fullname: "V", Role: constants.Viewer, OrgID: nil,
	}).Error)
	require.NoError(t, db.Create(&domain.User{
		UserID: uid2, UserName: "manager1", Email: "m@test.com", PasswordHash: "x", Fullname: "M", Role: constants.Manager, OrgID: nil,
	}).Error)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{"user_id": uid1.String(), "role": constants.Viewer, "org_id": nil})
		return c.Next()
	})
	app.Use(middleware.AuthorizePermission(constants.AssignRole))
	app.Patch("/update-role", h.UpdateRole)

	body, _ := json.Marshal(map[string]string{"user_id": uid2.String(), "role": constants.Admin})
	req := httptest.NewRequest("PATCH", "/update-role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	_ = svc
	_ = rdb
}

// Test RemoveUser returns 403 for role without REMOVE_USER (e.g. viewer).
func TestRemoveUser_ForbiddenForViewer(t *testing.T) {
	h, _, _, db := setupUserTest(t)
	uid1 := uuid.New()
	uid2 := uuid.New()
	require.NoError(t, db.Create(&domain.User{
		UserID: uid1, UserName: "viewer1", Email: "v2@test.com", PasswordHash: "x", Fullname: "V", Role: constants.Viewer, OrgID: nil,
	}).Error)
	require.NoError(t, db.Create(&domain.User{
		UserID: uid2, UserName: "other", Email: "o@test.com", PasswordHash: "x", Fullname: "O", Role: constants.Manager, OrgID: nil,
	}).Error)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{"user_id": uid1.String(), "role": constants.Viewer, "org_id": nil})
		return c.Next()
	})
	app.Use(middleware.AuthorizePermission(constants.RemoveUser))
	app.Delete("/remove-user", h.RemoveUser)

	body, _ := json.Marshal(map[string]string{"user_id": uid2.String()})
	req := httptest.NewRequest("DELETE", "/remove-user", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

// Test UpdateRole policy: "Users cannot modify their own role" (400).
func TestUpdateRole_SelfRoleChangeRejected(t *testing.T) {
	h, _, _, db := setupUserTest(t)
	uid := uuid.New()
	orgID := uuid.New()
	require.NoError(t, db.Create(&domain.User{
		UserID: uid, UserName: "admin1", Email: "a@test.com", PasswordHash: "x", Fullname: "A", Role: constants.Admin, OrgID: &orgID,
	}).Error)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		oid := orgID.String()
		c.Locals("user", map[string]interface{}{"user_id": uid.String(), "role": constants.Admin, "org_id": &oid})
		return c.Next()
	})
	app.Use(middleware.AuthorizePermission(constants.AssignRole))
	app.Patch("/update-role", h.UpdateRole)

	body, _ := json.Marshal(map[string]string{"user_id": uid.String(), "role": constants.Manager})
	req := httptest.NewRequest("PATCH", "/update-role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}

// Test RemoveUser policy: "You cannot remove yourself from the organization" (400).
func TestRemoveUser_SelfRemovalRejected(t *testing.T) {
	h, _, _, db := setupUserTest(t)
	uid := uuid.New()
	orgID := uuid.New()
	require.NoError(t, db.Create(&domain.User{
		UserID: uid, UserName: "admin1", Email: "a2@test.com", PasswordHash: "x", Fullname: "A", Role: constants.Admin, OrgID: &orgID,
	}).Error)

	app := fiber.New()
	oid := orgID.String()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", map[string]interface{}{"user_id": uid.String(), "role": constants.Admin, "org_id": &oid})
		return c.Next()
	})
	app.Use(middleware.AuthorizePermission(constants.RemoveUser))
	app.Delete("/remove-user", h.RemoveUser)

	body, _ := json.Marshal(map[string]string{"user_id": uid.String()})
	req := httptest.NewRequest("DELETE", "/remove-user", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
}
