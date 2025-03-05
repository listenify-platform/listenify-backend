# Listenify Backend

Backend service for the Listenify music streaming platform, written in Go.

## Description

Listenify Backend is a Go-based API that powers the Listenify music streaming application. It provides endpoints for user authentication, music streaming, playlist management, and other features required for a music streaming service.

## Prerequisites

- Go 1.24 or higher
- MongoDB (for user and music metadata)
- Redis (optional, for caching)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/listenify-platform/listenify-backend.git
cd listenify-backend
```

2. Install dependencies:
```bash
go mod download
```

## Configuration

Configuration files are located in the `/configs` directory:

- `app.yaml`: Main configuration file (database credentials, server settings)
- `secrets.yaml`: Sensitive information (API keys, tokens) - not committed to git

## Building the Application

### Development Build
```bash
go build -o listenify-server ./cmd/server
```

### Production Build
```bash
go build -tags=production -ldflags="-s -w" -o listenify-server ./cmd/server
```

## Running the Application

```bash
./listenify-server
```

## Testing

Run tests:
```bash
go test ./...
```
