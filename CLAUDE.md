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
svc-web-analyzer/
├── assets/                           # Project assets and branding
├── build/                            # Build system and configuration
│   ├── mk/                           # Make-based build system (Makefile, utils, config)
│   └── oapi/                         # OpenAPI code generation config
├── cmd/                              # Application entry points
│   └── svc-web-analyzer/             # Main application entry point
├── deployments/                      # Deployment configurations
│   └── docker/                       # Docker deployment setup (Dockerfile, services config)
├── docs/                             # Documentation and specifications
│   └── openapi-spec/                 # OpenAPI 3.0.3 specification (schemas, examples, public docs)
├── internal/                         # Private application packages
│   ├── adapters/                     # Infrastructure adapters (middleware, repositories, services)
│   ├── config/                       # Configuration management
│   ├── domain/                       # Domain models and business logic
│   ├── handlers/                     # HTTP handlers implementation
│   ├── infrastructure/               # Infrastructure implementations (cache, logger, metrics, etc.)
│   ├── ports/                        # Interface definitions (clean architecture)
│   ├── runtime/                      # Application bootstrap and dependency injection
│   ├── service/                      # Application service layer
│   ├── shared/                       # Shared cross-cutting concerns (decorators)
│   ├── tools/                        # Code generation tools
│   └── usecases/                     # Application use cases (CQRS commands/queries)
├── migrations/                       # Database migration files
├── scripts/                          # Build and utility scripts
├── compose.yaml                      # Docker Compose multi-service configuration
├── go.mod                            # Go module definition and dependencies
└── go.sum                            # Go module checksums for dependency verification
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

- **Tool**: Uses `build/oapi/codegen.yaml` configuration
- **Generated Code**: `internal/handlers/http_server_gen.go`
- **Build Integration**: Makefile targets for API generation
- **Docker Integration**: Uses Redocly CLI for bundling specifications

### Key Features

- **Clean Architecture**: Ports and adapters pattern with clear separation of concerns
- **Service Layer**: Application services implementing business logic and orchestration
- **Infrastructure Layer**: Complete implementation with cache, database, secrets, logging, metrics, queue, storage, and tracing
- **Repository Pattern**: Concrete implementations for PostgreSQL, Redis cache, and Vault secrets
- **Decorator Pattern**: Cross-cutting concerns implemented using decorators for logging, metrics, tracing, and CQRS commands/queries
- **Comprehensive Testing**: Unit tests for all adapters and services with parallel execution
- **HTML Analysis**: Complete HTML parsing and analysis with link checking
- **Web Fetching**: Robust web page fetching with configurable timeouts and headers
- **Comprehensive Error Handling**: Structured error responses with examples
- **Real-time Updates**: Server-sent events for analysis progress
- **Security Headers**: Complete set of security headers implemented
- **API Versioning**: Multiple versioning strategies supported
- **Frontend Application**: Modern Vue.js application with TypeScript and Tailwind CSS
- **Docker Deployment**: Complete containerization setup with Traefik
- **SSL/TLS**: Local development SSL certificate generation with mkcert
- **Database Migrations**: Automated database schema management
- **Configuration Management**: Environment-based configuration with Vault integration

### Module Information

- **Module**: `github.com/architeacher/svc-web-analyzer`
- **Go Version**: 1.25 with toolchain go1.25.1
- **Generated Code**: HTTP server interfaces and types from OpenAPI spec

### Development Environment

- **Local Domains**: Uses `*.web-analyzer.dev` with SSL certificates
  - **API**: https://api.web-analyzer.dev/v1/
  - **Documentation**: https://docs.web-analyzer.dev
  - **Traefik Dashboard**: https://traefik.web-analyzer.dev (admin/admin)
  - **Vault**: https://vault.web-analyzer.dev
  - **RabbitMQ**: https://rabbitmq.web-analyzer.dev
- **Reverse Proxy**: Traefik configuration for local development
- **API Documentation**: Auto-generated from OpenAPI specification
- **Container Orchestration**: Docker Compose setup for all services
- **Frontend**: Vue.js application with TypeScript, Tailwind CSS, and Vite
- **Setup**: Run `make init start` to initialize and start all services

### Architecture Patterns

- **Clean Architecture**: Ports and adapters pattern with clear separation of concerns
- **CQRS (Command Query Responsibility Segregation)**: Separate command and query handlers
- **Decorator Pattern**: Cross-cutting concerns for logging, metrics, tracing
- **Dependency Injection**: Runtime dependency management and configuration
- **Repository Pattern**: Data access abstraction with multiple implementations
- **Middleware Chain**: HTTP request processing pipeline with security, validation, and tracing
