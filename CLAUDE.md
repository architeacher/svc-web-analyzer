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
├── internal/                         # Private application packages
│   ├── adapters/                     # Infrastructure adapters (repositories)
│   │   ├── cache_repository.go       # Redis cache implementation
│   │   ├── health_checker.go         # Health check adapter
│   │   ├── html_analyzer.go          # HTML analysis implementation
│   │   ├── html_analyzer_test.go     # HTML analyzer tests
│   │   ├── link_checker.go           # Link validation implementation
│   │   ├── link_checker_test.go      # Link checker tests
│   │   ├── postgres_repository.go    # PostgreSQL database implementation
│   │   ├── vault_repository.go       # HashiCorp Vault secrets implementation
│   │   ├── web_fetcher.go            # Web page fetching implementation
│   │   └── web_fetcher_test.go       # Web fetcher tests
│   ├── config/                       # Configuration management
│   │   ├── loader.go                 # Configuration loader with Vault integration
│   │   ├── loader_test.go            # Configuration tests
│   │   └── settings.go               # Configuration structures
│   ├── domain/                       # Domain models and business logic
│   │   ├── analysis.go               # Web page analysis domain models
│   │   ├── errors.go                 # Domain-specific error types
│   │   └── types.go                  # Common domain types
│   ├── handlers/                     # HTTP handlers implementation
│   │   └── http_server_gen.go        # Generated HTTP server code
│   ├── service/                      # Application service layer
│   │   ├── application_service.go    # Business logic and orchestration
│   │   └── application_service_test.go # Comprehensive service tests
│   ├── infrastructure/               # Infrastructure implementations
│   │   ├── cache.go                  # Cache infrastructure setup
│   │   ├── logger.go                 # Logging infrastructure
│   │   ├── metrics.go                # Metrics collection setup
│   │   ├── queue.go                  # Message queue infrastructure
│   │   ├── storage.go                # Storage infrastructure
│   │   └── tracing.go                # OpenTelemetry tracing setup
│   ├── ports/                        # Interface definitions (clean architecture)
│   │   ├── cache_repository.go       # Cache repository interface
│   │   ├── health_checker.go         # Health check interface
│   │   ├── link_checker.go           # Link validation interface
│   │   ├── repository.go             # Data repository interface
│   │   ├── request_handler.go        # Request handling interface
│   │   ├── secrets_repository.go     # Secrets management interface
│   │   └── web_page_fetcher.go       # Web page fetching interface
│   ├── shared/                       # Shared cross-cutting concerns
│   │   └── decorator/                # Decorator pattern implementations
│   │       ├── command.go            # Command decorator for CQRS commands
│   │       ├── logging.go            # Logging decorator for cross-cutting logging
│   │       ├── metrics.go            # Metrics decorator for instrumentation
│   │       ├── query.go              # Query decorator for CQRS queries
│   │       └── tracing.go            # Tracing decorator for observability
│   └── tools/                        # Code generation tools
│       ├── generate.go               # Go generate entry point
│       ├── go.mod                    # Tools module definition
│       └── go.sum                    # Tools module checksums
├── docs/                             # Documentation and specifications
│   ├── openapi-spec/                 # Complete OpenAPI 3.0.3 specification
│   │   ├── svc-web-analyzer-api.yaml # Main API specification
│   │   ├── schemas/                  # Schema definitions
│   │   │   ├── common/               # Common schema components
│   │   │   ├── errors/               # Error response schemas
│   │   │   └── examples/             # Request/response examples
│   │   └── public/                   # Generated API documentation
│   ├── architecture-decisions.md     # Architecture Decision Records
│   └── features.md                   # Features documentation
├── deployments/docker/               # Docker deployment configuration
│   ├── Dockerfile                    # Main application Dockerfile
│   ├── svc-web-analyzer/             # Service-specific configuration
│   ├── traefik/                      # Traefik reverse proxy configuration
│   └── vault/                        # HashiCorp Vault initialization
├── migrations/                       # Database migration files
├── build/                            # Build system and tools
│   ├── mk/                           # Make build system
│   └── oapi/                         # OpenAPI code generation config
├── assets/                           # Project assets
├── scripts/                          # Build and utility scripts
├── .trees/frontend-feature/          # Frontend branch tree worktree
│   └── web/                          # Vue.js frontend application
│       ├── src/                      # Vue.js source code
│       ├── e2e/                      # End-to-end tests
│       ├── public/                   # Static assets
│       └── dist/                     # Built frontend assets
└── go.mod                            # Go module definition
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
  - **Frontend**: https://web-analyzer.dev (Vue.js application)
  - **Traefik Dashboard**: https://traefik.web-analyzer.dev (admin/admin)
- **Reverse Proxy**: Traefik configuration for local development
- **API Documentation**: Auto-generated from OpenAPI specification
- **Container Orchestration**: Docker Compose setup for all services
- **Frontend**: Vue.js application with TypeScript, Tailwind CSS, and Vite
- **Setup**: Run `make init start` to initialize and start all services

### Frontend Architecture

The project includes a comprehensive Vue.js frontend application:

- **Framework**: Vue 3 with Composition API and TypeScript
- **Build Tool**: Vite for fast development and optimized builds
- **Styling**: Tailwind CSS for utility-first styling
- **State Management**: Pinia for reactive state management
- **Testing**: Vitest for unit tests and Playwright for E2E tests
- **Components**: Modular component architecture with analysis, auth, and layout components
- **API Integration**: Axios-based API client with type-safe interfaces
- **Development**: Hot reload, TypeScript checking, and linting
- **Deployment**: Docker containerization with Nginx
