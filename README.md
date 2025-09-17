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

For complete feature documentation, see [Features Documentation](docs/features.md).

## Documentation

### Architecture & Design
- **[Architecture Decisions](docs/architecture-decisions.md)**: Comprehensive ADRs documenting all major architectural choices and their rationale
- **[Features Documentation](docs/features.md)**: Detailed documentation of all implemented features, APIs, and capabilities

### API Documentation
- **[OpenAPI Specification](docs/openapi-spec/svc-web-analyzer-api.yaml)**: Complete OpenAPI 3.0.3 specification
- **[Generated Documentation](https://docs.web-analyzer.dev)**: Interactive API documentation (available after running `make init`)

## Architecture

This project implements a **code-first API design** approach with comprehensive OpenAPI specification and generated server code.

### Project Structure

```
svc-web-analyzer/
├── assets/                       # Project assets and branding
│   └── logo.txt                 # ASCII art logo
├── build/                        # Build system and configuration
│   ├── mk/                      # Make-based build system
│   │   ├── Makefile            # Main Makefile with all targets
│   │   ├── config/             # Build configuration
│   │   └── utils.mk            # Build utilities
│   └── oapi/                    # OpenAPI code generation config
│       └── codegen.yaml        # oapi-codegen configuration
├── deployments/                  # Deployment configurations
│   └── docker/                  # Docker deployment setup
│       └── traefik/            # Traefik reverse proxy config
│           ├── dynamic.yaml    # Dynamic routing configuration
│           └── static.yaml     # Static Traefik configuration
├── docs/                         # Project documentation
│   ├── architecture-decisions.md # Architecture Decision Records
│   ├── features.md              # Feature documentation
│   └── openapi-spec/            # Complete OpenAPI 3.0.3 specification
│       ├── svc-web-analyzer-api.yaml # Main API specification
│       ├── schemas/             # Schema definitions
│       │   ├── analysis-*.yaml  # Analysis-related schemas
│       │   ├── health-*.yaml    # Health check schemas
│       │   ├── common/          # Shared schemas
│       │   ├── errors/          # Error response schemas
│       │   └── examples/        # Request/response examples
│       └── public/              # Generated API documentation
│           └── svc-web-analyzer-swagger-v1.json
├── internal/                     # Private application packages
│   ├── handlers/                # HTTP request handlers
│   │   └── http_server_gen.go  # Generated HTTP server code
│   └── tools/                   # Code generation tools
│       ├── generate.go         # Go generate directives
│       ├── go.mod             # Tools module definition
│       └── go.sum             # Tools dependencies
├── scripts/                      # Build and utility scripts
│   ├── build.sh               # Build automation script
│   ├── common.sh              # Common shell functions
│   └── logger.sh              # Logging utilities
├── web/                          # Frontend web application
├── compose.yaml                  # Symlink to deployments/docker/compose.yaml
├── go.mod                        # Go module definition
├── go.sum                        # Go module dependencies
├── Makefile                      # Symlink to build/mk/Makefile
└── oapi-codegen.yaml            # OpenAPI code generation config
```

## Technology Stack

### Backend
- **Language**: Go 1.25
- **Code Generation**: oapi-codegen for OpenAPI-to-Go conversion
- **Authentication**: PASETO tokens with enhanced security validation
- **API Specification**: OpenAPI 3.0.3 with comprehensive examples
- **Build System**: Make with modular build configuration

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
- Go 1.25+
- Docker and Docker Compose
- mkcert (for SSL certificates)

### Development Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/architeacher/svc-web-analyzer.git
   cd svc-web-analyzer
   ```

2. **Initialize and start the development environment**
   ```bash
   make init start
   ```
   This will:
   - Copy `.envrc.dist` to `.envrc` (edit as needed)
   - Add local domains to `/etc/hosts`
   - Generate SSL certificates with mkcert
   - Download Go dependencies with `go mod vendor`
   - Generate API code from OpenAPI specification
   - Start all services with Docker Compose

3. **Access the application**
   - **API**: https://api.web-analyzer.dev/v1/ (TBD: API documentation)
   - **Documentation**: https://docs.web-analyzer.dev
   - **Traefik Dashboard**: https://traefik.web-analyzer.dev (admin/admin)

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

# Run all tests
make test

# Update local hosts
make set-hosts

# View all available targets
make help

# List all targets
make list
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

## Development Tools

- **Make Targets**: Comprehensive build automation
- **Docker Integration**: Multi-stage builds and development containers
- **SSL/TLS**: Local development with valid certificates
- **API Documentation**: Auto-generated from OpenAPI specification

