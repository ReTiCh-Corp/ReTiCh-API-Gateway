package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddlewareJWKS struct {
	jwks      *keyfunc.JWKS
	jwtIssuer string
}

func NewAuthMiddlewareJWKS(jwksURL, jwtIssuer string) (*AuthMiddlewareJWKS, error) {
	// Créer le JWKS avec refresh automatique
	options := keyfunc.Options{
		RefreshInterval: 1 * time.Hour,
		RefreshTimeout:  10 * time.Second,
		RefreshErrorHandler: func(err error) {
			log.Printf("JWKS refresh error: %v", err)
		},
	}

	jwks, err := keyfunc.Get(jwksURL, options)
	if err != nil {
		return nil, err
	}

	return &AuthMiddlewareJWKS{
		jwks:      jwks,
		jwtIssuer: jwtIssuer,
	}, nil
}

func (am *AuthMiddlewareJWKS) ValidateJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, am.jwks.Keyfunc)

		if err != nil || !token.Valid {
			log.Printf("JWT validation error: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Vérifier l'issuer si configuré
		if am.jwtIssuer != "" {
			if iss, ok := claims["iss"].(string); !ok || iss != am.jwtIssuer {
				http.Error(w, "Invalid token issuer", http.StatusUnauthorized)
				return
			}
		}

		// Extraire les claims utilisateur
		userID := getStringClaim(claims, "sub")
		email := getStringClaim(claims, "email")
		role := getStringClaim(claims, "role")

		// Ajouter au contexte
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, UserEmailKey, email)
		ctx = context.WithValue(ctx, UserRoleKey, role)

		// Ajouter les headers pour les microservices downstream
		r.Header.Set("X-User-ID", userID)
		r.Header.Set("X-User-Email", email)
		r.Header.Set("X-User-Role", role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (am *AuthMiddlewareJWKS) OptionalJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			next.ServeHTTP(w, r)
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, am.jwks.Keyfunc)

		if err == nil && token.Valid {
			userID := getStringClaim(claims, "sub")
			email := getStringClaim(claims, "email")
			role := getStringClaim(claims, "role")

			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, userID)
			ctx = context.WithValue(ctx, UserEmailKey, email)
			ctx = context.WithValue(ctx, UserRoleKey, role)

			r.Header.Set("X-User-ID", userID)
			r.Header.Set("X-User-Email", email)
			r.Header.Set("X-User-Role", role)

			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func (am *AuthMiddlewareJWKS) Close() {
	if am.jwks != nil {
		am.jwks.EndBackground()
	}
}

// Helper pour extraire les claims string
func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
