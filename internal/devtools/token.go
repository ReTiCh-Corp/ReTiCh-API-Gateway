package devtools

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenRequest struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Audience string `json:"audience,omitempty"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type DevToolsHandler struct {
	jwtSecret string
	jwtIssuer string
}

func NewDevToolsHandler(jwtSecret, jwtIssuer string) *DevToolsHandler {
	return &DevToolsHandler{
		jwtSecret: jwtSecret,
		jwtIssuer: jwtIssuer,
	}
}

// GenerateToken génère un token JWT pour les tests
func (h *DevToolsHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Valeurs par défaut
	if req.UserID == "" {
		req.UserID = "test-user-123"
	}
	if req.Email == "" {
		req.Email = "test@retich.com"
	}
	if req.Role == "" {
		req.Role = "user"
	}

	// Créer le access token (15 min)
	accessToken, err := h.createToken(req, 15*time.Minute)
	if err != nil {
		http.Error(w, "Failed to create access token", http.StatusInternalServerError)
		return
	}

	// Créer le refresh token (7 jours)
	refreshToken, err := h.createToken(req, 7*24*time.Hour)
	if err != nil {
		http.Error(w, "Failed to create refresh token", http.StatusInternalServerError)
		return
	}

	response := TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes en secondes
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *DevToolsHandler) createToken(req TokenRequest, duration time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": req.UserID,
		"email":   req.Email,
		"role":    req.Role,
		"iss":     h.jwtIssuer,
		"iat":     now.Unix(),
		"exp":     now.Add(duration).Unix(),
	}

	if req.Audience != "" {
		claims["aud"] = req.Audience
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
