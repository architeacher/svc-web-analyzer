```bash
 _       __     __       ___                __
| |     / /__  / /_     /   |  ____  ____ _/ /_  ______  ___  _____
| | /| / / _ \/ __ \   / /| | / __ \/ __ `/ / / / /_  / / _ \/ ___/
| |/ |/ /  __/ /_/ /  / ___ |/ / / / /_/ / / /_/ / / /_/  __/ /
|__/|__/\___/_.___/  /_/  |_/_/ /_/\__,_/_/\__, / /___/\___/_/
                                          /____/
```

A comprehensive web application that analyzes web pages and provides detailed insights about HTML structure, links, and forms.

## Features

- **Web Page Analysis**: HTML version detection, title extraction, heading analysis, and form detection
- **Link Analysis**: Internal/external link identification with accessibility checking
- **Real-time Updates**: Server-Sent Events for live progress tracking
- **Secure API**: [PASETO](https://paseto.io/) token authentication with comprehensive security headers
- **Multiple API Versioning**: URL path, header, and content type versioning strategies
- **Complete Backend Implementation**: Fully functional Go backend with clean architecture
- **CQS Pattern**: Command Query Separation with decorators
- **Comprehensive Middleware**: Security, rate limiting, tracing, validation, and authentication
- **Production Ready**: Full dependency injection, configuration management, and runtime dispatcher

For complete feature documentation, see [Features Documentation](docs/features.md).

## Documentation

### Architecture & Design
- **[Architecture Decisions](docs/architecture-decisions.md)**: Comprehensive ADRs documenting all major architectural choices and their rationale
- **[Features Documentation](docs/features.md)**: Detailed documentation of all implemented features, APIs, and capabilities

### API Documentation
- **[OpenAPI Specification](docs/openapi-spec/web-analyzer-api.yaml)**: Complete OpenAPI 3.0.3 specification
- **[Generated Documentation](https://docs.web-analyzer.dev)**: Interactive API documentation (available after running `make init`)

## Architecture

This project implements a **code-first API design** approach with comprehensive OpenAPI specification and generated server code.

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

## Technology Stack

### Backend
- **Language**: Go 1.25 with clean architecture implementation
- **Code Generation**: oapi-codegen for OpenAPI-to-Go conversion
- **Authentication**: PASETO tokens with enhanced security validation
- **API Specification**: OpenAPI 3.0.3 with comprehensive examples
- **Build System**: Make with modular build configuration
- **Database**: PostgreSQL with lib/pq driver and migrations
- **Cache**: KeyDB with go-redis client
- **Logging**: Structured logging with zerolog
- **Testing**: Testify framework with parallel execution and comprehensive coverage
- **Observability**: OpenTelemetry for distributed tracing and metrics
- **Secret Management**: HashiCorp Vault integration
- **Configuration**: Environment-based configuration with envconfig
- **Architecture**: Clean architecture with ports/adapters, CQS, and dependency injection
- **Middleware**: Complete middleware stack (security, auth, validation, rate limiting, tracing)
- **Runtime**: Production-ready application dispatcher with graceful shutdown

### API Design
- **Specification**: OpenAPI 3.0.3 with detailed schemas and examples
- **Versioning**: Multiple strategies (URL path `/v1/`, headers, content type)
- **Real-time**: Server-Sent Events for analysis progress
- **Security**: Complete security headers and PASETO authentication
- **Documentation**: Auto-generated from OpenAPI specification

### Infrastructure
- **Containerization**: Docker with multi-stage builds
- **Reverse Proxy**: Traefik with automatic SSL/TLS
- **Local Development**: SSL certificate generation with mkcert
- **Documentation**: Redocly CLI for API bundling and validation

## Quick Start

### Prerequisites

Before running the application, ensure you have the following installed:

- **Go 1.25+**: Required for building and running the application
- **Docker & Docker Compose**: For containerized development environment
- **mkcert**: For generating local SSL certificates
- **Make**: For build automation (usually pre-installed on macOS/Linux)

### Installation Steps

1. **Install Go 1.25+**
   ```bash
   # macOS with Homebrew
   brew install go

   # Or download from https://golang.org/dl/
   ```

2. **Install Docker Desktop**
    - Download from [https://docker.com/products/docker-desktop](https://docker.com/products/docker-desktop)
    - Ensure Docker Compose is included (it comes with Docker Desktop)

3. **Install mkcert**
   ```bash
   # macOS with Homebrew
   brew install mkcert

   # Ubuntu/Debian
   sudo apt install libnss3-tools
   curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
   chmod +x mkcert-v*-linux-amd64
   sudo cp mkcert-v*-linux-amd64 /usr/local/bin/mkcert
   ```

### Development Setup

1. **Clone and setup**
   ```bash
   git clone https://github.com/architeacher/svc-web-analyzer.git
   cd svc-web-analyzer
   make init start
   ```
   This will:
    - Copy `.envrc.dist` to `.envrc` (edit as needed)
    - Add local domains to `/etc/hosts`
    - Generate SSL certificates with mkcert
    - Download Go dependencies with `go mod vendor`
    - Generate API code from OpenAPI specification
    - Start all services with Docker Compose

2. **Verify installation**
   ```bash
   # Check health endpoint
   curl -s https://api.web-analyzer.dev/v1/health | jq

   # Should return status: "healthy"
   ```

3. **Access the application**
    - **API**: https://api.web-analyzer.dev/v1/ (API documentation)
    - **Documentation**: https://docs.web-analyzer.dev
    - **Traefik Dashboard**: https://traefik.web-analyzer.dev (admin/admin)
    - **Vault**: https://vault.web-analyzer.dev
    - **RabbitMQ**: https://rabbitmq.web-analyzer.dev

### Environment Variables

The application uses environment variables for configuration. Copy `.envrc.dist` to `.envrc` and modify as needed:

```bash
make .envrc
# Edit .envrc with your preferred editor
```

Key environment variables:
- `PORT`: Server port (default: 8080)
- `ENVIRONMENT`: Application environment (development, staging, production)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)
- `KEYDB_HOST`: KeyDB server host
- `KEYDB_PORT`: KeyDB server port

### Development Commands

```bash
# Initialize project (hosts, SSL certs, API generation)
make init

# Start development services
make start

# Stop and remove development services
make destroy

# Generate SSL certificates for local development
make certify

# Generate API code from OpenAPI specification
make generate-api

# Create new database migration
make create-migration migration_name=<name>

# Run all tests
make test

# Update local hosts
make set-hosts

# View all available targets
make help

# List all targets
make list

# Default target
make default

# Study mode (certification preparation)
make study
```

## API Documentation

The API is fully documented using OpenAPI 3.0.3 specification with comprehensive examples.

- **API Specification**: [docs/openapi-spec/svc-web-analyzer-api.yaml](docs/openapi-spec/svc-web-analyzer-api.yaml)
- **Generated Bundle**: [docs/openapi-spec/public/svc-web-analyzer-swagger-v1.json](docs/openapi-spec/public/svc-web-analyzer-swagger-v1.json)
- **Documentation**: https://docs.web-analyzer.dev (after running `make init`)
- **API Endpoint**: https://api.web-analyzer.dev/v1/

### Core Endpoints

- `POST /v1/analyze` - Submit URL for analysis
- `GET /v1/analysis/{analysisId}` - Get analysis result
- `GET /v1/analysis/{analysisId}/events` - Real-time progress (SSE)
- `GET /v1/health` - Health check endpoint

### API Examples

#### Health Check

```bash
curl -s https://api.web-analyzer.dev/v1/health | jq
```

**Output:**
```json
{
  "checks": {
    "cache": {
      "last_checked": "2025-09-28T16:09:48.955734545Z",
      "response_time": 6,
      "status": "healthy"
    },
    "queue": {
      "last_checked": "2025-09-28T16:09:48.957722705Z",
      "response_time": 1,
      "status": "healthy"
    },
    "storage": {
      "last_checked": "2025-09-28T16:09:48.948794987Z",
      "response_time": 11,
      "status": "healthy"
    }
  },
  "status": "OK",
  "timestamp": "2025-09-28T16:09:48.937689986Z",
  "uptime": 900.4823,
  "version": "1.0.0"
}
```

#### Submit URL for Analysis

**Testing with github.com:**
```bash
curl https://api.web-analyzer.dev/v1/analyze \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer v4.public.eyJhdWQiOiJ3ZWItYW5hbHl6ZXItYXBpIiwiZXhwIjoiMjA2My0wOS0xOFQwMjoyMDoxNyswMjowMCIsImlhdCI6IjIwMjUtMDktMjdUMDI6MjA6MTcrMDI6MDAiLCJpc3MiOiJ3ZWItYW5hbHl6ZXItc2VydmljZSIsImp0aSI6InByb3Blci1wYXNldG8tdjQtdG9rZW4iLCJuYmYiOiIyMDI1LTA5LTI3VDAyOjIwOjE3KzAyOjAwIiwic2NvcGVzIjpbImFuYWx5emUiLCJyZWFkIl0sInN1YiI6InRlc3QtdXNlciJ9MVH2eMTu9jMw6ZUIB538m-4gUoonWUbkHPDReqzD_2lojhtO2d1l3FXc6RCOozfW3fIdbU9y9SWAzBBamKydAQ" \
  -d '{
        "url": "https://github.com/login"
  }' | jq
```

**Response:**
```json
{
  "analysis_id": "52d22202-b31c-498f-855d-c1effd079ec1",
  "url": "https://github.com/login",
  "status": "requested",
  "created_at": "2025-09-28T16:09:49.353077Z"
}
```

#### Get Analysis Result

```bash
curl https://api.web-analyzer.dev/v1/analysis/52d22202-b31c-498f-855d-c1effd079ec1 \
  -H "Authorization: Bearer v4.public.eyJhdWQiOiJ3ZWItYW5hbHl6ZXItYXBpIiwiZXhwIjoiMjA2My0wOS0xOFQwMjoyMDoxNyswMjowMCIsImlhdCI6IjIwMjUtMDktMjdUMDI6MjA6MTcrMDI6MDAiLCJpc3MiOiJ3ZWItYW5hbHl6ZXItc2VydmljZSIsImp0aSI6InByb3Blci1wYXNldG8tdjQtdG9rZW4iLCJuYmYiOiIyMDI1LTA5LTI3VDAyOjIwOjE3KzAyOjAwIiwic2NvcGVzIjpbImFuYWx5emUiLCJyZWFkIl0sInN1YiI6InRlc3QtdXNlciJ9MVH2eMTu9jMw6ZUIB538m-4gUoonWUbkHPDReqzD_2lojhtO2d1l3FXc6RCOozfW3fIdbU9y9SWAzBBamKydAQ" | jq
```

**Response (Analysis Requested):**
```json
{
  "analysis_id": "52d22202-b31c-498f-855d-c1effd079ec1",
  "url": "https://github.com/login",
  "status": "requested",
  "created_at": "2025-09-28T16:09:49.353077Z"
}
```

> **Note:** The analysis is queued for processing. The endpoint successfully authenticates with PASETO v4 tokens and queues the analysis. Processing status will update as the analysis progresses through different stages.

#### Real-time Progress (Server-Sent Events)

The API provides real-time progress updates for analysis operations through Server-Sent Events (SSE). This allows clients to track the progress of long-running analysis tasks and provide live feedback to users.

**Endpoint:** `GET /v1/analysis/{analysisId}/events`

**Example Request:**
```bash
curl https://api.web-analyzer.dev/v1/analysis/52d22202-b31c-498f-855d-c1effd079ec1/events \
  -H "Authorization: Bearer v4.public.eyJhdWQiOiJ3ZWItYW5hbHl6ZXItYXBpIiwiZXhwIjoiMjA2My0wOS0xOFQwMjoyMDoxNyswMjowMCIsImlhdCI6IjIwMjUtMDktMjdUMDI6MjA6MTcrMDI6MDAiLCJpc3MiOiJ3ZWItYW5hbHl6ZXItc2VydmljZSIsImp0aSI6InByb3Blci1wYXNldG8tdjQtdG9rZW4iLCJuYmYiOiIyMDI1LTA5LTI3VDAyOjIwOjE3KzAyOjAwIiwic2NvcGVzIjpbImFuYWx5emUiLCJyZWFkIl0sInN1YiI6InRlc3QtdXNlciJ9MVH2eMTu9jMw6ZUIB538m-4gUoonWUbkHPDReqzD_2lojhtO2d1l3FXc6RCOozfW3fIdbU9y9SWAzBBamKydAQ" \
  -H "Accept: text/event-stream"
```

**SSE Event Stream Example:**
```
event: progress
data: {"stage": "queued", "progress": 0, "message": "Analysis queued", "timestamp": "2025-09-28T16:09:49.353Z"}

event: progress
data: {"stage": "fetching", "progress": 20, "message": "Fetching web page", "timestamp": "2025-09-28T16:09:50.123Z"}

event: progress
data: {"stage": "parsing_html", "progress": 50, "message": "Parsing HTML structure", "timestamp": "2025-09-28T16:09:51.456Z"}

event: progress
data: {"stage": "analyzing_links", "progress": 75, "message": "Analyzing links", "timestamp": "2025-09-28T16:09:52.789Z"}

event: completed
data: {"stage": "completed", "progress": 100, "message": "Analysis completed", "timestamp": "2025-09-28T16:09:53.123Z", "analysis_id": "52d22202-b31c-498f-855d-c1effd079ec1"}

event: error
data: {"stage": "error", "progress": 0, "message": "Failed to fetch web page: connection timeout", "timestamp": "2025-09-28T16:09:54.456Z", "error_code": "FETCH_TIMEOUT"}
```

**Event Types:**

- **`progress`**: Regular progress updates during analysis
- **`completed`**: Analysis successfully completed
- **`error`**: Analysis failed with error details
- **`heartbeat`**: Keep-alive events (sent every 30 seconds)

**Connection Management:**

- **Automatic Reconnection**: Browser EventSource automatically reconnects on connection loss
- **Timeout**: SSE connections timeout after 5 minutes of inactivity
- **Authentication**: PASETO token required in Authorization header
- **CORS**: Configured for cross-origin SSE connections

### Authentication

> **Note:** Authentication is required for all endpoints except `/v1/health`. The API supports both PASETO v4 and custom token formats.

**Authorization Header (Bearer Token)**
```bash
-H "Authorization: Bearer v4.public.{base64url-payload}{base64url-signature}"
```

**Working PASETO v4 Example:**
The following token is valid for 38 years and includes `analyze` and `read` scopes:
```bash
v4.public.eyJhdWQiOiJ3ZWItYW5hbHl6ZXItYXBpIiwiZXhwIjoiMjA2My0wOS0xOFQwMjoyMDoxNyswMjowMCIsImlhdCI6IjIwMjUtMDktMjdUMDI6MjA6MTcrMDI6MDAiLCJpc3MiOiJ3ZWItYW5hbHl6ZXItc2VydmljZSIsImp0aSI6InByb3Blci1wYXNldG8tdjQtdG9rZW4iLCJuYmYiOiIyMDI1LTA5LTI3VDAyOjIwOjE3KzAyOjAwIiwic2NvcGVzIjpbImFuYWx5emUiLCJyZWFkIl0sInN1YiI6InRlc3QtdXNlciJ9MVH2eMTu9jMw6ZUIB538m-4gUoonWUbkHPDReqzD_2lojhtO2d1l3FXc6RCOozfW3fIdbU9y9SWAzBBamKydAQ
```

**About PASETO:**
[PASETO (Platform-Agnostic Security Tokens)](https://paseto.io/) provides secure, authenticated tokens with Ed25519 signatures for v4 public tokens. The implementation supports both standard PASETO v4 tokens and backward-compatible custom formats.

## Configuration

The application is configured using environment variables. See `.envrc.dist` for available configuration options.

### Local Development
The project includes a complete local development setup:
- **SSL Certificates**: Automatic generation with mkcert
- **Local Domains**: `*.web-analyzer.dev` configured in `/etc/hosts`
- **Reverse Proxy**: Traefik configuration for service routing
- **Docker Compose**: Multi-service development environment

## Code Generation

The project uses a code-first approach with OpenAPI specification:

### API Generation Process
1. **Define**: Write OpenAPI 3.0.3 specification in `docs/openapi-spec/`
2. **Bundle**: Use Redocly CLI to create a unified specification
3. **Generate**: Use oapi-codegen to create Go server interfaces
4. **Implement**: Write business logic implementing the generated interfaces

### Generated Code
- **HTTP Server**: Generated interfaces and types in `internal/httpserver/`
- **API Bundle**: Single JSON specification for documentation
- **Examples**: Comprehensive request/response examples

## Security Features

- **PASETO Authentication**: Enhanced security tokens with issuer validation
- **Security Headers**: Complete set of standard security headers
- **CORS Configuration**: Configurable cross-origin resource sharing
- **Input Validation**: Schema-based validation from OpenAPI specification

## Testing

The project follows Go testing best practices with comprehensive test coverage:

### Test Structure
- **Unit Tests**: All service layer logic with mock repositories
- **Parallel Execution**: Tests run concurrently for faster execution
- **Mock Interfaces**: Using testify/mock for dependency isolation
- **Test Coverage**: Comprehensive coverage of success and error scenarios

### Running Tests
```bash
# Run all tests
make test

# Run service layer tests specifically
go test ./internal/service/ -v

# Run tests with coverage
go test -cover ./...

# Run tests in parallel (default behavior)
go test -parallel 8 ./...
```

### Test Categories
- **Service Layer**: Business logic testing with mocked dependencies
- **Configuration**: Environment and Vault configuration testing
- **Integration**: End-to-end API testing (planned)
- **Performance**: Load testing for analysis endpoints (planned)

## Development Tools

- **Make Targets**: Comprehensive build automation
- **Docker Integration**: Multi-stage builds and development containers
- **SSL/TLS**: Local development with valid certificates
- **API Documentation**: Auto-generated from OpenAPI specification
- **Testing Framework**: Testify for assertions and mocking

## TODOs

### Testing & Quality
- **Increase Test Coverage**: Expand unit and integration test coverage across all layers
    - Add comprehensive tests for domain entities and business logic
    - Implement integration tests for API endpoints
    - Add middleware and adapter layer testing
    - Target 80%+ code coverage across the codebase

### Performance & Scalability
- **Background Job Processing**: Implement queue-based analysis processing
    - Integrate message queue system (Redis Queue, RabbitMQ, or similar)
    - Move web page analysis to background workers
    - Implement job status tracking and progress updates
    - Add retry mechanisms for failed analysis jobs

- **Content Deduplication**: Optimize analysis efficiency with content hashing
    - Calculate SHA-256 hash of HTML content
    - Store hash-to-analysis mapping to avoid duplicate processing
    - Implement cache lookup before initiating new analysis
    - Return cached results for identical content

### Code Quality & Architecture
- **Refactoring Improvements**
    - Extract common patterns into reusable components
    - Simplify complex adapter implementations
    - Improve error handling consistency across layers
    - Optimize dependency injection and configuration management
    - Review and consolidate middleware implementations
    - Adding linting rules

### Observability & Maintainability
- **Enhanced Analysis Features**
    - Metrics and monitoring integration
    - Performance metrics collection
    - Security vulnerability detection

- **Operational Improvements**
    - CI/CD pipeline
    - K8s deployment

These improvements will enhance the application's reliability, performance, and maintainability while reducing operational overhead.
