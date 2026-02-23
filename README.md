# ReTiCh API Gateway

Point d'entrée principal de la plateforme ReTiCh. Gère le routing, l'authentification et le load balancing vers les microservices.

## Fonctionnalités

- Routing vers les microservices (Auth, User, Messaging)
- Validation des tokens JWT
- Rate limiting
- Health checks
- Métriques Prometheus

## Prérequis

- Go 1.22+
- Docker (optionnel)

## Démarrage rapide

### Avec Docker (recommandé)

```bash
# Depuis le repo ReTiCh-Infrastucture
make up
```

### Sans Docker

```bash
# Installer les dépendances
go mod download

# Lancer le serveur
go run cmd/server/main.go

# Ou compiler et exécuter
go build -o bin/server cmd/server/main.go
./bin/server
```

### Développement avec hot-reload

```bash
# Installer Air
go install github.com/air-verse/air@latest

# Lancer avec hot-reload
air -c .air.toml
```

## Configuration

Variables d'environnement:

| Variable | Description | Défaut |
|----------|-------------|--------|
| `PORT` | Port du serveur | `8080` |
| `AUTH_SERVICE_URL` | URL du service Auth | `http://auth:8081` |
| `USER_SERVICE_URL` | URL du service User | `http://user:8083` |
| `MESSAGING_SERVICE_URL` | URL du service Messaging | `http://messaging:8082` |
| `NATS_URL` | URL du serveur NATS | `nats://nats:4222` |
| `REDIS_URL` | URL Redis | `redis:6379` |
| `LOG_LEVEL` | Niveau de log | `info` |

## Endpoints

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |

## Structure du projet

```
ReTiCh-API-Gateway/
├── cmd/
│   └── server/
│       └── main.go         # Point d'entrée
├── internal/               # Code interne
├── migrations/             # Migrations DB (si nécessaire)
├── Dockerfile              # Image production
├── Dockerfile.dev          # Image développement
├── .air.toml               # Config hot-reload
├── go.mod
└── go.sum
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
docker run -p 8080:8080 retich-api-gateway
```

## Tests

```bash
# Lancer les tests
go test ./...

# Avec couverture
go test -cover ./...
```

## CI/CD

Le workflow GitHub Actions build et push automatiquement vers `ghcr.io/retich-corp/api-gateway` sur chaque push sur `main`.

## Licence

MIT
