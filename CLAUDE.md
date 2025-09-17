# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

This is a Go 1.25 project using Go modules. Common development commands:

- `make init` - Initialize the project (sets hosts, certify SSL, generate API)
- `make start` - Start all development services with Docker Compose
- `make destroy` - Stop and remove all development containers
- `make generate-api` - Generate API code from OpenAPI specification
- `make certify` - Generate SSL certificates for local development
- `make help` - View all available Makefile targets
- `make list` - List all targets
- `make test` - Run all tests in the project
- `go mod tidy` - Clean up module dependencies

## Project Architecture

This is a web page analyzer service with comprehensive OpenAPI specification and Docker deployment setup.

### Project Structure
```
├── internal/                     # Private application packages
│   ├── handlers/                 # HTTP handlers
│   └── tools/                    # Code generation tools
├── docs/openapi-spec/            # Complete OpenAPI 3.0.3 specification
│   ├── svc-web-analyzer-api.yaml # Main API specification
│   ├── schemas/                  # Schema definitions
│   └── public/                   # Generated API documentation
├── deployments/docker/           # Docker deployment configuration
├── build/mk/                     # Make build system
├── assets/                       # Project assets
├── scripts/                      # Build and utility scripts
├── web/                          # Web assets
└── go.mod                        # Go module definition
```

### API Specification

The project includes a comprehensive OpenAPI 3.0.3 specification:

- **API Version**: v1.0.0
- **Base Path**: `/v1/` (no `/api` prefix)
- **Authentication**: PASETO token authentication
- **Endpoints**:
  - `POST /v1/analyze` - Analyze a web page
  - `GET /v1/analysis/{analysisId}` - Get analysis results
  - `GET /v1/analysis/{analysisId}/events` - SSE endpoint for real-time updates
  - `GET /v1/health` - Health check endpoint

### Code Generation

The project uses `oapi-codegen` for generating Go code from OpenAPI specifications:

- **Tool**: Uses `oapi-codegen.yaml` configuration
- **Generated Code**: `internal/httpserver/httpserver_gen.go`
- **Build Integration**: Makefile targets for API generation
- **Docker Integration**: Uses Redocly CLI for bundling specifications

### Key Features

- **Comprehensive Error Handling**: Structured error responses with examples
- **Real-time Updates**: Server-sent events for analysis progress
- **Security Headers**: Complete set of security headers implemented
- **API Versioning**: Multiple versioning strategies supported
- **Docker Deployment**: Complete containerization setup with Traefik
- **SSL/TLS**: Local development SSL certificate generation with mkcert

### Module Information

- **Module**: `github.com/architeacher/svc-web-analyzer`
- **Go Version**: 1.25 with toolchain go1.25.1
- **Generated Code**: HTTP server interfaces and types from OpenAPI spec

### Development Environment

- **Local Domains**: Uses `*.web-analyzer.dev` with SSL certificates
  - **API**: https://api.web-analyzer.dev/v1/
  - **Documentation**: https://docs.web-analyzer.dev
  - **Traefik Dashboard**: https://traefik.web-analyzer.dev (admin/admin)
- **Reverse Proxy**: Traefik configuration for local development
- **API Documentation**: Auto-generated from OpenAPI specification
- **Container Orchestration**: Docker Compose setup for all services
- **Setup**: Run `make init start` to initialize and start all services
