package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey est le type utilisé pour les clés de contexte afin d'éviter les collisions.
type contextKey string

const (
	UserIDKey    contextKey = "userID"
	UserEmailKey contextKey = "userEmail"
	UserRoleKey  contextKey = "userRole"

	headerUserID    = "X-User-ID"
	headerUserEmail = "X-User-Email"
	headerUserRole  = "X-User-Role"
	headerAuth      = "Authorization"
	bearerPrefix    = "Bearer "
)

// AuthMiddleware valide les JWT signés avec HMAC HS256 (usage local/dev).
type AuthMiddleware struct {
	jwtSecret string
	jwtIssuer string
}

// NewAuthMiddleware crée un nouveau middleware d'authentification HMAC.
func NewAuthMiddleware(secret, issuer string) *AuthMiddleware {
	return &AuthMiddleware{jwtSecret: secret, jwtIssuer: issuer}
}

// parseAndValidateToken parse et valide un token JWT HMAC HS256.
// Retourne les claims si valide, une erreur sinon.
func (am *AuthMiddleware) parseAndValidateToken(tokenString string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(am.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}

	// Vérifier l'issuer si configuré
	if am.jwtIssuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != am.jwtIssuer {
			return nil, jwt.ErrTokenInvalidClaims
		}
	}

	return claims, nil
}

// enrichRequest ajoute les claims en headers et en contexte sur la requête.
func enrichRequest(r *http.Request, claims jwt.MapClaims) *http.Request {
	userID := getClaimString(claims, "user_id")
	email := getClaimString(claims, "email")
	role := getClaimString(claims, "role")

	r.Header.Set(headerUserID, userID)
	r.Header.Set(headerUserEmail, email)
	r.Header.Set(headerUserRole, role)

	ctx := r.Context()
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, UserEmailKey, email)
	ctx = context.WithValue(ctx, UserRoleKey, role)
	return r.WithContext(ctx)
}

// ValidateJWT est un middleware qui rejette les requêtes sans token valide.
func (am *AuthMiddleware) ValidateJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(headerAuth)
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
		if tokenString == authHeader {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := am.parseAndValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, enrichRequest(r, claims))
	})
}

// OptionalJWT est un middleware qui passe sans token, mais enrichit la requête si un token valide est présent.
func (am *AuthMiddleware) OptionalJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(headerAuth)
		if authHeader != "" {
			tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
			if tokenString != authHeader {
				if claims, err := am.parseAndValidateToken(tokenString); err == nil {
					r = enrichRequest(r, claims)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// getClaimString extrait une valeur string depuis les claims JWT.
func getClaimString(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
