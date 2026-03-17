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
	Phone    string `json:"phone"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

func NewAuthHandler(app *appstate.App) *AuthHandler {
	return &AuthHandler{app: app}
}

func normalizeUserPhone(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	var digits strings.Builder
	digits.Grow(len(trimmed))
	for _, char := range trimmed {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}

	normalized := digits.String()
	if strings.HasPrefix(normalized, "86") && len(normalized) == 13 {
		normalized = normalized[2:]
	}
	if len(normalized) == 11 && strings.HasPrefix(normalized, "1") {
		return normalized
	}
	return ""
}

func normalizeUserEmail(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return ""
	}
	return trimmed
}

func resolveAuthIdentifiers(phoneInput string, emailInput string) (string, string, string) {
	rawPhone := strings.TrimSpace(phoneInput)
	rawEmail := strings.TrimSpace(emailInput)

	if rawPhone != "" {
		phone := normalizeUserPhone(rawPhone)
		if phone == "" {
			return "", "", "Invalid phone"
		}
		if rawEmail != "" {
			if candidate := normalizeUserPhone(rawEmail); candidate == phone {
				return phone, "", ""
			}
			email := normalizeUserEmail(rawEmail)
			if email == "" {
				return "", "", "Invalid email"
			}
			return phone, email, ""
		}
		return phone, "", ""
	}

	if rawEmail == "" {
		return "", "", "Phone or email is required"
	}
	if phone := normalizeUserPhone(rawEmail); phone != "" {
		return phone, "", ""
	}
	email := normalizeUserEmail(rawEmail)
	if email == "" {
		return "", "", "Invalid phone or email"
	}
	return "", email, ""
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var payload registerRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	phone, email, identifierErr := resolveAuthIdentifiers(payload.Phone, payload.Email)
	if identifierErr != "" {
		render.Error(w, http.StatusBadRequest, identifierErr)
		return
	}
	if len(payload.Name) == 0 {
		render.Error(w, http.StatusBadRequest, "Name is required")
		return
	}
	if len(payload.Password) < 6 {
		render.Error(w, http.StatusBadRequest, "Password must be at least 6 characters")
		return
	}

	if phone != "" {
		existing, err := h.app.Store.GetUserByPhone(r.Context(), phone)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to query user")
			return
		}
		if existing != nil {
			render.Error(w, http.StatusConflict, "Phone already exists")
			return
		}
	}

	if email != "" {
		existing, err := h.app.Store.GetUserByEmail(r.Context(), email)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to query user")
			return
		}
		if existing != nil {
			render.Error(w, http.StatusConflict, "Email already exists")
			return
		}
	}

	passwordHash, err := h.app.Tokens.HashPassword(payload.Password)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user, err := h.app.Store.CreateUser(r.Context(), store.CreateUserInput{
		ID:           uuid.NewString(),
		Email:        email,
		Phone:        phone,
		Name:         payload.Name,
		PasswordHash: passwordHash,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create user")
		return
	}
	h.app.Logger.Info("user registered", "user_id", user.ID, "email", user.Email, "phone", user.Phone)

	render.JSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	phone, email, identifierErr := resolveAuthIdentifiers(payload.Phone, payload.Email)
	if identifierErr != "" {
		render.Error(w, http.StatusBadRequest, identifierErr)
		return
	}

	var userWithPassword *store.UserWithPassword
	var err error
	if phone != "" {
		userWithPassword, err = h.app.Store.GetUserByPhone(r.Context(), phone)
	} else {
		userWithPassword, err = h.app.Store.GetUserByEmail(r.Context(), email)
	}
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
	h.app.Logger.Info("user login succeeded", "user_id", userWithPassword.User.ID, "email", userWithPassword.User.Email, "phone", userWithPassword.User.Phone)

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
