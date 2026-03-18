# Architecture détaillée de ReTiCh API Gateway

> Documentation technique complète expliquant le fonctionnement interne de l'API Gateway

## Table des matières

- [Vue d'ensemble](#vue-densemble)
- [Rôle et responsabilités](#rôle-et-responsabilités)
- [Architecture technique](#architecture-technique)
- [Flux de requêtes](#flux-de-requêtes)
- [Composants détaillés](#composants-détaillés)
- [Sécurité](#sécurité)
- [Performance et scalabilité](#performance-et-scalabilité)

---

## Vue d'ensemble

### Qu'est-ce que l'API Gateway ?

L'**API Gateway** est le **point d'entrée unique** de la plateforme ReTiCh. C'est un **reverse proxy intelligent** qui :

1. **Reçoit** toutes les requêtes des clients (web, mobile, applications tierces)
2. **Authentifie** les utilisateurs en validant leurs tokens JWT
3. **Route** les requêtes vers le bon microservice
4. **Injecte** les informations utilisateur dans les headers
5. **Retourne** la réponse au client

### Pourquoi un API Gateway ?

Sans API Gateway, chaque client devrait :
- Connaître l'URL de chaque microservice
- Gérer l'authentification séparément pour chaque service
- Dupliquer la logique de sécurité partout

Avec l'API Gateway :
- **Point d'entrée unique** : Une seule URL (exemple : `https://api.retich.com`)
- **Authentification centralisée** : validation JWT en un seul endroit
- **Isolation des microservices** : ils ne sont pas exposés publiquement
- **Évolution facilitée** : ajouter/retirer des services sans impacter les clients

> **État actuel** : Le service Auth est déjà déployé. Les services User et Messaging seront ajoutés progressivement à la plateforme.

---

## Rôle et responsabilités

### Ce que fait l'API Gateway

✅ **Routage** : Rediriger les requêtes vers le bon microservice (Auth, User, Messaging, etc.)
✅ **Authentification** : Valider les tokens JWT avec JWKS (clés publiques RS256)
✅ **Enrichissement** : Ajouter `X-User-ID`, `X-User-Email`, `X-User-Role` aux requêtes
✅ **Logging** : Tracer toutes les requêtes (avec masquage des tokens)
✅ **Health checks** : Vérifier que l'API Gateway est en vie

> **Note** : Les routes vers User et Messaging sont déjà configurées dans le code, prêtes à être utilisées quand ces services seront déployés.

### Ce que l'API Gateway ne fait PAS

❌ **Logique métier** : aucune logique applicative (c'est le rôle des microservices)
❌ **Stockage de données** : pas de base de données
❌ **Transformation de données** : pas de modification du body des requêtes/réponses
❌ **Génération de tokens** : c'est le rôle du service Auth

---

## Architecture technique

### Pattern : API Gateway + Microservices

```
┌─────────────────────────────────────────────────────────────┐
│                         Internet                             │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           │ HTTPS
                           ▼
                 ┌──────────────────────┐
                 │   Load Balancer      │  (Optionnel en prod)
                 │   (nginx/Traefik)    │
                 └──────────┬───────────┘
                            │
                            │ HTTP
                            ▼
         ╔══════════════════════════════════════╗
         ║       API GATEWAY (Port 8080)        ║
         ║  ┌────────────────────────────────┐  ║
         ║  │  1. Validation JWT (JWKS)      │  ║
         ║  │  2. Extraction user claims     │  ║
         ║  │  3. Injection headers X-User-* │  ║
         ║  │  4. Routing vers services      │  ║
         ║  └────────────────────────────────┘  ║
         ╚══════════════════════════════════════╝
                   │        │         │
        ┌──────────┘        │         └──────────┐
        │                   │                    │
        ▼                   ▼                    ▼
   ┌─────────┐       ┌──────────┐        ┌───────────┐
   │  Auth   │       │   User   │        │ Messaging │
   │ Service │       │ Service  │        │  Service  │
   │  :8081  │       │  :8083   │        │   :8082   │
   └─────────┘       └──────────┘        └───────────┘
        │                  │                    │
        └──────────────────┴────────────────────┘
                           │
                           ▼
                  ┌─────────────────┐
                  │   PostgreSQL    │
                  │     Redis       │
                  │      NATS       │
                  └─────────────────┘
```

### Caractéristiques

| Aspect | Détail |
|--------|--------|
| **Type** | Reverse Proxy / API Gateway |
| **Langage** | Go 1.22+ |
| **Architecture** | Stateless (aucun stockage local) |
| **Scalabilité** | Horizontal (plusieurs instances derrière load balancer) |
| **Authentification** | JWT RS256 avec JWKS |
| **Protocole** | HTTP/1.1 |

---

## Flux de requêtes

### Scénario 1 : Route publique (health check)

```
1. Client → GET /health

2. API Gateway
   ├─ Pas de middleware auth (route publique)
   └─ Exécute healthHandler()

3. Réponse ← 200 OK
   {
     "status": "healthy",
     "service": "api-gateway",
     "timestamp": "2026-03-17T12:00:00Z"
   }
```

### Scénario 2 : Route protégée (succès)

```
1. Client → GET /api/v1/user/profile
   Header: Authorization: Bearer eyJhbGc...

2. API Gateway - Middleware Auth
   ├─ Extrait le token du header Authorization
   ├─ Récupère les clés publiques depuis JWKS
   │  (https://auth.retich.com/.well-known/jwks.json)
   ├─ Valide la signature RS256
   ├─ Vérifie l'expiration (exp claim)
   ├─ Vérifie l'issuer (iss claim)
   ├─ Extrait les claims : sub, email, role
   └─ ✅ Token valide

3. API Gateway - Injection headers
   ├─ Ajoute X-User-ID: user-123
   ├─ Ajoute X-User-Email: alice@retich.com
   └─ Ajoute X-User-Role: admin

4. API Gateway - Proxy
   └─ Forward vers http://user:8083/api/v1/user/profile
      avec tous les headers (incluant X-User-*)

5. Service User
   ├─ Lit X-User-ID depuis les headers
   ├─ Récupère le profil de user-123 en DB
   └─ Retourne { "id": "user-123", "name": "Alice", ... }

6. API Gateway
   └─ Retourne la réponse telle quelle au client

7. Client ← 200 OK
   { "id": "user-123", "name": "Alice", ... }
```

### Scénario 3 : Route protégée (token invalide)

```
1. Client → GET /api/v1/user/profile
   Header: Authorization: Bearer invalid-token

2. API Gateway - Middleware Auth
   ├─ Extrait le token
   ├─ Tente de valider avec JWKS
   └─ ❌ Signature invalide / Token expiré

3. Client ← 401 Unauthorized
   "Invalid or expired token"
```

### Scénario 4 : Route protégée (pas de token)

```
1. Client → GET /api/v1/user/profile
   (Pas de header Authorization)

2. API Gateway - Middleware Auth
   └─ ❌ Header Authorization manquant

3. Client ← 401 Unauthorized
   "Authorization header required"
```

### Scénario 5 : Proxy vers Auth service

```
1. Client → POST /api/v1/auth/login
   Body: { "email": "alice@retich.com", "password": "..." }

2. API Gateway
   ├─ Pas de middleware auth (route publique)
   └─ Proxy directement vers https://auth.retich.com/api/v1/auth/login

3. Service Auth
   ├─ Vérifie email/password
   └─ Génère access_token + refresh_token

4. API Gateway
   └─ Retourne la réponse du service Auth

5. Client ← 200 OK
   {
     "access_token": "eyJhbGc...",
     "refresh_token": "eyJhbGc...",
     "expires_in": 900
   }
```

---

## Composants détaillés

### 1. `cmd/server/main.go` - Point d'entrée

**Rôle** : Initialiser et démarrer le serveur HTTP

**Ce qu'il fait** :
1. Charge le fichier `.env` (si présent)
2. Charge la configuration (`config.Load()`)
3. Initialise le middleware JWKS
4. Crée les proxies pour chaque service
5. Configure les routes avec Gorilla Mux
6. Démarre le serveur HTTP
7. Gère le graceful shutdown (SIGINT/SIGTERM)

**Code clé** :
```go
// Initialisation JWKS
authMiddleware, err := middleware.NewAuthMiddlewareJWKS(cfg.JWKSURL, cfg.JWTIssuer)
if err != nil {
    log.Fatalf("Failed to initialize JWKS: %v", err)
}
defer authMiddleware.Close() // Arrête le refresh automatique

// Routes protégées
userRouter := r.PathPrefix("/api/v1/user").Subrouter()
userRouter.Use(authMiddleware.ValidateJWT) // Middleware appliqué à toutes les sous-routes
userRouter.PathPrefix("/").HandlerFunc(userProxy.ProxyRequest)
```

**Pourquoi Go ?**
- Performance (10x plus rapide que Node.js pour un proxy)
- Concurrence native (goroutines)
- Binaire statique (pas de dépendances runtime)
- Faible empreinte mémoire

---

### 2. `internal/config/config.go` - Configuration

**Rôle** : Charger et valider les variables d'environnement

**Variables obligatoires** :
- `JWKS_URL` : URL du endpoint JWKS du service Auth

**Variables optionnelles** :
- `PORT` : Port du serveur (défaut : 8080)
- `JWT_ISSUER` : Issuer attendu dans les tokens (optionnel mais recommandé)
- `AUTH_SERVICE_URL` : URL du service Auth
- etc.

**Validation** :
```go
func Load() *Config {
    jwksURL := os.Getenv("JWKS_URL")
    if jwksURL == "" {
        log.Fatal("JWKS_URL environment variable is required")
    }
    // ...
}
```

**Pourquoi cette approche ?**
- **Fail-fast** : Le serveur refuse de démarrer si la config est invalide
- **12-factor app** : Configuration via variables d'environnement
- **Pas de secrets en code** : Tout est externalisé

---

### 3. `internal/middleware/auth_jwks.go` - Validation JWT

**Rôle** : Valider les tokens JWT avec RS256 et JWKS

**Comment ça fonctionne** :

#### Étape 1 : Initialisation (au démarrage)
```go
jwks, err := keyfunc.Get(jwksURL, options)
```
- Télécharge les clés publiques depuis `https://auth.retich.com/.well-known/jwks.json`
- Les met en cache
- Démarre un refresh automatique toutes les heures

#### Étape 2 : Validation (à chaque requête)
```go
token, err := jwt.ParseWithClaims(tokenString, &claims, am.jwks.Keyfunc)
```
1. Parse le token JWT
2. Récupère le `kid` (Key ID) du header JWT
3. Trouve la clé publique correspondante dans le JWKS
4. Vérifie la signature RS256
5. Vérifie l'expiration (`exp`)
6. Vérifie l'issuer (`iss`)

#### Étape 3 : Extraction des claims
```go
userID := getStringClaim(claims, "sub")     // Identifiant utilisateur
email := getStringClaim(claims, "email")     // Email
role := getStringClaim(claims, "role")       // Rôle (optionnel)
```

#### Étape 4 : Injection dans le contexte
```go
ctx = context.WithValue(ctx, UserIDKey, userID)
r.Header.Set("X-User-ID", userID)
```

**Pourquoi JWKS ?**
- **Pas de secret partagé** : Plus sécurisé que HS256
- **Rotation de clés** : Le service Auth peut changer ses clés sans redéployer l'API Gateway
- **Standard OIDC** : Compatible avec tous les providers OAuth

**RS256 vs HS256** :

| Aspect | RS256 (utilisé ici) | HS256 (non utilisé) |
|--------|---------------------|---------------------|
| Type | Clé publique/privée | Secret partagé |
| Validation | Avec clé publique (JWKS) | Avec secret partagé |
| Sécurité | ✅ Haute (secret jamais partagé) | ⚠️ Moyenne (secret doit être partagé) |
| Rotation | ✅ Facile | ❌ Difficile |

---

### 4. `internal/proxy/auth.go` - Proxy Auth

**Rôle** : Rediriger les requêtes `/api/v1/auth/*` vers le service Auth

**Routes concernées** :
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`

**Fonctionnement** :
```go
func (ap *AuthProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
    // 1. Construire l'URL cible
    targetURL := authServiceURL + r.URL.Path + r.URL.RawQuery

    // 2. Créer une nouvelle requête HTTP
    proxyReq, _ := http.NewRequest(r.Method, targetURL, r.Body)

    // 3. Copier tous les headers
    for name, values := range r.Header {
        proxyReq.Header[name] = values
    }

    // 4. Ajouter X-Forwarded-For
    proxyReq.Header.Set("X-Forwarded-For", r.RemoteAddr)

    // 5. Envoyer la requête
    resp, _ := client.Do(proxyReq)

    // 6. Copier la réponse vers le client
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)
}
```

**Pourquoi un proxy dédié pour Auth ?**
- Les routes auth sont **publiques** (pas de JWT requis)
- Permet de gérer différemment les timeouts/retry
- Facilite l'ajout de logique spécifique (rate limiting, etc.)

---

### 5. `internal/proxy/service.go` - Proxy générique

**Rôle** : Rediriger les requêtes vers les autres microservices

**Différence avec `auth.go`** :
- Utilisé pour les routes **protégées** (après validation JWT)
- Les headers `X-User-*` sont déjà injectés par le middleware

**Services concernés** :
- User Service (`/api/v1/user/*`)
- Messaging Service (`/api/v1/messaging/*`)
- (Futurs services)

**Exemple de requête complète** :

```
# Requête originale du client
GET /api/v1/user/profile
Authorization: Bearer eyJhbGc...

# Après middleware, requête envoyée au service User
GET http://user:8083/api/v1/user/profile
Authorization: Bearer eyJhbGc...
X-User-ID: user-123
X-User-Email: alice@retich.com
X-User-Role: admin
X-Forwarded-For: 192.168.1.10
```

Le service User peut alors **faire confiance** aux headers `X-User-*` car ils proviennent forcément de l'API Gateway (les services ne sont pas exposés publiquement).

---

## Sécurité

### 1. Validation JWT robuste

✅ **Signature vérifiée** : Avec la clé publique RS256
✅ **Expiration vérifiée** : Refuse les tokens expirés
✅ **Issuer vérifié** : S'assure que le token vient du bon Auth service
✅ **Algorithm whitelist** : Seulement RS256 accepté (protection contre alg:none)

### 2. Headers injectés sécurisés

Les microservices peuvent **faire confiance** aux headers `X-User-*` car :
- ✅ Les microservices ne sont **pas exposés publiquement**
- ✅ Seul l'API Gateway peut les atteindre (réseau Docker interne)
- ✅ L'API Gateway a déjà validé le JWT avant d'injecter ces headers

### 3. Pas de secrets en code

- ❌ Pas de `JWT_SECRET` en dur
- ✅ Chargement depuis `.env`
- ✅ `.env` dans `.gitignore`

### 4. Logging sécurisé

```go
authHeader = "Bearer [REDACTED]"
log.Printf("[%s] %s (Auth: %s)", r.Method, r.URL.Path, authHeader)
```
Les tokens ne sont **jamais loggés** en clair.

### 5. Isolation réseau

```yaml
# docker-compose.yml
services:
  api-gateway:
    ports:
      - "8080:8080"  # Exposé publiquement

  user:
    # Pas de ports exposés ! Accessible uniquement via le réseau Docker interne
```

---

## Performance et scalabilité

### 1. Stateless

L'API Gateway ne stocke **aucun état** :
- Pas de session
- Pas de cache local
- Pas de base de données

**Avantage** : On peut lancer autant d'instances qu'on veut derrière un load balancer.

### 2. JWKS en cache

Les clés publiques sont :
- Téléchargées **une seule fois** au démarrage
- Rafraîchies automatiquement toutes les heures
- Pas d'appel réseau à chaque validation de token

### 3. Goroutines

Go gère des **milliers de requêtes concurrentes** avec des goroutines (threads légers).

### 4. Timeouts configurés

```go
srv := &http.Server{
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    IdleTimeout:  60 * time.Second,
}
```
Empêche les connexions de rester ouvertes indéfiniment.

---

## Résumé

L'API Gateway ReTiCh est un **reverse proxy intelligent** qui :

1. ✅ **Centralise** l'authentification (validation JWT avec JWKS)
2. ✅ **Protège** les microservices (pas d'exposition publique)
3. ✅ **Simplifie** les clients (un seul point d'entrée)
4. ✅ **Enrichit** les requêtes (injection headers `X-User-*`)
5. ✅ **Scale** horizontalement (stateless)
6. ✅ **Sécurise** les logs (masquage des tokens)

**En production**, on aura :
```
Internet → Load Balancer → [API Gateway 1, API Gateway 2, API Gateway 3] → Microservices
```

Chaque instance d'API Gateway est **identique** et **indépendante**.
