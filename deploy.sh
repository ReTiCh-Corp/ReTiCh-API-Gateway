#!/bin/bash
set -e

# ---- Config ----
APP_NAME="retich-api-gateway"
RESOURCE_GROUP="rg-retich-v2"
ENVIRONMENT="retich-env"
REGISTRY="retichregistry"
IMAGE="${REGISTRY}.azurecr.io/${APP_NAME}:latest"
ENV_FILE=".env.prod"

# ---- Check env file ----
if [ ! -f "$ENV_FILE" ]; then
  echo "Error: $ENV_FILE not found"
  exit 1
fi

# ---- Load env vars ----
ENV_VARS=""
while IFS= read -r line || [ -n "$line" ]; do
  # Skip empty lines and comments
  [[ -z "$line" || "$line" =~ ^# ]] && continue
  ENV_VARS="$ENV_VARS $line"
done < "$ENV_FILE"

echo "==> Building and pushing image to ACR..."
az acr build \
  --registry "$REGISTRY" \
  --resource-group "$RESOURCE_GROUP" \
  --image "${APP_NAME}:latest" \
  --file Dockerfile .

echo "==> Updating container app..."
az containerapp update \
  --name "$APP_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --image "$IMAGE" \
  --set-env-vars $ENV_VARS

echo "==> Deployment complete!"
echo "URL: $(az containerapp show --name "$APP_NAME" --resource-group "$RESOURCE_GROUP" --query 'properties.configuration.ingress.fqdn' -o tsv)"
