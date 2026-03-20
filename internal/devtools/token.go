package devtools

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	contentTypeHeader = "Content-Type"
	contentTypeJSON   = "application/json"
)

// TokenRequest représente le corps de la requête pour générer un token.
type TokenRequest struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Audience string `json:"audience"`
}

// TokenResponse représente la réponse retournée par GenerateToken.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// DevToolsHandler gère les endpoints de développement (génération de tokens).
type DevToolsHandler struct {
	secret string
	issuer string
}

// NewDevToolsHandler crée un nouveau DevToolsHandler.
func NewDevToolsHandler(secret, issuer string) *DevToolsHandler {
	return &DevToolsHandler{secret: secret, issuer: issuer}
}

// GenerateToken génère un JWT de test signé avec HMAC HS256.
func (h *DevToolsHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Valeurs par défaut pour le développement
	if req.UserID == "" {
		req.UserID = "test-user-123"
	}
	if req.Email == "" {
		req.Email = "test@retich.com"
	}
	if req.Role == "" {
		req.Role = "user"
	}

	now := time.Now()
	expiresIn := 900 // 15 minutes

	// Claims pour l'access token
	accessClaims := jwt.MapClaims{
		"user_id": req.UserID,
		"email":   req.Email,
		"role":    req.Role,
		"iss":     h.issuer,
		"iat":     now.Unix(),
		"exp":     now.Add(time.Duration(expiresIn) * time.Second).Unix(),
	}
	if req.Audience != "" {
		accessClaims["aud"] = req.Audience
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(h.secret))
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	// Claims pour le refresh token (durée plus longue)
	refreshClaims := jwt.MapClaims{
		"user_id": req.UserID,
		"iss":     h.issuer,
		"iat":     now.Unix(),
		"exp":     now.Add(7 * 24 * time.Hour).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(h.secret))
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(TokenResponse{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    expiresIn,
	})
}
