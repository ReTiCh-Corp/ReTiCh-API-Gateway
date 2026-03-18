# ReTiCh API Gateway

Point d'entrée principal de la plateforme ReTiCh. Gère le routing, l'authentification et le load balancing vers les microservices.

> **État du projet** : L'API Gateway est fonctionnel et prêt pour la production. Le service Auth est déjà déployé. Les services User et Messaging seront ajoutés progressivement (les routes sont déjà configurées).

## Table des matières

- [Fonctionnalités](#fonctionnalités)
- [Architecture](#architecture)
- [Prérequis](#prérequis)
- [Configuration](#configuration)
- [Démarrage rapide](#démarrage-rapide)
- [Endpoints exposés](#endpoints-exposés)
- [Authentification JWT](#authentification-jwt)
- [Ajouter un nouveau microservice](#ajouter-un-nouveau-microservice)
- [Structure du projet](#structure-du-projet)
- [Tests](#tests)
- [Docker](#docker)

## Fonctionnalités

- **Reverse Proxy** vers les microservices (Auth, User, Messaging)
- **Validation JWT locale** (pas d'appel réseau à chaque requête)
- **Injection automatique** des headers utilisateur (`X-User-ID`, `X-User-Email`, `X-User-Role`)
- **Logging** des requêtes avec masquage des tokens
- **Health checks**
- **Graceful shutdown**
- **Hot-reload** pour le développement

## Architecture

### Flux d'authentification

```
Client → API Gateway → Validation JWT locale → Microservice
                ↓ si invalide/expiré
            Retourne 401
```

L'API Gateway utilise une architecture **stateless** :
- **Aucune base de données** : pas de stockage local
- **Validation JWT locale avec JWKS** : récupère les clés publiques RS256 pour vérifier les tokens sans interroger le service Auth à chaque requête
- **Proxy transparent** : transmet les requêtes aux microservices avec les informations utilisateur

> 📖 **Pour une documentation technique complète**, consultez [ARCHITECTURE.md](./ARCHITECTURE.md)

### Composants

| Composant | Rôle | Fichier |
|-----------|------|---------|
| **Config** | Charge les variables d'environnement | `internal/config/config.go` |
| **Middleware Auth JWKS** | Valide les JWT RS256 avec JWKS et injecte le contexte utilisateur | `internal/middleware/auth_jwks.go` |
| **Proxy Auth** | Redirige les requêtes `/auth/*` vers le service Auth | `internal/proxy/auth.go` |
| **Proxy Service** | Redirige les requêtes vers les autres microservices | `internal/proxy/service.go` |

## Prérequis

- Go 1.22+
- Docker (optionnel)
- Accès au service ReTiCh Auth (pour le JWKS endpoint)

## Configuration

### Variables d'environnement

Créer un fichier `.env` à la racine du projet :

```bash
cp .env.example .env
```

| Variable | Description | Défaut | Obligatoire |
|----------|-------------|--------|-------------|
| `PORT` | Port du serveur | `8080` | Non |
| `JWKS_URL` | URL du endpoint JWKS pour récupérer les clés publiques JWT | - | **OUI** |
| `JWT_ISSUER` | Émetteur attendu dans les tokens (claim `iss`) | (vide) | Recommandé |
| `AUTH_SERVICE_URL` | URL du service Auth | `http://auth:8081` | Non |
| `USER_SERVICE_URL` | URL du service User | `http://user:8083` | Non |
| `MESSAGING_SERVICE_URL` | URL du service Messaging | `http://messaging:8082` | Non |
| `NATS_URL` | URL du serveur NATS | `nats://nats:4222` | Non |
| `REDIS_URL` | URL Redis | `redis:6379` | Non |
| `LOG_LEVEL` | Niveau de log | `info` | Non |

**IMPORTANT :**
- `JWKS_URL` doit pointer vers l'endpoint `.well-known/jwks.json` du service Auth
- Exemple : `https://auth.retich.com/.well-known/jwks.json`
- Les clés publiques sont automatiquement rafraîchies toutes les heures

## Démarrage rapide

### 1. Configuration

```bash
# Cloner le repo
git clone https://github.com/ReTiCh-Corp/ReTiCh-API-Gateway.git
cd ReTiCh-API-Gateway

# Créer le fichier .env
cp .env.example .env

# Éditer .env et définir JWKS_URL
# JWKS_URL=https://auth.retich.com/.well-known/jwks.json
# JWT_ISSUER=https://auth.retich.com/
```

### 2. Installation des dépendances

```bash
go mod download
```

### 3. Lancement

#### Sans Docker

```bash
# Lancer le serveur
go run cmd/server/main.go

# Ou compiler puis exécuter
go build -o bin/server cmd/server/main.go
./bin/server
```

#### Avec Docker (recommandé en production)

```bash
# Depuis le repo ReTiCh-Infrastructure
make up
```

#### Développement avec hot-reload

```bash
# Installer Air
go install github.com/air-verse/air@latest

# Lancer avec hot-reload
air -c .air.toml
```

Le serveur démarre sur `http://localhost:8080`.

## Endpoints exposés

### Routes publiques (pas d'authentification)

| Méthode | Endpoint | Description | Corps de la requête |
|---------|----------|-------------|---------------------|
| `GET` | `/health` | Health check de l'API Gateway | - |
| `GET` | `/ready` | Readiness check | - |

### Routes Auth (publiques, proxy vers le service Auth)

| Méthode | Endpoint | Description | Destination |
|---------|----------|-------------|-------------|
| `POST` | `/api/v1/auth/login` | Connexion utilisateur | Auth service |
| `POST` | `/api/v1/auth/register` | Inscription utilisateur | Auth service |
| `POST` | `/api/v1/auth/refresh` | Renouveler le token | Auth service |
| `POST` | `/api/v1/auth/logout` | Déconnexion utilisateur | Auth service |

### Routes User (protégées par JWT)

| Méthode | Endpoint | Description | Destination |
|---------|----------|-------------|-------------|
| `*` | `/api/v1/user/*` | Toutes les routes utilisateur | User service |

**Exemples :**
- `GET /api/v1/user/profile`
- `PUT /api/v1/user/profile`
- `GET /api/v1/user/:id`

### Routes Messaging (protégées par JWT)

| Méthode | Endpoint | Description | Destination |
|---------|----------|-------------|-------------|
| `*` | `/api/v1/messaging/*` | Toutes les routes de messagerie | Messaging service |

**Exemples :**
- `GET /api/v1/messaging/conversations`
- `POST /api/v1/messaging/conversations/:id/messages`

## Authentification JWT

### Fonctionnement (RS256 avec JWKS)

1. **Le client se connecte** via OAuth 2.0 Authorization Code Flow (ou login direct)
2. **Il reçoit un token JWT** (access_token + refresh_token) signé avec **RS256**
3. **Il envoie ce token** dans le header `Authorization: Bearer <token>` pour chaque requête
4. **L'API Gateway valide le token** :
   - Récupère les clés publiques depuis JWKS (`.well-known/jwks.json`)
   - Vérifie la signature RS256
   - Vérifie l'expiration (`exp`)
   - Vérifie l'issuer (`iss`)
5. **Si valide**, elle injecte les informations utilisateur et proxie vers le microservice
6. **Si invalide**, elle retourne `401 Unauthorized`

**Avantage de RS256 + JWKS** :
- Pas de secret partagé (plus sécurisé)
- Rotation de clés sans redémarrage
- Standard OIDC (compatible avec NextAuth, Auth0, etc.)

### Structure du token JWT

Le token contient les claims suivants (format OIDC) :

```json
{
  "sub": "user-123",              // Subject (ID utilisateur)
  "email": "alice@retich.com",
  "email_verified": true,
  "role": "admin",                // Claim custom (optionnel)
  "iss": "https://auth.retich.com/",  // Issuer
  "aud": "54a3afb1-...",          // Audience (client_id)
  "iat": 1234567890,              // Issued at
  "exp": 1234568790               // Expiration
}
```

**Claims utilisés par l'API Gateway** :
- `sub` → Extrait dans `X-User-ID`
- `email` → Extrait dans `X-User-Email`
- `role` → Extrait dans `X-User-Role` (si présent)
- `iss` → Vérifié contre `JWT_ISSUER`

### Headers injectés automatiquement

Quand le JWT est valide, l'API Gateway ajoute ces headers avant de transmettre aux microservices :

```
X-User-ID: user-123
X-User-Email: alice@retich.com
X-User-Role: admin
```

Les microservices peuvent donc utiliser ces informations **sans valider le JWT eux-mêmes**.

### Tester l'authentification

**En production**, les tokens sont obtenus via le service Auth (OAuth 2.0 flow).

**Pour tester localement**, vous pouvez :

1. **Utiliser le playground OAuth** du service Auth :
   ```
   https://auth.retich.com/oauth/playground
   ```

2. **Ou créer une app Next.js** qui se connecte via OAuth (voir README du service Auth)

3. **Ou utiliser curl** avec un token obtenu manuellement :
   ```bash
   # Obtenir un token via le service Auth
   # (nécessite un client_id et client_secret configurés)

   # Puis tester une route protégée
   curl http://localhost:8080/api/v1/user/profile \
     -H "Authorization: Bearer <votre_token_ici>"
   ```

**Vérification du token** :
- Vous pouvez décoder votre JWT sur [jwt.io](https://jwt.io) pour voir les claims
- L'API Gateway validera la signature avec la clé publique du JWKS

## Ajouter un nouveau microservice

Vous souhaitez ajouter un nouveau microservice (par exemple **Payment**) à la plateforme ? Suivez ces étapes :

### 1. Ajouter la variable d'environnement

**Fichier `.env.example` :**
```bash
PAYMENT_SERVICE_URL=http://payment:8084
```

**Fichier `internal/config/config.go` :**
```go
type Config struct {
    // ... autres champs
    PaymentServiceURL string
}

func Load() *Config {
    return &Config{
        // ... autres champs
        PaymentServiceURL: getEnv("PAYMENT_SERVICE_URL", "http://payment:8084"),
    }
}
```

### 2. Créer le proxy dans `main.go`

**Fichier `cmd/server/main.go` :**

```go
func main() {
    cfg := config.Load()
    authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret, cfg.JWTIssuer)

    // ... proxies existants
    paymentProxy := proxy.NewServiceProxy(cfg.PaymentServiceURL)

    r := mux.NewRouter()

    // ... routes existantes

    // Routes payment (protégées par JWT)
    paymentRouter := r.PathPrefix("/api/v1/payment").Subrouter()
    paymentRouter.Use(authMiddleware.ValidateJWT)
    paymentRouter.PathPrefix("/").HandlerFunc(paymentProxy.ProxyRequest)

    // ... reste du code
}
```

### 3. Mettre à jour la documentation

Ajouter dans ce README :
- La variable `PAYMENT_SERVICE_URL` dans la section Configuration
- Les routes exposées dans la section Endpoints

### 4. Redémarrer l'API Gateway

```bash
# Relancer le serveur
go run cmd/server/main.go
```

### Exemple complet

Voici le diff pour ajouter le service **Payment** :

**`internal/config/config.go` :**
```diff
type Config struct {
    Port                string
    JWTSecret           string
    JWTIssuer           string
    AuthServiceURL      string
    UserServiceURL      string
    MessagingServiceURL string
+   PaymentServiceURL   string
    NatsURL             string
    RedisURL            string
    LogLevel            string
}

func Load() *Config {
    return &Config{
        Port:                getEnv("PORT", "8080"),
        JWTSecret:           jwtSecret,
        JWTIssuer:           getEnv("JWT_ISSUER", "retich-auth"),
        AuthServiceURL:      authServiceURL,
        UserServiceURL:      getEnv("USER_SERVICE_URL", "http://user:8083"),
        MessagingServiceURL: getEnv("MESSAGING_SERVICE_URL", "http://messaging:8082"),
+       PaymentServiceURL:   getEnv("PAYMENT_SERVICE_URL", "http://payment:8084"),
        NatsURL:             getEnv("NATS_URL", "nats://nats:4222"),
        RedisURL:            getEnv("REDIS_URL", "redis:6379"),
        LogLevel:            getEnv("LOG_LEVEL", "info"),
    }
}
```

**`cmd/server/main.go` :**
```diff
func main() {
    cfg := config.Load()
    authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret, cfg.JWTIssuer)

    authProxy := proxy.NewAuthProxy(cfg.AuthServiceURL)
    userProxy := proxy.NewServiceProxy(cfg.UserServiceURL)
    messagingProxy := proxy.NewServiceProxy(cfg.MessagingServiceURL)
+   paymentProxy := proxy.NewServiceProxy(cfg.PaymentServiceURL)
    devToolsHandler := devtools.NewDevToolsHandler(cfg.JWTSecret, cfg.JWTIssuer)

    r := mux.NewRouter()

    // Routes publiques
    r.HandleFunc("/health", healthHandler).Methods("GET")
    r.HandleFunc("/ready", readyHandler).Methods("GET")
    r.HandleFunc("/dev/generate-token", devToolsHandler.GenerateToken).Methods("POST")

    // Routes auth
    authRouter := r.PathPrefix("/api/v1/auth").Subrouter()
    authRouter.HandleFunc("/login", authProxy.HandleLogin).Methods("POST")
    authRouter.HandleFunc("/register", authProxy.HandleRegister).Methods("POST")
    authRouter.HandleFunc("/refresh", authProxy.HandleRefresh).Methods("POST")
    authRouter.HandleFunc("/logout", authProxy.HandleLogout).Methods("POST")

    // Routes user
    userRouter := r.PathPrefix("/api/v1/user").Subrouter()
    userRouter.Use(authMiddleware.ValidateJWT)
    userRouter.PathPrefix("/").HandlerFunc(userProxy.ProxyRequest)

    // Routes messaging
    messagingRouter := r.PathPrefix("/api/v1/messaging").Subrouter()
    messagingRouter.Use(authMiddleware.ValidateJWT)
    messagingRouter.PathPrefix("/").HandlerFunc(messagingProxy.ProxyRequest)

+   // Routes payment
+   paymentRouter := r.PathPrefix("/api/v1/payment").Subrouter()
+   paymentRouter.Use(authMiddleware.ValidateJWT)
+   paymentRouter.PathPrefix("/").HandlerFunc(paymentProxy.ProxyRequest)

    // ... reste du code
}
```

C'est tout ! Votre nouveau microservice est maintenant accessible via l'API Gateway.

## Structure du projet

```
ReTiCh-API-Gateway/
├── cmd/
│   └── server/
│       └── main.go                  # Point d'entrée principal du serveur
├── internal/
│   ├── config/
│   │   └── config.go                # Chargement des variables d'environnement
│   ├── middleware/
│   │   └── auth.go                  # Middleware de validation JWT
│   ├── proxy/
│   │   ├── auth.go                  # Proxy vers le service Auth
│   │   └── service.go               # Proxy générique vers les microservices
│   └── devtools/
│       └── token.go                 # Générateur de tokens JWT (dev only)
├── .air.toml                        # Configuration hot-reload
├── .env.example                     # Template des variables d'environnement
├── .env                             # Variables d'environnement (ne pas commit)
├── Dockerfile                       # Image production
├── Dockerfile.dev                   # Image développement
├── go.mod                           # Dépendances Go
├── go.sum                           # Checksums des dépendances
├── README.md                        # Documentation
└── TEST.md                          # Guide de test manuel

```

### Détail des composants

#### `cmd/server/main.go`
Point d'entrée de l'application. Gère :
- Chargement de la configuration
- Initialisation des middlewares et proxies
- Définition des routes
- Démarrage du serveur HTTP
- Graceful shutdown

#### `internal/config/config.go`
Charge les variables d'environnement et fournit une structure `Config` avec des valeurs par défaut.

#### `internal/middleware/auth.go`
Contient deux middlewares :
- **`ValidateJWT`** : Bloque les requêtes si le token est absent ou invalide (pour routes protégées)
- **`OptionalJWT`** : Extrait les infos du token s'il est présent, mais ne bloque pas (pour routes optionnellement authentifiées)

#### `internal/proxy/auth.go`
Proxy spécifique pour les routes `/api/v1/auth/*`. Transmet les requêtes au service Auth.

#### `internal/proxy/service.go`
Proxy générique pour les autres microservices. Transmet les requêtes et copie les headers (incluant `X-User-*`).

#### `internal/devtools/token.go`
Générateur de tokens JWT pour faciliter les tests en développement. **À désactiver en production**.

## Tests

### Tests manuels

Suivez le guide dans `TEST.md` pour tester manuellement avec `curl` ou Postman.

```bash
# 1. Démarrer le serveur
go run cmd/server/main.go

# 2. Générer un token
curl -X POST http://localhost:8080/dev/generate-token \
  -H "Content-Type: application/json" \
  -d '{"user_id":"test","email":"test@retich.com","role":"admin"}'

# 3. Tester avec le token
curl http://localhost:8080/api/v1/user/profile \
  -H "Authorization: Bearer <token>"
```

### Tests unitaires (à venir)

```bash
# Lancer les tests
go test ./...

# Avec couverture
go test -cover ./...

# Avec détails
go test -v ./...
```

## Docker

### Build manuel

```bash
# Production
docker build -t retich-api-gateway .

# Développement
docker build -f Dockerfile.dev -t retich-api-gateway:dev .
```

### Exécution

```bash
# Avec variables d'environnement inline
docker run -p 8080:8080 \
  -e JWT_SECRET=your-secret \
  -e AUTH_SERVICE_URL=http://auth:8081 \
  retich-api-gateway

# Ou avec un fichier .env
docker run -p 8080:8080 --env-file .env retich-api-gateway
```

## CI/CD

Le workflow GitHub Actions build et push automatiquement vers `ghcr.io/retich-corp/api-gateway` sur chaque push sur `main`.

### Déploiement

L'API Gateway est déployée via Docker Compose depuis le repo `ReTiCh-Infrastructure`.

## Sécurité

### Bonnes pratiques

1. **Ne jamais commit le `.env`** (déjà dans `.gitignore`)
2. **Utiliser un `JWT_SECRET` fort** (minimum 32 caractères aléatoires)
3. **Désactiver `/dev/generate-token` en production** (ou le protéger par IP whitelisting)
4. **Utiliser HTTPS en production** (via reverse proxy nginx/Traefik)
5. **Limiter les logs sensibles** (tokens déjà masqués)

### Variables sensibles

```bash
# Générer un secret sécurisé
openssl rand -base64 32
# Exemple : 8vZ3kL9mN2pQ5rT6wX8yA1bC4dE7fG0h
```

## Troubleshooting

### Le serveur ne démarre pas

**Erreur : `JWT_SECRET environment variable is required`**
→ Créez un fichier `.env` avec `JWT_SECRET=votre-secret`

**Erreur : `bind: address already in use`**
→ Un autre processus utilise le port 8080. Changez le `PORT` dans `.env` ou tuez le processus :
```bash
# Windows
netstat -ano | findstr :8080
taskkill //F //PID <PID>

# Linux/Mac
lsof -i :8080
kill -9 <PID>
```

### Routes 404

**Problème : Toutes les routes retournent 404**
→ Vérifiez que vous avez bien redémarré le serveur après les modifications

**Problème : `/dev/generate-token` retourne 404**
→ Vérifiez que le code dans `cmd/server/main.go` inclut bien l'endpoint

### Authentification échoue

**Erreur : `Invalid or expired token`**
→ Le token est expiré ou la signature ne peut pas être vérifiée avec les clés JWKS

**Erreur : `Invalid token issuer`**
→ Le claim `iss` du token ne correspond pas à `JWT_ISSUER` dans la config

**Erreur : `Failed to initialize JWKS`**
→ L'URL JWKS est incorrecte ou le service Auth n'est pas accessible

## Roadmap

- [ ] Rate limiting par IP/utilisateur
- [ ] Métriques Prometheus (`/metrics`)
- [ ] Support CORS configurable
- [ ] Circuit breaker pour les services downstream
- [ ] Load balancing multiple instances
- [ ] Tests unitaires complets
- [ ] Health check des services downstream

## Documentation complémentaire

- 📖 **[ARCHITECTURE.md](./ARCHITECTURE.md)** - Documentation technique ultra-détaillée
  - Flux de requêtes complets avec exemples
  - Explication de chaque composant
  - Diagrammes d'architecture
  - Comparaison RS256 vs HS256
  - Performance et scalabilité

## Licence

MIT

---

**Développé par ReTiCh Corp** | [GitHub](https://github.com/ReTiCh-Corp/ReTiCh-API-Gateway)
