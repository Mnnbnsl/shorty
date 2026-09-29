package handlers

import (
	"encoding/json"
	"net/http"

	"url-shortner/internal/auth"
	"url-shortner/internal/services"
)

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type UserDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type AuthResponse struct {
	Success      bool    `json:"success"`
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken,omitempty"`
	User         UserDTO `json:"user"`
}

func setRefreshCookie(w http.ResponseWriter, rawRefreshToken string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    rawRefreshToken,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   false, // set to true in production HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func getRefreshCookie(r *http.Request) string {
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var body RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	user, accessToken, refreshToken, err := services.UserSvc.Register(r.Context(), body.Email, body.Password, body.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	setRefreshCookie(w, refreshToken, 7*86400)

	writeJSON(w, http.StatusCreated, AuthResponse{
		Success:      true,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: UserDTO{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var body LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	user, accessToken, refreshToken, err := services.UserSvc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	setRefreshCookie(w, refreshToken, 7*86400)

	writeJSON(w, http.StatusOK, AuthResponse{
		Success:      true,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: UserDTO{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}

func RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	rawToken := getRefreshCookie(r)
	if rawToken == "" {
		var body RefreshRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		rawToken = body.RefreshToken
	}

	if rawToken == "" {
		writeError(w, http.StatusUnauthorized, "Missing refresh token")
		return
	}

	newAccess, newRefresh, err := services.UserSvc.RefreshToken(r.Context(), rawToken)
	if err != nil {
		setRefreshCookie(w, "", -1)
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	setRefreshCookie(w, newRefresh, 7*86400)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"accessToken":  newAccess,
		"refreshToken": newRefresh,
	})
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	rawToken := getRefreshCookie(r)
	if rawToken != "" {
		_ = services.UserSvc.Logout(r.Context(), rawToken)
	}
	setRefreshCookie(w, "", -1)

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func GetMeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	user, err := services.UserSvc.GetUserProfile(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"user": UserDTO{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
		},
	})
}
