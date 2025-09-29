# Features Documentation

This document provides comprehensive documentation of all features implemented in the Web Analyzer application.

## Core Analysis Features

### HTML Analysis
- **HTML Version Detection**: Automatically detects the HTML version (HTML5, XHTML, HTML 4.01, etc.).
- **Page Title Extraction**: Extracts and returns the page's title from the `<title>` tag.
- **Heading Analysis**: Counts headings by level (H1-H6) and provides structural insights.
- **Meta Tag Analysis**: Processes the meta tags for SEO and content information.

### Link Analysis
- **Internal Link Detection**: Identifies links that point to the same domain.
- **External Link Detection**: Catalogs links pointing to external domains.
- **Accessibility Checking**: Tests links for accessibility and reports inaccessible ones.
- **Link Classification**: Categorizes links by type (navigation, content, footer, etc.).

### Form Detection
- **Login Form Detection**: Specifically identifies login forms based on field patterns.
- **Form Structure Analysis**: Analyzes form elements, input types, and validation patterns.
- **Security Assessment**: Checks for proper form security implementations.

## API Features

### Authentication & Security
- **PASETO Token Authentication**: Enhanced security tokens with issuer validation.
  - Platform Agnostic Security Token Exchange and Operations.
  - Extended validation with expiration checks.
  - Issuer verification for enhanced security.
- **Security Headers**: Comprehensive security header implementation.
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `X-XSS-Protection: 1; mode=block`
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains`
  - `Content-Security-Policy: default-src 'self'`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Permissions-Policy: camera=(), microphone=(), geolocation=()`

### API Versioning
- **Multiple Versioning Strategies**:
  - **URL Path Versioning**: `/v1/` (primary method)
  - **Header Versioning**: `API-Version: v1` header (alternative)
  - **Content Type Versioning**: `application/vnd.web-analyzer.v1+json`
- **Version Information**: All responses include `API-Version` header.
- **Backward Compatibility**: Semantic versioning with clear breaking change policies.

### Real-time Features
- **Server-Sent Events (SSE)**: Live progress updates during analysis.
  - Endpoint: `GET /v1/analysis/{analysisId}/events`
  - Real-time status updates.
  - Progress tracking.
  - Error notifications.
- **Automatic Reconnection**: Browser handles connection drops automatically.

### Request/Response Features
- **Comprehensive Error Handling**: Structured error responses with detailed information.
- **Schema Validation**: Request/response validation based on OpenAPI specification.
- **Example Responses**: Complete examples for all endpoints and scenarios.
- **Content Negotiation**: Support for multiple content types.

## Backend Architecture Features

### Event-Driven Architecture
- **Publisher/Subscriber Pattern**: Decoupled services for scalable message processing
  - **Publisher Service**: Monitors outbox events and publishes to message queue
  - **Subscriber Service**: Consumes analysis events and processes web pages
  - **HTTP API Service**: Handles REST endpoints and real-time updates
- **Outbox Pattern**: Transactional event publishing for guaranteed delivery
  - Atomic database transactions with event storage
  - Reliable message publishing with retry mechanisms
  - No message loss guarantee with PostgreSQL persistence
- **Message Queue Integration**: RabbitMQ-based asynchronous communication
  - Dead letter queues for failed message handling
  - Message persistence and acknowledgment
  - Concurrent worker processing with horizontal scaling

### Clean Architecture Implementation
- **Ports and Adapters**: Clear separation between business logic and infrastructure
  - **Ports**: Interface definitions for external dependencies
  - **Adapters**: Concrete implementations for databases, queues, and external services
  - **Domain Layer**: Pure business logic without external dependencies
- **Dependency Injection**: Runtime configuration and service composition
  - Centralized dependency management in `internal/runtime/`
  - Interface-based dependency injection for testability
  - Environment-specific configuration management
- **CQRS Pattern**: Command Query Responsibility Segregation
  - Separate command and query handlers with dedicated decorators
  - Optimized read/write operations for different use cases
  - Clear separation of concerns for data access patterns

### Service Layer Architecture
- **Application Services**: Orchestration of business operations
  - Transaction management and coordination
  - Cross-cutting concern integration (logging, metrics, tracing)
  - Use case implementation with comprehensive error handling
- **Decorator Pattern**: Cross-cutting concerns implementation
  - **Logging Decorators**: Structured logging for commands and queries
  - **Metrics Decorators**: Performance monitoring and business metrics
  - **Tracing Decorators**: Distributed tracing with OpenTelemetry
  - **Validation Decorators**: Input validation and business rule enforcement

### Repository Pattern
- **Multiple Implementations**: Support for different storage backends
  - **PostgreSQL Repository**: Primary data persistence with ACID compliance
  - **Cache Repository**: KeyDB/Redis integration for temporary data
  - **Vault Repository**: Secure configuration and secret management
- **Interface Abstractions**: Storage-agnostic business logic
  - Clean interfaces for all data access operations
  - Easy swapping of storage implementations
  - Comprehensive mock support for testing

### Infrastructure Features

### Development Environment
- **Docker-based Setup**: Complete containerized development environment
- **SSL/TLS Support**: Local development with valid certificates using mkcert
- **Reverse Proxy**: Traefik configuration for service routing
- **Local Domains**: `*.web-analyzer.dev` domains for development
- **Multi-Service Architecture**: Separate containers for API, publisher, subscriber, and infrastructure

### Build & Deployment
- **Make-based Build System**: Modular build configuration
- **Code Generation**: OpenAPI-to-Go code generation with oapi-codegen
- **API Documentation**: Auto-generated documentation from OpenAPI specification
- **Multi-stage Docker Builds**: Optimized container images
- **Database Migrations**: Automated schema management with version control

## User Experience Features

### API Documentation
- **Interactive Documentation**: Auto-generated from OpenAPI 3.0.3 specification.
- **Comprehensive Examples**: Request/response examples for all endpoints.
- **Schema Documentation**: Detailed schema definitions with validation rules.
- **Try-it-out Interface**: Interactive API testing from documentation.

### Development Tools
- **Health Check Endpoint**: `GET /v1/health` for service monitoring.
- **Comprehensive Logging**: Structured logging for debugging and monitoring.
- **Error Reporting**: Detailed error messages with correlation IDs.
- **API Explorer**: Browser-based API testing interface.

## Endpoint Features

### Analysis Endpoints

#### POST /v1/analyze
- **Purpose**: Submit URL for analysis.
- **Features**:
  - URL validation and sanitization.
  - Asynchronous processing.
  - Unique analysis ID generation.
  - Progress tracking initialization.
- **Response**: Analysis ID for tracking progress.

#### GET /v1/analysis/{analysisId}
- **Purpose**: Retrieve analysis results.
- **Features**:
  - Result caching.
  - Complete analysis data.
  - Structured response format.
  - Error handling for non-existent analyses.

#### GET /v1/analysis/{analysisId}/events
- **Purpose**: Real-time progress updates via SSE.
- **Features**:
  - Live progress streaming.
  - Connection management.
  - Automatic retry logic.
  - Error event handling.

#### GET /v1/health
- **Purpose**: Service health monitoring.
- **Features**:
  - Service status check.
  - Dependency health validation.
  - Response time metrics.
  - Version information.

## Security Features

### Input Validation
- **URL Validation**: Comprehensive URL format and security validation.
- **Schema Validation**: Request validation against OpenAPI schemas.
- **Sanitization**: Input sanitization to prevent injection attacks.
- **Rate Limiting**: Protection against abuse and DoS attacks.

### Data Protection
- **No Persistent Storage**: Analysis results are temporary by design.
- **Secure Communication**: HTTPS enforcement for all communications.
- **Token Security**: Secure token validation and lifecycle management.
- **Privacy Protection**: No logging of sensitive URL content.

## Performance Features

### Optimization
- **Resource Management**: Proper cleanup of resources and connections.

### Scalability
- **Stateless Design**: Horizontally scalable architecture.
- **Load Balancer Ready**: Traefik integration for load balancing.
- **Container Orchestration**: Kubernetes-ready deployment.

## Testing & Quality Features

### Comprehensive Testing Strategy
- **Unit Tests**: Complete test coverage for all service layers
  - **Service Layer Tests**: Business logic testing with mocked dependencies
  - **Repository Tests**: Data access layer testing with test databases
  - **Adapter Tests**: Infrastructure component testing with real integrations
- **Parallel Test Execution**: Optimized test performance with concurrent execution
  - Go test parallel execution with `-parallel` flag
  - Independent test isolation for reliable results
  - Comprehensive test coverage across all layers
- **Mock Framework Integration**: Testify framework for dependency mocking
  - Interface-based mocking for all external dependencies
  - Comprehensive test scenarios for success and error cases
  - Easy test maintenance and refactoring support

### Code Quality & Architecture
- **Clean Code Principles**: Consistent code organization and standards
  - Early returns and minimal nesting for better readability
  - Comprehensive error handling with structured error types
  - Interface-based design for testability and flexibility
- **Documentation Coverage**: Comprehensive code and API documentation
  - OpenAPI specification as single source of truth
  - Auto-generated documentation from code
  - Architecture decision records for design rationale

## Persistent Storage Features

### PostgreSQL Integration
- **ACID Compliance**: Full transaction support for data integrity
- **JSON/JSONB Support**: Flexible analysis data storage with native JSON types
- **Outbox Events Table**: Event sourcing implementation for reliable message delivery
- **Migration System**: Automated database schema management
- **Connection Pooling**: Optimized database connection management

### Vault Secret Management
- **Centralized Secrets**: Secure storage for all sensitive configuration
- **Dynamic Credentials**: Automatic credential rotation for database access
- **Policy-based Access**: Fine-grained access control for different services
- **Audit Logging**: Complete audit trail for all secret access operations

### KeyDB Caching
- **High-Performance Cache**: Redis-compatible in-memory data store
- **TTL-based Cleanup**: Automatic expiration for temporary analysis results
- **Clustering Support**: Horizontal scaling with KeyDB clustering
- **Connection Management**: Optimized connection pooling and health monitoring

## Monitoring & Observability

### Distributed Tracing
- **OpenTelemetry Integration**: Complete tracing across all service boundaries
- **Request Correlation**: End-to-end request tracking with trace IDs
- **Performance Monitoring**: Detailed performance metrics for all operations
- **Error Tracking**: Comprehensive error tracking with contextual information

### Logging & Metrics
- **Structured Logging**: JSON-formatted logs with consistent schema
- **Request Tracking**: Correlation IDs for request tracing across services
- **Error Logging**: Comprehensive error logging with stack traces and context
- **Business Metrics**: Custom metrics for analysis operations and performance
- **Infrastructure Metrics**: System health and resource utilization monitoring

### Health Monitoring
- **Health Checks**: Built-in health check endpoints for all services
- **Dependency Health**: Monitoring of database, cache, and queue connections
- **Service Discovery**: Automatic service health registration with Traefik
- **Graceful Shutdown**: Proper resource cleanup on service termination
