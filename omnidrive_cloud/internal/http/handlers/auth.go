package handlers

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AuthHandler struct {
	app *appstate.App
}

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewAuthHandler(app *appstate.App) *AuthHandler {
	return &AuthHandler{app: app}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var payload registerRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Email = strings.TrimSpace(strings.ToLower(payload.Email))
	payload.Name = strings.TrimSpace(payload.Name)
	if _, err := mail.ParseAddress(payload.Email); err != nil {
		render.Error(w, http.StatusBadRequest, "Invalid email")
		return
	}
	if len(payload.Name) == 0 {
		render.Error(w, http.StatusBadRequest, "Name is required")
		return
	}
	if len(payload.Password) < 8 {
		render.Error(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	existing, err := h.app.Store.GetUserByEmail(r.Context(), payload.Email)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to query user")
		return
	}
	if existing != nil {
		render.Error(w, http.StatusConflict, "Email already exists")
		return
	}

	passwordHash, err := h.app.Tokens.HashPassword(payload.Password)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user, err := h.app.Store.CreateUser(r.Context(), store.CreateUserInput{
		ID:           uuid.NewString(),
		Email:        payload.Email,
		Name:         payload.Name,
		PasswordHash: passwordHash,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create user")
		return
	}
	h.app.Logger.Info("user registered", "user_id", user.ID, "email", user.Email)

	render.JSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	userWithPassword, err := h.app.Store.GetUserByEmail(r.Context(), payload.Email)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to query user")
		return
	}
	if userWithPassword == nil {
		render.Error(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	if err := h.app.Tokens.VerifyPassword(payload.Password, userWithPassword.PasswordHash); err != nil {
		render.Error(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := h.app.Tokens.IssueToken(userWithPassword.User.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to issue token")
		return
	}
	h.app.Logger.Info("user login succeeded", "user_id", userWithPassword.User.ID, "email", userWithPassword.User.Email)

	render.JSON(w, http.StatusOK, map[string]any{
		"accessToken": token,
		"tokenType":   "bearer",
		"user":        userWithPassword.User,
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	if user == nil {
		render.Error(w, http.StatusUnauthorized, "User not found")
		return
	}
	render.JSON(w, http.StatusOK, user)
}
