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

## Infrastructure Features

### Development Environment
- **Docker-based Setup**: Complete containerized development environment.
- **SSL/TLS Support**: Local development with valid certificates using mkcert.
- **Reverse Proxy**: Traefik configuration for service routing.
- **Local Domains**: `*.web-analyzer.dev` domains for development.

### Build & Deployment
- **Make-based Build System**: Modular build configuration.
- **Code Generation**: OpenAPI-to-Go code generation with oapi-codegen.
- **API Documentation**: Auto-generated documentation from OpenAPI specification.
- **Multi-stage Docker Builds**: Optimized container images.

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

## Monitoring & Observability

### Logging
- **Structured Logging**: JSON-formatted logs for better parsing.
- **Request Tracking**: Correlation IDs for request tracing.
- **Error Logging**: Comprehensive error logging with stack traces.

### Health Monitoring
- **Health Checks**: Built-in health check endpoints.
