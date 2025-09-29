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
│   ├── publisher/                    # Publisher service entry point
│   ├── subscriber/                   # Subscriber service entry point
│   └── svc-web-analyzer/             # Main HTTP API service entry point
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
│   │   ├── dispatcher.go             # HTTP server runtime dispatcher
│   │   ├── publisher.go              # Publisher service runtime
│   │   └── subscriber.go             # Subscriber service runtime
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

#### Core Architecture
- **Event-Driven Architecture**: Publisher/subscriber pattern with outbox implementation for reliable message processing
- **Clean Architecture**: Ports and adapters pattern with clear separation of concerns
- **CQRS Pattern**: Command Query Responsibility Segregation with decorator pattern for cross-cutting concerns
- **Outbox Pattern**: Transactional outbox for guaranteed event publishing and delivery
- **Three-Service Design**: HTTP API, Publisher, and Subscriber services for scalable processing

#### Backend Implementation
- **Service Layer**: Application services implementing business logic and orchestration
- **Repository Pattern**: Concrete implementations for PostgreSQL, KeyDB cache, and Vault secrets
- **Infrastructure Layer**: Complete implementation with cache, database, secrets, logging, metrics, queue, storage, and tracing
- **Message Queue Integration**: RabbitMQ-based asynchronous processing with reliable delivery
- **Dependency Injection**: Runtime-based dependency management with configuration

#### API & Communication
- **RESTful API**: OpenAPI 3.0.3 specification with code generation
- **Real-time Updates**: Server-Sent Events (SSE) for analysis progress tracking
- **PASETO Authentication**: Secure token-based authentication with v4 public tokens
- **API Versioning**: Multiple versioning strategies (URL path, headers, content type)
- **Security Headers**: Complete set of HTTP security headers

#### Web Analysis Features
- **HTML Analysis**: Complete HTML parsing with version detection, heading counts, and form analysis
- **Link Analysis**: Internal/external link identification with accessibility checking
- **Web Fetching**: Robust web page fetching with configurable timeouts and custom headers
- **Error Handling**: Comprehensive error handling with structured error responses

#### Development & Operations
- **Comprehensive Testing**: Unit tests for all adapters and services with parallel execution (testify framework)
- **Database Migrations**: Automated PostgreSQL schema management with migration versioning
- **Docker Deployment**: Complete containerization with multi-stage builds and Traefik reverse proxy
- **SSL/TLS**: Local development SSL certificate generation with mkcert for `*.web-analyzer.dev` domains
- **Configuration Management**: Environment-based configuration with Vault integration for secrets
- **Observability**: Structured logging (zerolog), distributed tracing (OpenTelemetry), and metrics collection

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

- **Event-Driven Architecture**: Asynchronous message-based communication between services
- **Outbox Pattern**: Transactional outbox for reliable event publishing and delivery
- **Publisher/Subscriber**: Decoupled services communicating through message queues
- **Clean Architecture**: Ports and adapters pattern with clear separation of concerns
- **CQRS (Command Query Responsibility Segregation)**: Separate command and query handlers
- **Decorator Pattern**: Cross-cutting concerns for logging, metrics, tracing
- **Dependency Injection**: Runtime dependency management and configuration
- **Repository Pattern**: Data access abstraction with multiple implementations
- **Middleware Chain**: HTTP request processing pipeline with security, validation, and tracing

### Service Architecture

The application consists of three main services:

#### 1. HTTP API Service (`cmd/svc-web-analyzer/`)
- **Purpose**: RESTful API endpoints for web page analysis requests
- **Port**: 8080 (https://api.web-analyzer.dev/v1/)
- **Runtime**: `internal/runtime/dispatcher.go`
- **Responsibilities**:
  - Handle HTTP requests for analysis submission
  - Authenticate requests using PASETO tokens
  - Store analysis requests in PostgreSQL
  - Publish events to outbox table for processing
  - Provide real-time updates via Server-Sent Events (SSE)

#### 2. Publisher Service (`cmd/publisher/`)
- **Purpose**: Event publishing and outbox pattern implementation
- **Runtime**: `internal/runtime/publisher.go`
- **Responsibilities**:
  - Monitor outbox events table for new analysis requests
  - Publish events to RabbitMQ message queue
  - Ensure reliable event delivery with transactional outbox pattern
  - Handle event retries and error scenarios
  - Mark events as published after successful delivery

#### 3. Subscriber Service (`cmd/subscriber/`)
- **Purpose**: Asynchronous web page analysis processing
- **Runtime**: `internal/runtime/subscriber.go`
- **Responsibilities**:
  - Consume analysis events from RabbitMQ
  - Perform actual web page fetching and analysis
  - Update analysis status and results in PostgreSQL
  - Emit progress events for real-time updates
  - Handle processing errors and retries

### Event Flow

```
HTTP API → PostgreSQL → Publisher → RabbitMQ → Subscriber → PostgreSQL
    ↓        (outbox)                                        ↓
Analysis                                               Analysis
Request                                               Processing
    ↓                                                       ↓
SSE Updates ←─────────────────────────────────── Status Updates
```

**Benefits:**
- **Scalability**: Services can be scaled independently based on load
- **Reliability**: Transactional outbox ensures no message loss
- **Separation of Concerns**: Clear boundaries between request handling and processing
- **Resilience**: Asynchronous processing with retry capabilities
- **Performance**: Non-blocking request handling with background processing
