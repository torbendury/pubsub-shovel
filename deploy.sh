#!/bin/bash

# Deployment script for Google Cloud Functions
# Usage: ./deploy.sh [FUNCTION_NAME] [PROJECT_ID]

FUNCTION_NAME=${1:-"pubsub-shovel"}
PROJECT_ID=${2:-$GOOGLE_CLOUD_PROJECT}

if [ -z "$PROJECT_ID" ]; then
    echo "Error: PROJECT_ID not set. Please provide it as second argument or set GOOGLE_CLOUD_PROJECT environment variable."
    exit 1
fi

echo "Deploying function '$FUNCTION_NAME' to project '$PROJECT_ID'..."

gcloud functions deploy $FUNCTION_NAME \
    --runtime go124 \
    --trigger-http \
    --entry-point Handler \
    --allow-unauthenticated \
    --project $PROJECT_ID \
    --memory 256MB \
    --timeout 3600s \
    --max-instances 10 \
    --region europe-west1 \
    --gen2 \
    --source .

echo "Deployment completed!"
echo "Function URL:"
gcloud functions describe $FUNCTION_NAME --project $PROJECT_ID --format="value(httpsTrigger.url)" --region europe-west1
