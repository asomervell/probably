# Deployment Setup Guide

This guide explains how to set up CI/CD for deploying to Google Cloud Run.

## Prerequisites

1. A Google Cloud Project with the following APIs enabled:
   - Cloud Run API
   - Artifact Registry API
   - Cloud Build API (optional, for build logs)

2. An Artifact Registry repository created:
   ```bash
   gcloud artifacts repositories create probably \
     --repository-format=docker \
     --location=us-central1 \
     --description="Docker repository for Probably"
   ```

3. A Cloud Run service named `money-probably` (or update the workflow to match your service name)

## Required GitHub Secrets

Add the following secrets to your GitHub repository (Settings → Secrets and variables → Actions):

1. **GCP_PROJECT_ID**: Your Google Cloud Project ID
   - Example: `my-project-123456`

2. **GCP_SA_KEY**: Service Account JSON key with the following permissions:
   - Cloud Run Admin (`roles/run.admin`)
   - Artifact Registry Writer (`roles/artifactregistry.writer`)
   - Service Account User (`roles/iam.serviceAccountUser`)

### Creating the Service Account

```bash
# Create service account
gcloud iam service-accounts create github-actions \
  --display-name="GitHub Actions Service Account"

# Grant necessary permissions
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:github-actions@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:github-actions@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.writer"

gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:github-actions@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"

# Create and download key
gcloud iam service-accounts keys create key.json \
  --iam-account=github-actions@PROJECT_ID.iam.gserviceaccount.com
```

Copy the contents of `key.json` and add it as the `GCP_SA_KEY` secret in GitHub.

## Environment Variables

The Cloud Run service will need environment variables configured. You can set these in the Cloud Run console or via the workflow. Common variables include:

- `DATABASE_URL`: PostgreSQL connection string
- `SESSION_SECRET`: Secret key for session encryption (32+ characters)
- `COOKIE_DOMAIN`: Domain for cookies (e.g., `probably.money`)
- `BASE_URL`: Full base URL (e.g., `https://probably.money`)
- `TELLER_APP_ID`: Teller API app ID (if using Teller integration)
- `TELLER_CERT`: Teller certificate
- `TELLER_KEY`: Teller private key
- `TELLER_WEBHOOK_SECRET`: Teller webhook secret
- `GROQ_API_KEY`: Groq API key (if using AI categorization)
- `GROQ_MODEL`: Groq model name (e.g., `llama-3.3-70b-versatile`)

To set environment variables in Cloud Run:

```bash
gcloud run services update money-probably \
  --region=us-central1 \
  --set-env-vars="DATABASE_URL=postgres://...,SESSION_SECRET=..."
```

Or use the Cloud Console: Cloud Run → money-probably → Edit & Deploy New Revision → Variables & Secrets

## Workflow Configuration

The workflow is configured to:
- Trigger on pushes to `main` branch
- Build Docker image from `./Dockerfile`
- Push to Artifact Registry: `us-central1-docker.pkg.dev/PROJECT_ID/probably/probably-server`
- Deploy to Cloud Run service: `money-probably`
- Use region: `us-central1`

To customize, edit `.github/workflows/deploy.yml` and update the `env` section.

## Manual Deployment

You can also deploy manually:

```bash
# Build and push
docker build -t us-central1-docker.pkg.dev/PROJECT_ID/probably/probably-server:latest .
docker push us-central1-docker.pkg.dev/PROJECT_ID/probably/probably-server:latest

# Deploy
gcloud run deploy money-probably \
  --image us-central1-docker.pkg.dev/PROJECT_ID/probably/probably-server:latest \
  --region us-central1 \
  --platform managed
```
