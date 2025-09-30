# Web Analyzer Service - Improvement Plan

> **Generated**: 2025-10-18
> **Updated**: 2025-10-28
> **Status**: In Progress - 2 Critical Improvements Completed
> **Priority**: Critical improvements identified for production readiness

## Executive Summary

After comprehensive analysis of the Web Analyzer service documentation and codebase, this plan identifies 19 remaining key improvements needed to bridge the gap between the documented architecture and current implementation. These improvements range from critical infrastructure components to enhanced operational capabilities, including comprehensive performance testing with k6, end-to-end testing with Playwright, a data lifecycle management system with a janitor service, a full API Gateway with HTTP→gRPC protocol conversion, priority-based worker allocation, analysis results export in multiple formats (CSV, PDF, JSON, Excel), webhook integration for event notifications, standardized C4 architecture documentation, and resolution of all TODO comments found in the codebase for production readiness.

## Completed Improvements ✅

1. **Complete Metrics Implementation** - Implemented OpenTelemetry metrics with OTEL Collector, Prometheus, Grafana, and Jaeger for distributed tracing
2. **Fix HTTP Access Log Middleware Format** - Converted plain text logs to structured JSON format with zerolog

## Priority Matrix

| Priority | Count | Focus Areas |
|----------|-------|-------------|
| 🔴 CRITICAL | 1 | CI/CD Pipeline |
| 🟡 HIGH | 12 | Code Quality, Testing, Security (RFC 9421), CSRF Protection, Secret Rotation & Vault Lease/TTL, Data Lifecycle (Janitor), API Gateway, TODO Comments Resolution, Export Functionality, Webhook Integration, Performance Testing (k6), E2E Testing (Playwright) |
| 🟢 MEDIUM | 4 | Pipeline, Kubernetes, Resilience, Rate Limiting |
| 🔵 LOW | 2 | Documentation, C4 Architecture Diagrams |

---

## 1. CI/CD Pipeline Setup with Security & Automation 🔴 **CRITICAL**


### Current State
- No `.github/workflows` directory exists
- No automated testing or deployment
- Manual build and deployment process
- No security scanning
- No automated dependency updates
- No vulnerability detection

### Overview

Implement comprehensive CI/CD pipeline with security-first approach, including SAST/DAST scanning, automated dependency management, and continuous security monitoring.

### Required Workflows

```yaml
GitHub Actions Workflows Architecture:
  ├── ci.yaml                      # Continuous Integration (tests, build, quality)
  ├── cd.yaml                      # Continuous Deployment (staging, production)
  ├── security-sast.yaml           # SAST security scanning
  ├── security-dast.yaml           # DAST security scanning
  ├── dependency-review.yaml       # Dependency vulnerability scanning
  ├── container-scan.yaml          # Container image security
  ├── license-check.yaml           # License compliance verification
  ├── secret-scan.yaml             # Secret detection in code
  ├── codeql-analysis.yaml         # GitHub Advanced Security (CodeQL)
  └── performance-benchmark.yml   # Performance regression detection
```

---

### 1. Continuous Integration Workflow

**File**: `.github/workflows/ci.yml`

```yaml
name: Continuous Integration

on:
  pull_request:
    branches: [main, develop]
  push:
    branches: [main, develop]
  workflow_dispatch:

env:
  GO_VERSION: '1.25.3'
  GOLANGCI_LINT_VERSION: 'v1.55'

jobs:
  # Job 1: Code Quality and Linting
  lint:
    name: Lint and Code Quality
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: ${{ env.GOLANGCI_LINT_VERSION }}
          args: --timeout=10m --config=.golangci.yml

      - name: Check code formatting
        run: |
          if [ -n "$(gofmt -s -l .)" ]; then
            echo "Go code is not formatted:"
            gofmt -s -d .
            exit 1
          fi

      - name: Go mod tidy check
        run: |
          go mod tidy
          git diff --exit-code go.mod go.sum

  # Job 2: Unit Tests with Coverage
  test:
    name: Unit Tests
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.25.3']
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go-version }}
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Run unit tests with coverage
        run: |
          go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

      - name: Check test coverage threshold
        run: |
          total_coverage=$(go tool cover -func=coverage.txt | grep total | awk '{print $3}' | sed 's/%//')
          threshold=80
          if (( $(echo "$total_coverage < $threshold" | bc -l) )); then
            echo "Coverage $total_coverage% is below threshold $threshold%"
            exit 1
          fi
          echo "Coverage: $total_coverage% (threshold: $threshold%)"

      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.txt
          flags: unittests
          name: codecov-umbrella
          token: ${{ secrets.CODECOV_TOKEN }}
          fail_ci_if_error: true

  # Job 3: Integration Tests
  integration-test:
    name: Integration Tests
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: web_analyzer_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

      rabbitmq:
        image: rabbitmq:3.12-management-alpine
        env:
          RABBITMQ_DEFAULT_USER: admin
          RABBITMQ_DEFAULT_PASS: admin
        ports:
          - 5672:5672
          - 15672:15672

      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Run database migrations
        run: |
          go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
          migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/web_analyzer_test?sslmode=disable" up

      - name: Run integration tests
        env:
          DB_HOST: localhost
          DB_PORT: 5432
          DB_USER: postgres
          DB_PASSWORD: postgres
          DB_NAME: web_analyzer_test
          RABBITMQ_URL: amqp://admin:admin@localhost:5672/
          REDIS_URL: redis://localhost:6379
        run: |
          go test -v -tags=integration ./itest/...

  # Job 4: Build and Validate
  build:
    name: Build Application
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Build binaries
        run: |
          make build

      - name: Build Docker images
        run: |
          docker build -t web-analyzer/api:${{ github.sha }} -f deployments/docker/Dockerfile --target api .
          docker build -t web-analyzer/publisher:${{ github.sha }} -f deployments/docker/Dockerfile --target publisher .
          docker build -t web-analyzer/subscriber:${{ github.sha }} -f deployments/docker/Dockerfile --target subscriber .

      - name: Save Docker images
        run: |
          docker save web-analyzer/api:${{ github.sha }} | gzip > api-image.tar.gz
          docker save web-analyzer/publisher:${{ github.sha }} | gzip > publisher-image.tar.gz
          docker save web-analyzer/subscriber:${{ github.sha }} | gzip > subscriber-image.tar.gz

      - name: Upload Docker images as artifacts
        uses: actions/upload-artifact@v4
        with:
          name: docker-images
          path: |
            api-image.tar.gz
            publisher-image.tar.gz
            subscriber-image.tar.gz
          retention-days: 7
```

---

### 2. SAST Security Scanning Workflow

**File**: `.github/workflows/security-sast.yml`

```yaml
name: SAST Security Scanning

on:
  pull_request:
    branches: [main, develop]
  push:
    branches: [main, develop]
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
  workflow_dispatch:

jobs:
  # Job 1: GoSec - Go Security Checker
  gosec:
    name: GoSec Security Scanner
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Run GoSec Security Scanner
        uses: securego/gosec@master
        with:
          args: '-fmt sarif -out gosec-results.sarif ./...'

      - name: Upload GoSec results to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: gosec-results.sarif
          category: gosec

      - name: Upload GoSec results as artifact
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: gosec-results
          path: gosec-results.sarif

  # Job 2: Semgrep - Static Analysis
  semgrep:
    name: Semgrep Security Analysis
    runs-on: ubuntu-latest
    container:
      image: returntocorp/semgrep
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Run Semgrep
        run: |
          semgrep scan --config=auto --sarif --output=semgrep-results.sarif

      - name: Upload Semgrep results to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: semgrep-results.sarif
          category: semgrep

      - name: Upload Semgrep results as artifact
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: semgrep-results
          path: semgrep-results.sarif

  # Job 3: CodeQL Analysis (GitHub Advanced Security)
  codeql:
    name: CodeQL Analysis
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      actions: read
      contents: read
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: go
          queries: security-extended,security-and-quality

      - name: Autobuild
        uses: github/codeql-action/autobuild@v3

      - name: Perform CodeQL Analysis
        uses: github/codeql-action/analyze@v3
        with:
          category: "/language:go"

  # Job 4: Nancy - Go Dependency Vulnerability Scanner
  nancy:
    name: Nancy Dependency Scanner
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.3'

      - name: Install Nancy
        run: go install github.com/sonatype-nexus-community/nancy@latest

      - name: Run Nancy scan
        run: |
          go list -json -deps ./... | nancy sleuth --output=json > nancy-results.json

      - name: Upload Nancy results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: nancy-results
          path: nancy-results.json

  # Job 5: govulncheck - Official Go Vulnerability Scanner
  govulncheck:
    name: Go Vulnerability Check
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.3'

      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest

      - name: Run govulncheck
        run: |
          govulncheck -format json ./... > govulncheck-results.json

      - name: Upload govulncheck results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: govulncheck-results
          path: govulncheck-results.json

      - name: Check for vulnerabilities
        run: |
          if govulncheck ./... | grep -q "Vulnerability"; then
            echo "Vulnerabilities found!"
            exit 1
          fi
```

---

### 3. Secret Scanning Workflow

**File**: `.github/workflows/secret-scan.yml`

```yaml
name: Secret Scanning

on:
  pull_request:
    branches: [main, develop]
  push:
    branches: [main, develop]
  schedule:
    - cron: '0 3 * * *'  # Daily at 3 AM

jobs:
  # Job 1: Gitleaks - Secret Detection
  gitleaks:
    name: Gitleaks Secret Scanner
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run Gitleaks
        uses: gitleaks/gitleaks-action@v2
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GITLEAKS_LICENSE: ${{ secrets.GITLEAKS_LICENSE }}

  # Job 2: TruffleHog - Secret Scanner
  trufflehog:
    name: TruffleHog Secret Scanner
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run TruffleHog
        uses: trufflesecurity/trufflehog@main
        with:
          path: ./
          base: ${{ github.event.repository.default_branch }}
          head: HEAD
          extra_args: --debug --only-verified
```

---

### 4. Container Security Scanning

**File**: `.github/workflows/container-scan.yml`

```yaml
name: Container Security Scanning

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
  schedule:
    - cron: '0 4 * * *'  # Daily at 4 AM

jobs:
  # Job 1: Trivy Container Scan
  trivy:
    name: Trivy Container Scanner
    runs-on: ubuntu-latest
    strategy:
      matrix:
        image: [api, publisher, subscriber]
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Build Docker image
        run: |
          docker build -t web-analyzer/${{ matrix.image }}:${{ github.sha }} \
            -f deployments/docker/Dockerfile \
            --target ${{ matrix.image }} .

      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: web-analyzer/${{ matrix.image }}:${{ github.sha }}
          format: 'sarif'
          output: 'trivy-${{ matrix.image }}-results.sarif'
          severity: 'CRITICAL,HIGH,MEDIUM'
          exit-code: '1'

      - name: Upload Trivy results to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: trivy-${{ matrix.image }}-results.sarif
          category: trivy-${{ matrix.image }}

      - name: Generate SBOM
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: web-analyzer/${{ matrix.image }}:${{ github.sha }}
          format: 'cyclonedx'
          output: 'sbom-${{ matrix.image }}.json'

      - name: Upload SBOM
        uses: actions/upload-artifact@v4
        with:
          name: sbom-${{ matrix.image }}
          path: sbom-${{ matrix.image }}.json

  # Job 2: Grype Container Scanner
  grype:
    name: Grype Container Scanner
    runs-on: ubuntu-latest
    strategy:
      matrix:
        image: [api, publisher, subscriber]
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Build Docker image
        run: |
          docker build -t web-analyzer/${{ matrix.image }}:${{ github.sha }} \
            -f deployments/docker/Dockerfile \
            --target ${{ matrix.image }} .

      - name: Run Grype scanner
        uses: anchore/scan-action@v3
        with:
          image: web-analyzer/${{ matrix.image }}:${{ github.sha }}
          fail-build: true
          severity-cutoff: high
          output-format: sarif

      - name: Upload Grype results
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: ${{ steps.scan.outputs.sarif }}
```

---

### 5. Dependency Review and Management

**File**: `.github/workflows/dependency-review.yml`

```yaml
name: Dependency Review

on:
  pull_request:
    branches: [main, develop]

permissions:
  contents: read
  pull-requests: write

jobs:
  dependency-review:
    name: Dependency Review
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Dependency Review
        uses: actions/dependency-review-action@v4
        with:
          fail-on-severity: moderate
          allow-licenses: MIT, Apache-2.0, BSD-3-Clause, ISC
          deny-licenses: GPL-3.0, AGPL-3.0
```

---

### 6. Automated Dependency Updates - Dependabot

**File**: `.github/dependabot.yml`

```yaml
version: 2
updates:
  # Go modules
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "03:00"
    open-pull-requests-limit: 10
    reviewers:
      - "web-analyzer-team"
    labels:
      - "dependencies"
      - "go"
    commit-message:
      prefix: "chore"
      prefix-development: "chore"
      include: "scope"
    versioning-strategy: increase
    groups:
      go-dependencies:
        patterns:
          - "*"
        update-types:
          - "minor"
          - "patch"

  # GitHub Actions
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
      time: "03:00"
    open-pull-requests-limit: 5
    labels:
      - "dependencies"
      - "github-actions"
    commit-message:
      prefix: "ci"

  # Docker
  - package-ecosystem: "docker"
    directory: "/deployments/docker"
    schedule:
      interval: "weekly"
      day: "tuesday"
      time: "03:00"
    open-pull-requests-limit: 5
    labels:
      - "dependencies"
      - "docker"
    commit-message:
      prefix: "build"

  # npm (for frontend if applicable)
  - package-ecosystem: "npm"
    directory: "/web"
    schedule:
      interval: "weekly"
      day: "wednesday"
      time: "03:00"
    open-pull-requests-limit: 10
    labels:
      - "dependencies"
      - "javascript"
    commit-message:
      prefix: "chore"
    groups:
      dev-dependencies:
        dependency-type: "development"
      production-dependencies:
        dependency-type: "production"
        update-types:
          - "minor"
          - "patch"
```

---

### 7. Automated Dependency Updates - Renovate (Alternative)

**File**: `.github/renovate.json`

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "config:recommended",
    ":dependencyDashboard",
    ":semanticCommits",
    ":automergeDigest",
    ":automergeMinor"
  ],
  "timezone": "America/New_York",
  "schedule": [
    "after 10pm every weekday",
    "before 5am every weekday",
    "every weekend"
  ],
  "labels": ["dependencies", "renovate"],
  "assignees": ["@web-analyzer-team"],
  "reviewers": ["@web-analyzer-team"],
  "vulnerabilityAlerts": {
    "enabled": true,
    "labels": ["security", "dependencies"],
    "assignees": ["@security-team"]
  },
  "packageRules": [
    {
      "matchUpdateTypes": ["minor", "patch"],
      "matchCurrentVersion": "!/^0/",
      "automerge": true,
      "automergeType": "pr",
      "platformAutomerge": true
    },
    {
      "matchDepTypes": ["devDependencies"],
      "automerge": true
    },
    {
      "matchPackagePatterns": ["^go.opentelemetry.io"],
      "groupName": "OpenTelemetry packages",
      "automerge": false
    },
    {
      "matchPackagePatterns": ["^github.com/redis"],
      "groupName": "Redis packages"
    },
    {
      "matchDatasources": ["docker"],
      "enabled": true,
      "commitMessageTopic": "Docker tag {{depName}}",
      "groupName": "Docker images"
    }
  ],
  "golang": {
    "enabled": true,
    "commitMessageTopic": "module {{depName}}"
  },
  "docker": {
    "enabled": true,
    "major": {
      "enabled": false
    }
  },
  "prConcurrentLimit": 10,
  "prHourlyLimit": 5,
  "enabledManagers": ["gomod", "docker-compose", "dockerfile", "github-actions"],
  "separateMajorMinor": true,
  "separateMultipleMajor": true,
  "separateMinorPatch": false
}
```

---

### 8. License Compliance Checking

**File**: `.github/workflows/license-check.yml`

```yaml
name: License Compliance

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
  schedule:
    - cron: '0 5 * * 0'  # Weekly on Sunday at 5 AM

jobs:
  license-check:
    name: License Compliance Check
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.3'

      - name: Install go-licenses
        run: go install github.com/google/go-licenses@latest

      - name: Check licenses
        run: |
          go-licenses check ./... --allowed_licenses=MIT,Apache-2.0,BSD-3-Clause,ISC,MPL-2.0

      - name: Generate license report
        run: |
          go-licenses report ./... > license-report.csv

      - name: Upload license report
        uses: actions/upload-artifact@v4
        with:
          name: license-report
          path: license-report.csv

      - name: Check for forbidden licenses
        run: |
          if go-licenses check ./... --disallowed_types=forbidden,restricted | grep -q "forbidden\|restricted"; then
            echo "Forbidden or restricted licenses detected!"
            exit 1
          fi
```

---

### 9. Continuous Deployment Workflow

**File**: `.github/workflows/cd.yml`

```yaml
name: Continuous Deployment

on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      environment:
        description: 'Deployment environment'
        required: true
        type: choice
        options:
          - staging
          - production

jobs:
  # Job 1: Deploy to Staging
  deploy-staging:
    name: Deploy to Staging
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main' && github.event_name == 'push'
    environment:
      name: staging
      url: https://staging.web-analyzer.dev
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Download Docker images
        uses: actions/download-artifact@v4
        with:
          name: docker-images

      - name: Load Docker images
        run: |
          docker load < api-image.tar.gz
          docker load < publisher-image.tar.gz
          docker load < subscriber-image.tar.gz

      - name: Tag images for staging
        run: |
          docker tag web-analyzer/api:${{ github.sha }} registry.web-analyzer.dev/api:staging
          docker tag web-analyzer/publisher:${{ github.sha }} registry.web-analyzer.dev/publisher:staging
          docker tag web-analyzer/subscriber:${{ github.sha }} registry.web-analyzer.dev/subscriber:staging

      - name: Push to container registry
        run: |
          echo ${{ secrets.REGISTRY_PASSWORD }} | docker login registry.web-analyzer.dev -u ${{ secrets.REGISTRY_USERNAME }} --password-stdin
          docker push registry.web-analyzer.dev/api:staging
          docker push registry.web-analyzer.dev/publisher:staging
          docker push registry.web-analyzer.dev/subscriber:staging

      - name: Deploy to staging with ArgoCD
        run: |
          argocd app sync web-analyzer-staging --prune

      - name: Wait for deployment
        run: |
          argocd app wait web-analyzer-staging --health --timeout 600

      - name: Run smoke tests
        run: |
          curl -f https://staging.web-analyzer.dev/v1/health || exit 1

  # Job 2: Deploy to Production (Manual Approval)
  deploy-production:
    name: Deploy to Production
    runs-on: ubuntu-latest
    needs: deploy-staging
    if: github.event_name == 'workflow_dispatch' && github.event.inputs.environment == 'production'
    environment:
      name: production
      url: https://api.web-analyzer.dev
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Download Docker images
        uses: actions/download-artifact@v4
        with:
          name: docker-images

      - name: Tag images for production
        run: |
          docker tag web-analyzer/api:${{ github.sha }} registry.web-analyzer.dev/api:${{ github.sha }}
          docker tag web-analyzer/api:${{ github.sha }} registry.web-analyzer.dev/api:latest

      - name: Push to production registry
        run: |
          docker push registry.web-analyzer.dev/api:${{ github.sha }}
          docker push registry.web-analyzer.dev/api:latest

      - name: Deploy to production with ArgoCD
        run: |
          argocd app set web-analyzer-prod --image registry.web-analyzer.dev/api:${{ github.sha }}
          argocd app sync web-analyzer-prod --prune

      - name: Wait for deployment
        run: |
          argocd app wait web-analyzer-prod --health --timeout 600

      - name: Run production smoke tests
        run: |
          curl -f https://api.web-analyzer.dev/v1/health || exit 1

      - name: Create GitHub Release
        uses: actions/create-release@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          tag_name: v${{ github.run_number }}
          release_name: Release v${{ github.run_number }}
          body: |
            Production deployment of commit ${{ github.sha }}
          draft: false
          prerelease: false
```

---

### 10. Implementation Checklist

#### Security Scanning
- [ ] Configure GoSec for SAST scanning
- [ ] Set up Semgrep with custom rules
- [ ] Enable GitHub CodeQL Advanced Security
- [ ] Configure Nancy for dependency scanning
- [ ] Set up govulncheck for Go vulnerabilities
- [ ] Configure Gitleaks for secret detection
- [ ] Set up TruffleHog for additional secret scanning
- [ ] Configure Trivy for container scanning
- [ ] Set up Grype as backup container scanner

#### Dependency Management
- [ ] Configure Dependabot for automated updates
- [ ] Set up Renovate as alternative (choose one)
- [ ] Configure dependency grouping rules
- [ ] Set up auto-merge for safe updates
- [ ] Configure vulnerability alerts
- [ ] Set up license compliance checking
- [ ] Configure forbidden license detection

#### CI/CD Pipeline
- [ ] Create CI workflow for builds and tests
- [ ] Set up integration testing workflow
- [ ] Configure code coverage requirements (80%+)
- [ ] Set up CD workflow for staging
- [ ] Configure manual approval for production
- [ ] Set up rollback procedures
- [ ] Create smoke test suite
- [ ] Configure deployment notifications

#### Monitoring & Reporting
- [ ] Set up SARIF upload to GitHub Security
- [ ] Configure Codecov for coverage reporting
- [ ] Create security dashboard
- [ ] Set up vulnerability tracking
- [ ] Configure Slack/Teams notifications
- [ ] Create weekly security reports
- [ ] Set up dependency update summaries

---

### 11. Security Best Practices

#### SAST Configuration
```yaml
# .golangci.yml additions for security
linters:
  enable:
    - gosec          # Security checker
    - bodyclose      # HTTP response body closure
    - errcheck       # Error handling
    - exportloopref  # Loop variable capture
    - gocritic       # Code quality
    - gocyclo        # Complexity checking
    - revive         # Fast linter
    - staticcheck    # Static analysis

linters-settings:
  gosec:
    excludes:
      - G104  # Audit errors not checked (we use errcheck)
    severity: "medium"
    confidence: "medium"
```

#### Secret Scanning Configuration
```yaml
# .gitleaks.toml
title = "Web Analyzer Gitleaks Configuration"

[[rules]]
id = "aws-access-key"
description = "AWS Access Key"
regex = '''(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}'''

[[rules]]
id = "github-token"
description = "GitHub Token"
regex = '''ghp_[0-9a-zA-Z]{36}'''

[[rules]]
id = "private-key"
description = "Private Key"
regex = '''-----BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY-----'''

[allowlist]
paths = [
  '''\.md$''',
  '''\.txt$''',
  '''test/fixtures/'''
]
```

---

### 12. Benefits

**Security:**
- Automated vulnerability detection at multiple layers
- Continuous secret scanning prevents credential leaks
- License compliance ensures legal safety
- Container security prevents runtime exploits
- SBOM generation for supply chain security

**Automation:**
- Zero-touch dependency updates for safe patches
- Automated security patch application
- Continuous security monitoring
- Reduced manual review burden

**Quality:**
- Enforced code coverage thresholds
- Automated code quality checks
- Integration testing in realistic environments
- Performance regression detection

**Compliance:**
- Audit trail for all deployments
- License compliance verification
- Vulnerability tracking and remediation
- Security policy enforcement

---

## 2. Secret Rotation and Vault Lease/TTL Management 🟡 **HIGH**

### Current State
- Secrets fetched from Vault at application startup only
- No automatic renewal of expiring secrets
- No handling of Vault lease expiration
- Static credentials for database, RabbitMQ, and other services
- Secrets remain valid until application restart
- No graceful handling of credential rotation

### Problem

**Current Implementation Issues:**

1. **Static Secret Management**
   - Secrets loaded once during application initialization
   - No mechanism to detect or handle secret expiration
   - Requires service restart to pick up new credentials
   - Vulnerable to credential compromise with extended exposure window

2. **No Lease Awareness**
   - Vault dynamic secrets have TTL (Time-To-Live) but not monitored
   - Leases expire without renewal, causing service failures
   - No automatic refresh before expiration
   - Missing lease renewal logic in all three services

3. **Database Credential Risk**
   - PostgreSQL credentials never rotate
   - Static credentials across all environments
   - No automated rotation for database connections
   - Connection pools not designed for credential updates

4. **RabbitMQ Credential Risk**
   - Static RabbitMQ credentials
   - No integration with Vault's RabbitMQ secrets engine
   - Manual credential rotation requires downtime

5. **PASETO Signing Key Risk**
   - Single signing key with no rotation strategy
   - No support for key versioning or rollover
   - Key compromise affects all tokens immediately

### Security Impact

**Without secret rotation:**
- **Extended Exposure Window**: Compromised secrets remain valid indefinitely
- **Compliance Violations**: Fails PCI DSS, SOC 2, HIPAA rotation requirements
- **Lateral Movement**: Attackers can maintain persistent access
- **Audit Failures**: No evidence of regular credential rotation
- **Service Disruption**: Expired leases cause unexpected failures

**Industry Standards:**
- **Database Credentials**: Rotate every 24-48 hours
- **API Tokens**: Rotate every 7-30 days
- **Signing Keys**: Rotate every 30-90 days
- **Service Account Keys**: Rotate every 90 days

---

### Required Implementation

#### 1. Vault Lease Watcher Service

**Purpose**: Monitor and renew Vault leases automatically before expiration

**Implementation**: `internal/infrastructure/secrets/lease_watcher.go`

```go
package secrets

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/hashicorp/vault/api"
    "github.com/rs/zerolog"
)

const (
    renewalThreshold = 0.67 // Renew at 2/3 of TTL
    renewalInterval  = 30 * time.Second
    maxRetries       = 3
)

type LeaseWatcher struct {
    client        *api.Client
    logger        infrastructure.Logger
    leases        map[string]*LeaseInfo
    mu            sync.RWMutex
    ctx           context.Context
    cancel        context.CancelFunc
    renewalCallbacks map[string]RenewalCallback
}

type LeaseInfo struct {
    LeaseID       string
    TTL           time.Duration
    Renewable     bool
    CreatedAt     time.Time
    LastRenewedAt time.Time
    SecretPath    string
    Data          *api.Secret
}

type RenewalCallback func(ctx context.Context, newSecret *api.Secret) error

func NewLeaseWatcher(client *api.Client, logger infrastructure.Logger) *LeaseWatcher {
    ctx, cancel := context.WithCancel(context.Background())

    return &LeaseWatcher{
        client:           client,
        logger:           logger.With().Str("component", "lease_watcher").Logger(),
        leases:           make(map[string]*LeaseInfo),
        ctx:              ctx,
        cancel:           cancel,
        renewalCallbacks: make(map[string]RenewalCallback),
    }
}

func (lw *LeaseWatcher) Start() {
    lw.logger.Info().Msg("Starting Vault lease watcher")

    go lw.watchLoop()
}

func (lw *LeaseWatcher) Stop() {
    lw.logger.Info().Msg("Stopping Vault lease watcher")
    lw.cancel()
}

func (lw *LeaseWatcher) RegisterLease(leaseInfo *LeaseInfo, callback RenewalCallback) {
    lw.mu.Lock()
    defer lw.mu.Unlock()

    lw.leases[leaseInfo.LeaseID] = leaseInfo
    lw.renewalCallbacks[leaseInfo.LeaseID] = callback

    lw.logger.Info().
        Str("lease_id", leaseInfo.LeaseID).
        Str("secret_path", leaseInfo.SecretPath).
        Dur("ttl", leaseInfo.TTL).
        Msg("Registered lease for watching")
}

func (lw *LeaseWatcher) watchLoop() {
    ticker := time.NewTicker(renewalInterval)
    defer ticker.Stop()

    for {
        select {
        case <-lw.ctx.Done():
            return
        case <-ticker.C:
            lw.checkAndRenewLeases()
        }
    }
}

func (lw *LeaseWatcher) checkAndRenewLeases() {
    lw.mu.RLock()
    leases := make([]*LeaseInfo, 0, len(lw.leases))
    for _, lease := range lw.leases {
        leases = append(leases, lease)
    }
    lw.mu.RUnlock()

    for _, lease := range leases {
        if lw.shouldRenew(lease) {
            lw.renewLease(lease)
        }
    }
}

func (lw *LeaseWatcher) shouldRenew(lease *LeaseInfo) bool {
    if !lease.Renewable {
        return false
    }

    elapsed := time.Since(lease.LastRenewedAt)
    threshold := time.Duration(float64(lease.TTL) * renewalThreshold)

    return elapsed >= threshold
}

func (lw *LeaseWatcher) renewLease(lease *LeaseInfo) {
    lw.logger.Info().
        Str("lease_id", lease.LeaseID).
        Str("secret_path", lease.SecretPath).
        Msg("Attempting to renew lease")

    var lastErr error
    for attempt := 1; attempt <= maxRetries; attempt++ {
        secret, err := lw.client.Sys().Renew(lease.LeaseID, 0)
        if err != nil {
            lastErr = err
            lw.logger.Warn().
                Err(err).
                Str("lease_id", lease.LeaseID).
                Int("attempt", attempt).
                Msg("Failed to renew lease")

            time.Sleep(time.Duration(attempt) * time.Second)
            continue
        }

        // Update lease info
        lw.mu.Lock()
        lease.LastRenewedAt = time.Now()
        lease.TTL = time.Duration(secret.LeaseDuration) * time.Second
        lease.Data = secret
        lw.mu.Unlock()

        lw.logger.Info().
            Str("lease_id", lease.LeaseID).
            Dur("new_ttl", lease.TTL).
            Msg("Successfully renewed lease")

        // Call renewal callback if registered
        if callback, exists := lw.renewalCallbacks[lease.LeaseID]; exists {
            if err := callback(lw.ctx, secret); err != nil {
                lw.logger.Error().
                    Err(err).
                    Str("lease_id", lease.LeaseID).
                    Msg("Renewal callback failed")
            }
        }

        return
    }

    lw.logger.Error().
        Err(lastErr).
        Str("lease_id", lease.LeaseID).
        Str("secret_path", lease.SecretPath).
        Msg("Failed to renew lease after max retries")
}
```

---

#### 2. Database Credential Rotation

**Enable Vault Database Secrets Engine:**

```bash
# Configure Vault database secrets engine
vault secrets enable database

# Configure PostgreSQL connection
vault write database/config/web-analyzer-db \
    plugin_name=postgresql-database-plugin \
    allowed_roles="web-analyzer-api,web-analyzer-publisher,web-analyzer-subscriber" \
    connection_url="postgresql://{{username}}:{{password}}@postgres:5432/web_analyzer?sslmode=disable" \
    username="vault-admin" \
    password="vault-admin-password"

# Create role with 24-hour TTL
vault write database/roles/web-analyzer-api \
    db_name=web-analyzer-db \
    creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}'; \
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO \"{{name}}\";" \
    default_ttl="24h" \
    max_ttl="48h"
```

**Implementation**: `internal/adapters/repository/postgres_rotator.go`

```go
package repository

import (
    "context"
    "database/sql"
    "fmt"
    "sync"

    "github.com/hashicorp/vault/api"
    "github.com/rs/zerolog"
)

type DatabaseRotator struct {
    db            *sql.DB
    vaultClient   *api.Client
    logger        infrastructure.Logger
    currentCreds  *DatabaseCredentials
    mu            sync.RWMutex
    roleName      string
}

type DatabaseCredentials struct {
    Username  string
    Password  string
    LeaseID   string
    TTL       int
}

func NewDatabaseRotator(
    db *sql.DB,
    vaultClient *api.Client,
    roleName string,
    logger infrastructure.Logger,
) *DatabaseRotator {
    return &DatabaseRotator{
        db:          db,
        vaultClient: vaultClient,
        logger:      logger.With().Str("component", "db_rotator").Logger(),
        roleName:    roleName,
    }
}

func (dr *DatabaseRotator) RotateCredentials(ctx context.Context, newSecret *api.Secret) error {
    dr.logger.Info().Msg("Rotating database credentials")

    // Extract new credentials
    username, ok := newSecret.Data["username"].(string)
    if !ok {
        return fmt.Errorf("failed to extract username from secret")
    }

    password, ok := newSecret.Data["password"].(string)
    if !ok {
        return fmt.Errorf("failed to extract password from secret")
    }

    newCreds := &DatabaseCredentials{
        Username: username,
        Password: password,
        LeaseID:  newSecret.LeaseID,
        TTL:      newSecret.LeaseDuration,
    }

    // Test new credentials
    testDB, err := sql.Open("postgres", fmt.Sprintf(
        "host=postgres port=5432 user=%s password=%s dbname=web_analyzer sslmode=disable",
        newCreds.Username, newCreds.Password,
    ))
    if err != nil {
        return fmt.Errorf("failed to open test connection: %w", err)
    }
    defer testDB.Close()

    if err := testDB.PingContext(ctx); err != nil {
        return fmt.Errorf("failed to ping with new credentials: %w", err)
    }

    // Update current credentials
    dr.mu.Lock()
    oldCreds := dr.currentCreds
    dr.currentCreds = newCreds
    dr.mu.Unlock()

    // Revoke old credentials if they exist
    if oldCreds != nil && oldCreds.LeaseID != "" {
        if err := dr.vaultClient.Sys().Revoke(oldCreds.LeaseID); err != nil {
            dr.logger.Warn().
                Err(err).
                Str("old_lease_id", oldCreds.LeaseID).
                Msg("Failed to revoke old credentials")
        }
    }

    dr.logger.Info().
        Str("username", newCreds.Username).
        Str("lease_id", newCreds.LeaseID).
        Int("ttl", newCreds.TTL).
        Msg("Database credentials rotated successfully")

    return nil
}

func (dr *DatabaseRotator) GetCurrentCredentials() *DatabaseCredentials {
    dr.mu.RLock()
    defer dr.mu.RUnlock()

    return dr.currentCreds
}
```

---

#### 3. RabbitMQ Credential Rotation

**Enable Vault RabbitMQ Secrets Engine:**

```bash
# Enable RabbitMQ secrets engine
vault secrets enable rabbitmq

# Configure RabbitMQ connection
vault write rabbitmq/config/connection \
    connection_uri="http://rabbitmq:15672" \
    username="admin" \
    password="admin"

# Create role with 24-hour TTL
vault write rabbitmq/roles/web-analyzer \
    vhosts='{"/":{"write": ".*", "read": ".*"}}' \
    default_ttl="24h" \
    max_ttl="48h"
```

**Implementation**: Similar pattern to database rotator with AMQP connection pool updates.

---

#### 4. PASETO Signing Key Rotation

**Implementation**: `internal/infrastructure/auth/key_rotator.go`

```go
package auth

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/rs/zerolog"
    "aidanwoods.dev/go-paseto"
)

const (
    keyRotationInterval = 30 * 24 * time.Hour // 30 days
    keyGracePeriod     = 7 * 24 * time.Hour   // 7 days overlap
)

type KeyRotator struct {
    currentKey  *KeyVersion
    previousKey *KeyVersion
    mu          sync.RWMutex
    logger      infrastructure.Logger
    vaultPath   string
}

type KeyVersion struct {
    Version    int
    PrivateKey paseto.V4AsymmetricSecretKey
    PublicKey  paseto.V4AsymmetricPublicKey
    CreatedAt  time.Time
    ExpiresAt  time.Time
}

func (kr *KeyRotator) RotateKey(ctx context.Context) error {
    kr.logger.Info().Msg("Rotating PASETO signing key")

    // Generate new key pair
    newPrivateKey := paseto.NewV4AsymmetricSecretKey()
    newPublicKey := newPrivateKey.Public()

    newVersion := &KeyVersion{
        Version:    kr.getCurrentVersion() + 1,
        PrivateKey: newPrivateKey,
        PublicKey:  newPublicKey,
        CreatedAt:  time.Now(),
        ExpiresAt:  time.Now().Add(keyRotationInterval + keyGracePeriod),
    }

    // Store new key in Vault
    if err := kr.storeKeyInVault(ctx, newVersion); err != nil {
        return fmt.Errorf("failed to store new key in Vault: %w", err)
    }

    // Update current and previous keys
    kr.mu.Lock()
    kr.previousKey = kr.currentKey
    kr.currentKey = newVersion
    kr.mu.Unlock()

    kr.logger.Info().
        Int("new_version", newVersion.Version).
        Time("expires_at", newVersion.ExpiresAt).
        Msg("PASETO signing key rotated successfully")

    return nil
}

func (kr *KeyRotator) SignToken(token *paseto.Token) (string, error) {
    kr.mu.RLock()
    defer kr.mu.RUnlock()

    if kr.currentKey == nil {
        return "", fmt.Errorf("no current signing key available")
    }

    // Add key version to token footer
    token.SetFooter([]byte(fmt.Sprintf(`{"kid":"%d"}`, kr.currentKey.Version)))

    return token.V4Sign(kr.currentKey.PrivateKey, nil), nil
}

func (kr *KeyRotator) VerifyToken(tokenString string) (*paseto.Token, error) {
    parser := paseto.NewParser()

    // Try current key first
    kr.mu.RLock()
    currentKey := kr.currentKey
    previousKey := kr.previousKey
    kr.mu.RUnlock()

    if currentKey != nil {
        if token, err := parser.ParseV4Public(currentKey.PublicKey, tokenString, nil); err == nil {
            return token, nil
        }
    }

    // Fall back to previous key (grace period)
    if previousKey != nil && time.Now().Before(previousKey.ExpiresAt) {
        if token, err := parser.ParseV4Public(previousKey.PublicKey, tokenString, nil); err == nil {
            return token, nil
        }
    }

    return nil, fmt.Errorf("token verification failed with all available keys")
}

func (kr *KeyRotator) getCurrentVersion() int {
    kr.mu.RLock()
    defer kr.mu.RUnlock()

    if kr.currentKey == nil {
        return 0
    }

    return kr.currentKey.Version
}

func (kr *KeyRotator) storeKeyInVault(ctx context.Context, keyVersion *KeyVersion) error {
    // Implementation to store key in Vault transit engine or KV v2
    return nil
}
```

---

#### 5. Service Integration

**Runtime Integration**: `internal/runtime/dispatcher.go`

```go
func (d *Dispatcher) initializeSecretsManagement(ctx context.Context) error {
    // Initialize lease watcher
    leaseWatcher := secrets.NewLeaseWatcher(d.vaultClient, d.logger)
    leaseWatcher.Start()
    d.leaseWatcher = leaseWatcher

    // Fetch initial database credentials
    dbSecret, err := d.vaultClient.Logical().Read("database/creds/web-analyzer-api")
    if err != nil {
        return fmt.Errorf("failed to read database credentials: %w", err)
    }

    // Register database lease
    dbLeaseInfo := &secrets.LeaseInfo{
        LeaseID:       dbSecret.LeaseID,
        TTL:           time.Duration(dbSecret.LeaseDuration) * time.Second,
        Renewable:     dbSecret.Renewable,
        CreatedAt:     time.Now(),
        LastRenewedAt: time.Now(),
        SecretPath:    "database/creds/web-analyzer-api",
        Data:          dbSecret,
    }

    dbRotator := repository.NewDatabaseRotator(
        d.db,
        d.vaultClient,
        "web-analyzer-api",
        d.logger,
    )

    leaseWatcher.RegisterLease(dbLeaseInfo, dbRotator.RotateCredentials)

    // Initialize key rotator for PASETO
    keyRotator := auth.NewKeyRotator(d.vaultClient, d.logger, "secret/paseto-keys")
    if err := keyRotator.LoadCurrentKey(ctx); err != nil {
        return fmt.Errorf("failed to load PASETO key: %w", err)
    }

    // Schedule periodic key rotation
    go d.scheduleKeyRotation(ctx, keyRotator)

    return nil
}

func (d *Dispatcher) scheduleKeyRotation(ctx context.Context, keyRotator *auth.KeyRotator) {
    ticker := time.NewTicker(30 * 24 * time.Hour) // 30 days
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := keyRotator.RotateKey(ctx); err != nil {
                d.logger.Error().Err(err).Msg("Failed to rotate PASETO key")
            }
        }
    }
}
```

---

### Configuration

**Vault Policy**: `deployments/vault/policies/web-analyzer.hcl`

```hcl
# Database credentials
path "database/creds/web-analyzer-*" {
  capabilities = ["read"]
}

# RabbitMQ credentials
path "rabbitmq/creds/web-analyzer" {
  capabilities = ["read"]
}

# PASETO key storage
path "secret/data/paseto-keys/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

# Lease renewal
path "sys/leases/renew" {
  capabilities = ["update"]
}

# Lease revocation
path "sys/leases/revoke" {
  capabilities = ["update"]
}
```

**Application Configuration**: `config/settings.yaml`

```yaml
vault:
  address: https://vault.web-analyzer.dev
  token: ${VAULT_TOKEN}
  lease_renewal:
    enabled: true
    renewal_threshold: 0.67  # Renew at 2/3 of TTL
    check_interval: 30s
    max_retries: 3

  database:
    role: web-analyzer-api
    rotation_enabled: true

  rabbitmq:
    role: web-analyzer
    rotation_enabled: true

  paseto_keys:
    rotation_interval: 720h  # 30 days
    grace_period: 168h       # 7 days
```

---

### Implementation Checklist

#### Phase 1: Foundation (Week 1-2)
- [ ] Implement `LeaseWatcher` service for monitoring Vault leases
- [ ] Add lease registration and renewal logic
- [ ] Create renewal callback mechanism
- [ ] Add comprehensive logging and error handling
- [ ] Write unit tests for lease watcher

#### Phase 2: Database Credential Rotation (Week 3-4)
- [ ] Configure Vault database secrets engine
- [ ] Implement `DatabaseRotator` for PostgreSQL
- [ ] Add connection pool credential update logic
- [ ] Test credential rotation without downtime
- [ ] Add integration tests for database rotation
- [ ] Document database rotation procedures

#### Phase 3: RabbitMQ Credential Rotation (Week 5-6)
- [ ] Configure Vault RabbitMQ secrets engine
- [ ] Implement RabbitMQ credential rotator
- [ ] Update AMQP connection management
- [ ] Test message queue credential rotation
- [ ] Add integration tests for RabbitMQ rotation

#### Phase 4: PASETO Key Rotation (Week 7-8)
- [ ] Implement `KeyRotator` for PASETO keys
- [ ] Add key versioning support
- [ ] Implement graceful key rollover with overlap period
- [ ] Update token verification to support multiple keys
- [ ] Test zero-downtime key rotation
- [ ] Document key rotation procedures

#### Phase 5: Service Integration (Week 9-10)
- [ ] Integrate lease watcher in all three services
- [ ] Update runtime initialization for secret rotation
- [ ] Add graceful shutdown for lease watcher
- [ ] Implement health checks for secret validity
- [ ] Add metrics for rotation events and failures
- [ ] Create alerts for rotation failures

#### Phase 6: Monitoring & Observability (Week 11-12)
- [ ] Add Prometheus metrics for lease renewals
- [ ] Track rotation success/failure rates
- [ ] Monitor time until next rotation
- [ ] Create Grafana dashboards for secret health
- [ ] Set up alerts for lease expiration warnings
- [ ] Add distributed tracing for rotation events

#### Phase 7: Documentation & Runbooks (Week 13-14)
- [ ] Document secret rotation architecture
- [ ] Create runbooks for rotation failures
- [ ] Document manual rotation procedures
- [ ] Create disaster recovery procedures
- [ ] Document Vault policy requirements
- [ ] Create operational playbooks

---

### Testing Strategy

#### Unit Tests
```go
func TestLeaseWatcher_ShouldRenew(t *testing.T) {
    t.Parallel()

    cases := []struct {
        name     string
        lease    *LeaseInfo
        expected bool
    }{
        {
            name: "should renew at 2/3 TTL",
            lease: &LeaseInfo{
                TTL:           90 * time.Second,
                LastRenewedAt: time.Now().Add(-60 * time.Second),
                Renewable:     true,
            },
            expected: true,
        },
        {
            name: "should not renew non-renewable lease",
            lease: &LeaseInfo{
                TTL:           90 * time.Second,
                LastRenewedAt: time.Now().Add(-60 * time.Second),
                Renewable:     false,
            },
            expected: false,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()

            lw := &LeaseWatcher{}
            result := lw.shouldRenew(tc.lease)

            assert.Equal(t, tc.expected, result)
        })
    }
}
```

#### Integration Tests
```go
func TestDatabaseRotation_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup Vault test server
    vaultClient := setupVaultTestServer(t)

    // Configure database secrets engine
    configureDatabaseEngine(t, vaultClient)

    // Create initial credentials
    secret, err := vaultClient.Logical().Read("database/creds/test-role")
    require.NoError(t, err)

    // Test rotation
    rotator := NewDatabaseRotator(db, vaultClient, "test-role", logger)
    err = rotator.RotateCredentials(context.Background(), secret)
    require.NoError(t, err)

    // Verify new credentials work
    newCreds := rotator.GetCurrentCredentials()
    testDB := connectWithCredentials(t, newCreds)
    require.NoError(t, testDB.Ping())
}
```

---

### Monitoring & Alerts

#### Prometheus Metrics
```go
var (
    leaseRenewalTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "vault_lease_renewal_total",
            Help: "Total number of Vault lease renewals",
        },
        []string{"status", "secret_path"},
    )

    leaseRenewalDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "vault_lease_renewal_duration_seconds",
            Help:    "Duration of Vault lease renewal operations",
            Buckets: prometheus.DefBuckets,
        },
        []string{"secret_path"},
    )

    leaseTimeUntilExpiry = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "vault_lease_time_until_expiry_seconds",
            Help: "Time until Vault lease expiry",
        },
        []string{"secret_path", "lease_id"},
    )

    credentialRotationTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "credential_rotation_total",
            Help: "Total number of credential rotations",
        },
        []string{"type", "status"},
    )
)
```

#### Grafana Dashboard Queries
```promql
# Lease renewal success rate
rate(vault_lease_renewal_total{status="success"}[5m])
/
rate(vault_lease_renewal_total[5m])

# Time until lease expiry
vault_lease_time_until_expiry_seconds

# Credential rotation failures
rate(credential_rotation_total{status="failure"}[1h])

# Average rotation duration
histogram_quantile(0.95,
  rate(vault_lease_renewal_duration_seconds_bucket[5m])
)
```

#### Alert Rules
```yaml
groups:
  - name: secret_rotation
    rules:
      - alert: LeaseExpiringWithoutRenewal
        expr: vault_lease_time_until_expiry_seconds < 300
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Vault lease expiring soon without renewal"
          description: "Lease {{ $labels.lease_id }} expires in {{ $value }}s"

      - alert: CredentialRotationFailure
        expr: rate(credential_rotation_total{status="failure"}[1h]) > 0
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Credential rotation failures detected"
          description: "{{ $value }} rotation failures in the last hour"

      - alert: LeaseRenewalFailureRate
        expr: |
          rate(vault_lease_renewal_total{status="failure"}[5m])
          /
          rate(vault_lease_renewal_total[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High lease renewal failure rate"
          description: "{{ $value | humanizePercentage }} of renewals failing"
```

---

### Benefits

**Security:**
- **Reduced Blast Radius**: Compromised credentials expire automatically
- **Compliance**: Meets regulatory rotation requirements (PCI DSS, SOC 2, HIPAA)
- **Defense in Depth**: Multiple layers of credential protection
- **Audit Trail**: Complete history of credential rotations
- **Zero-Trust**: Continuous credential validation and rotation

**Reliability:**
- **No Service Disruption**: Zero-downtime credential rotation
- **Automatic Renewal**: No manual intervention required
- **Graceful Degradation**: Handles rotation failures with retries
- **Connection Pool Updates**: Seamless database connection updates

**Operations:**
- **Automated Management**: No manual credential rotation procedures
- **Observability**: Complete visibility into secret lifecycle
- **Alerting**: Proactive notifications for rotation issues
- **Disaster Recovery**: Documented procedures for rotation failures

**Cost Savings:**
- **Reduced Manual Effort**: Eliminates manual rotation procedures
- **Faster Incident Response**: Automatic credential revocation
- **Lower Risk**: Reduced exposure window for compromised secrets

---

## 3. API Client Generation from OpenAPI Specification 🟡 **HIGH**

### Current State
- OpenAPI 3.0.3 specification exists in `docs/openapi-spec/`
- No automated client generation
- Frontend must manually write API calls
- No type safety between frontend and backend
- No SDK for external integrations

### Overview

Implement automated API client generation from the OpenAPI specification to provide type-safe SDKs for the frontend (TypeScript/JavaScript) and other potential consumers (Go, Python, etc.). This ensures consistency between backend API contracts and client implementations.

### Benefits

- **Type Safety**: Compile-time type checking for API requests/responses
- **Developer Experience**: Auto-complete and IntelliSense in IDEs
- **Consistency**: Single source of truth (OpenAPI spec)
- **Reduced Errors**: Eliminates manual API call implementation
- **Documentation**: Generated clients include inline documentation
- **React Query Integration**: Generated hooks for React applications
- **Multi-Language Support**: Generate clients for various languages

---

### 1. TypeScript/JavaScript Client Generation (Frontend)

#### Option A: Using OpenAPI Generator (Basic Client)

**Configuration File**: `build/oapi/client-codegen.yaml`

```yaml
generatorName: typescript-axios
inputSpec: docs/openapi-spec/openapi.yaml
outputDir: web/src/generated/api-client
additionalProperties:
  npmName: "@web-analyzer/api-client"
  npmVersion: "1.0.0"
  supportsES6: true
  useSingleRequestParameter: true
  withSeparateModelsAndApi: true
  apiPackage: api
  modelPackage: models
globalProperties:
  models: ""
  apis: ""
  supportingFiles: index.ts
```

**GitHub Workflow Addition**: `.github/workflows/generate-clients.yml`

```yaml
name: Generate API Clients

on:
  push:
    branches: [main, develop]
    paths:
      - 'docs/openapi-spec/**'
  pull_request:
    paths:
      - 'docs/openapi-spec/**'
  workflow_dispatch:

jobs:
  generate-typescript-client:
    name: Generate TypeScript Client
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install OpenAPI Generator
        run: npm install -g @openapitools/openapi-generator-cli

      - name: Validate OpenAPI spec
        run: |
          openapi-generator-cli validate -i docs/openapi-spec/openapi.yaml

      - name: Generate TypeScript client
        run: |
          openapi-generator-cli generate \
            -i docs/openapi-spec/openapi.yaml \
            -g typescript-axios \
            -o web/src/generated/api-client \
            --additional-properties=npmName=@web-analyzer/api-client,supportsES6=true,useSingleRequestParameter=true,withSeparateModelsAndApi=true

      - name: Format generated code
        run: |
          cd web/src/generated/api-client
          npm install
          npx prettier --write "**/*.ts"

      - name: Create Pull Request
        uses: peter-evans/create-pull-request@v6
        with:
          commit-message: "chore: regenerate TypeScript API client from OpenAPI spec"
          title: "chore: Update TypeScript API Client"
          body: |
            Auto-generated TypeScript API client from OpenAPI specification changes.

            **Changes:**
            - Updated API client based on OpenAPI spec
            - Regenerated models and API interfaces

            Please review the changes and merge if they look correct.
          branch: auto/update-api-client
          delete-branch: true
          labels: |
            auto-generated
            api-client
            dependencies
```

#### Option B: Using Orval (React Query Hooks)

**Why Orval?**
- Generates React Query/SWR hooks automatically
- Better TypeScript integration
- Customizable templates
- Zod schema validation support
- Mock data generation

**Configuration File**: `orval.config.ts`

```typescript
import { defineConfig } from 'orval';

export default defineConfig({
  webAnalyzer: {
    input: {
      target: './docs/openapi-spec/openapi.yaml',
    },
    output: {
      mode: 'tags-split',
      target: './web/src/generated/api-client/endpoints',
      schemas: './web/src/generated/api-client/models',
      client: 'react-query',
      mock: true,
      prettier: true,
      override: {
        mutator: {
          path: './web/src/api/custom-instance.ts',
          name: 'customInstance',
        },
        query: {
          useQuery: true,
          useMutation: true,
          signal: true,
        },
      },
    },
    hooks: {
      afterAllFilesWrite: 'prettier --write',
    },
  },
});
```

**Custom Axios Instance**: `web/src/api/custom-instance.ts`

```typescript
import Axios, { AxiosRequestConfig, AxiosError } from 'axios';

// Base API configuration
export const AXIOS_INSTANCE = Axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'https://api.web-analyzer.dev/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor for auth token
AXIOS_INSTANCE.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }

    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor for error handling
AXIOS_INSTANCE.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      // Handle unauthorized - redirect to login
      window.location.href = '/login';
    }

    return Promise.reject(error);
  }
);

// Custom instance for Orval
export const customInstance = <T>(config: AxiosRequestConfig): Promise<T> => {
  const source = Axios.CancelToken.source();

  const promise = AXIOS_INSTANCE({
    ...config,
    cancelToken: source.token,
  }).then(({ data }) => data);

  // @ts-ignore
  promise.cancel = () => {
    source.cancel('Query was cancelled');
  };

  return promise;
};

export default customInstance;
```

**Package.json Scripts**:

```json
{
  "scripts": {
    "generate:api": "orval --config orval.config.ts",
    "generate:api:watch": "orval --config orval.config.ts --watch"
  },
  "devDependencies": {
    "orval": "^6.29.1",
    "@tanstack/react-query": "^5.17.0",
    "axios": "^1.6.5",
    "zod": "^3.22.4"
  }
}
```

**Generated Usage Example**:

```typescript
// web/src/components/AnalysisForm.tsx
import { usePostAnalyze } from '@/generated/api-client/endpoints/analysis';
import { AnalyzeRequest } from '@/generated/api-client/models';

export function AnalysisForm() {
  const { mutate: analyzeUrl, isPending, isError, data } = usePostAnalyze();

  const handleSubmit = (url: string) => {
    const request: AnalyzeRequest = { url };

    analyzeUrl(
      { data: request },
      {
        onSuccess: (response) => {
          console.log('Analysis started:', response.analysisId);
        },
        onError: (error) => {
          console.error('Analysis failed:', error);
        },
      }
    );
  };

  return (
    <form onSubmit={(e) => {
      e.preventDefault();
      const url = new FormData(e.currentTarget).get('url') as string;
      handleSubmit(url);
    }}>
      <input name="url" type="url" placeholder="Enter URL to analyze" />
      <button type="submit" disabled={isPending}>
        {isPending ? 'Analyzing...' : 'Analyze'}
      </button>
      {isError && <p className="error">Failed to start analysis</p>}
    </form>
  );
}

// Using the generated query hook
import { useGetAnalysis } from '@/generated/api-client/endpoints/analysis';

export function AnalysisResults({ analysisId }: { analysisId: string }) {
  const { data, isLoading, error, refetch } = useGetAnalysis(analysisId, {
    query: {
      refetchInterval: 5000, // Poll every 5 seconds
      enabled: !!analysisId,
    },
  });

  if (isLoading) return <div>Loading analysis...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div>
      <h2>Analysis Results</h2>
      <p>Status: {data?.status}</p>
      <p>URL: {data?.url}</p>
      {data?.results && (
        <div>
          <h3>Results</h3>
          <pre>{JSON.stringify(data.results, null, 2)}</pre>
        </div>
      )}
      <button onClick={() => refetch()}>Refresh</button>
    </div>
  );
}
```

---

### 2. Go Client Generation (For Testing/Integration)

**Configuration File**: `build/oapi/go-client-codegen.yaml`

```yaml
package: client
generate:
  client: true
  models: true
  embedded-spec: true
output: pkg/client/api_client.go
output-options:
  skip-prune: true
```

**Makefile Integration**:

```makefile
.PHONY: generate-go-client
generate-go-client: ## Generate Go API client
	@echo "🔧 Generating Go API client from OpenAPI spec..."
	@oapi-codegen -config build/oapi/go-client-codegen.yaml docs/openapi-spec/openapi.yaml

.PHONY: generate-clients
generate-clients: generate-api generate-go-client generate-ts-client ## Generate all API clients
	@echo "✅ All API clients generated successfully"
```

**Usage Example**:

```go
// Example integration test using generated Go client
package itest

import (
	"context"
	"testing"

	"github.com/architeacher/svc-web-analyzer/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalysisWorkflow(t *testing.T) {
	t.Parallel()

	// Create API client
	apiClient, err := client.NewClientWithResponses(
		"http://localhost:8080/v1",
		client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+testToken)
			return nil
		}),
	)
	require.NoError(t, err)

	// Submit analysis request
	analyzeReq := client.AnalyzeJSONRequestBody{
		Url: "https://example.com",
	}

	resp, err := apiClient.PostAnalyzeWithResponse(context.Background(), analyzeReq)
	require.NoError(t, err)
	assert.Equal(t, 202, resp.StatusCode())

	analysisID := resp.JSON202.AnalysisId

	// Poll for completion
	for i := 0; i < 30; i++ {
		statusResp, err := apiClient.GetAnalysisByIdWithResponse(context.Background(), analysisID)
		require.NoError(t, err)

		if statusResp.JSON200.Status == "completed" {
			assert.NotNil(t, statusResp.JSON200.Results)
			return
		}

		time.Sleep(time.Second)
	}

	t.Fatal("Analysis did not complete in time")
}
```

---

### 3. Python Client Generation (Optional)

**For external integrations or Python-based tools:**

```bash
# Generate Python client
openapi-generator-cli generate \
  -i docs/openapi-spec/openapi.yaml \
  -g python \
  -o clients/python \
  --additional-properties=packageName=web_analyzer_client,projectName=web-analyzer-client

# Publish to PyPI
cd clients/python
python setup.py sdist bdist_wheel
twine upload dist/*
```

**Usage Example**:

```python
from web_analyzer_client import ApiClient, Configuration, AnalysisApi
from web_analyzer_client.models import AnalyzeRequest

# Configure client
config = Configuration(
    host="https://api.web-analyzer.dev/v1",
    access_token="your-api-token"
)

# Create API instance
with ApiClient(config) as client:
    api = AnalysisApi(client)

    # Submit analysis
    request = AnalyzeRequest(url="https://example.com")
    response = api.post_analyze(request)

    print(f"Analysis ID: {response.analysis_id}")

    # Get results
    analysis = api.get_analysis_by_id(response.analysis_id)
    print(f"Status: {analysis.status}")
    print(f"Results: {analysis.results}")
```

---

### 4. OpenAPI Spec Validation in CI/CD

Add to `.github/workflows/ci.yml`:

```yaml
  validate-openapi:
    name: Validate OpenAPI Specification
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Install Spectral (OpenAPI Linter)
        run: npm install -g @stoplight/spectral-cli

      - name: Validate OpenAPI spec
        run: |
          spectral lint docs/openapi-spec/openapi.yaml \
            --ruleset .spectral.yaml \
            --fail-severity warn

      - name: Validate with Redocly
        run: |
          npx @redocly/cli lint docs/openapi-spec/openapi.yaml

      - name: Check for breaking changes
        uses: oasdiff/oasdiff-action@main
        with:
          base: 'https://api.web-analyzer.dev/v1/openapi.yaml'
          revision: 'docs/openapi-spec/openapi.yaml'
          fail-on-diff: true
```

**Spectral Configuration**: `.spectral.yaml`

```yaml
extends: ["spectral:oas", "spectral:asyncapi"]
rules:
  operation-description: error
  operation-operationId: error
  operation-tags: warn
  operation-tag-defined: error
  info-contact: warn
  info-description: error
  info-license: warn
  tag-description: error
  no-$ref-siblings: error
  typed-enum: warn
  oas3-api-servers: error
  oas3-examples-value-or-externalValue: error
  oas3-server-trailing-slash: error
  oas3-valid-media-example: error
  oas3-valid-schema-example: error
  openapi-tags-alphabetical: off
  openapi-tags-uniqueness: error
```

---

### 5. Mock Server Generation (For Frontend Development)

**Using Prism for Mock API Server:**

```bash
# Install Prism
npm install -g @stoplight/prism-cli

# Start mock server
prism mock docs/openapi-spec/openapi.yaml --port 8081

# Or with Docker
docker run --rm -p 8081:4010 \
  -v $(pwd)/docs/openapi-spec:/tmp \
  stoplight/prism:latest \
  mock -h 0.0.0.0 /tmp/openapi.yaml
```

**Package.json Script**:

```json
{
  "scripts": {
    "mock:api": "prism mock docs/openapi-spec/openapi.yaml --port 8081",
    "dev:with-mock": "concurrently \"npm run mock:api\" \"npm run dev\""
  }
}
```

**Frontend Environment Configuration**:

```typescript
// web/src/config/api.ts
export const API_CONFIG = {
  baseURL: import.meta.env.VITE_USE_MOCK_API === 'true'
    ? 'http://localhost:8081'  // Prism mock server
    : import.meta.env.VITE_API_BASE_URL || 'https://api.web-analyzer.dev/v1',
};
```

---

### 6. Client Library Publishing (npm Package)

**Package Configuration**: `web/src/generated/api-client/package.json`

```json
{
  "name": "@web-analyzer/api-client",
  "version": "1.0.0",
  "description": "TypeScript client for Web Analyzer API",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "files": [
    "dist"
  ],
  "scripts": {
    "build": "tsc",
    "prepublishOnly": "npm run build"
  },
  "keywords": [
    "web-analyzer",
    "api-client",
    "typescript",
    "axios"
  ],
  "author": "Web Analyzer Team",
  "license": "MIT",
  "peerDependencies": {
    "axios": "^1.6.0",
    "@tanstack/react-query": "^5.0.0"
  },
  "devDependencies": {
    "typescript": "^5.3.3"
  }
}
```

**GitHub Workflow for Publishing**: `.github/workflows/publish-clients.yml`

```yaml
name: Publish API Clients

on:
  release:
    types: [published]

jobs:
  publish-npm:
    name: Publish to npm
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          registry-url: 'https://registry.npmjs.org'

      - name: Generate TypeScript client
        run: npm run generate:api

      - name: Build client package
        working-directory: web/src/generated/api-client
        run: |
          npm install
          npm run build

      - name: Publish to npm
        working-directory: web/src/generated/api-client
        run: npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

---

### 7. Implementation Checklist

#### OpenAPI Specification
- [ ] Validate OpenAPI spec is complete and accurate
- [ ] Add examples for all request/response bodies
- [ ] Document all error responses
- [ ] Add operation IDs for all endpoints
- [ ] Include tags for grouping operations
- [ ] Add security schemes documentation

#### TypeScript/JavaScript Client
- [ ] Install Orval or OpenAPI Generator
- [ ] Create Orval configuration file
- [ ] Set up custom Axios instance with auth
- [ ] Generate initial TypeScript client
- [ ] Add client generation to CI/CD pipeline
- [ ] Configure auto-PR creation on spec changes
- [ ] Add generated code to .gitignore (or commit if preferred)
- [ ] Set up React Query integration
- [ ] Create usage documentation

#### Go Client (Optional)
- [ ] Configure oapi-codegen for client generation
- [ ] Add Go client generation to Makefile
- [ ] Create integration tests using Go client
- [ ] Document Go client usage

#### Validation & Quality
- [ ] Install Spectral for OpenAPI linting
- [ ] Create .spectral.yaml ruleset
- [ ] Add OpenAPI validation to CI/CD
- [ ] Set up breaking change detection
- [ ] Configure Redocly linting

#### Mock Server
- [ ] Install Prism mock server
- [ ] Add mock server npm scripts
- [ ] Configure frontend to use mock API in dev mode
- [ ] Document mock server usage

#### Publishing (Optional)
- [ ] Set up npm organization (@web-analyzer)
- [ ] Configure package.json for npm publishing
- [ ] Create publish workflow
- [ ] Set up NPM_TOKEN secret
- [ ] Version client alongside API releases

---

### 8. Best Practices

#### Keep Spec in Sync
```yaml
# Pre-commit hook to validate spec changes
# .husky/pre-commit
#!/bin/sh
if git diff --cached --name-only | grep -q "docs/openapi-spec"; then
  echo "OpenAPI spec changed, validating..."
  spectral lint docs/openapi-spec/openapi.yaml

  echo "Regenerating clients..."
  npm run generate:api

  git add web/src/generated/api-client
fi
```

#### Semantic Versioning for Clients
- Major version: Breaking API changes
- Minor version: New endpoints or fields (backward compatible)
- Patch version: Bug fixes, documentation updates

#### Generated Code Management
- **Option A**: Commit generated code to repository
  - Pros: No build step needed, easier CI/CD
  - Cons: Larger repository, merge conflicts

- **Option B**: Generate on-demand
  - Pros: Smaller repository, no merge conflicts
  - Cons: Build step required, potential for drift

**Recommendation**: Commit generated code for stability, regenerate in CI to verify.

---

### 9. Benefits

**Developer Experience:**
- IntelliSense and auto-complete in IDEs
- Compile-time type checking
- Reduced manual coding errors
- Consistent API usage patterns

**Type Safety:**
- Frontend types match backend contracts
- Automatic validation of requests/responses
- Catch API mismatches early

**Maintenance:**
- Single source of truth (OpenAPI spec)
- Automated client updates on spec changes
- Breaking change detection
- Version compatibility tracking

**Integration:**
- Easy external integrations with generated clients
- Multiple language support
- Mock server for frontend development
- Test automation with typed clients

---

## 4. Performance Testing with k6 🟡 **HIGH**

### Current State
- No load testing infrastructure
- No performance benchmarks or SLOs defined
- No automated performance regression detection
- Manual testing only, inconsistent results
- No performance metrics in CI/CD pipeline

### Overview

Implement comprehensive performance testing using [k6](https://k6.io/), a modern load testing tool designed for testing the performance of APIs, microservices, and websites. k6 provides developer-friendly scripting in JavaScript, excellent performance for generating high load, and powerful metrics collection.

### Performance SLOs (Service Level Objectives)

Specific performance targets based on production requirements:

```yaml
# Performance SLOs
service_level_objectives:
  api_endpoints:
    POST /v1/analyze:
      p95_latency: 200ms
      p99_latency: 500ms
      throughput: 1000 requests/second
      error_rate: 0.1%
      success_rate: 99.9%

    GET /v1/analysis/{id}:
      p95_latency: 100ms
      p99_latency: 200ms
      throughput: 2000 requests/second
      error_rate: 0.05%
      success_rate: 99.95%

    GET /v1/analysis/{id}/events (SSE):
      connection_time: 50ms
      message_latency: 100ms
      connection_stability: 99.9%
      max_concurrent_connections: 10000

  database:
    query_latency:
      simple_select: 10ms
      complex_join: 50ms
      insert: 20ms
      update: 30ms

  message_queue:
    publish_latency: 10ms
    consume_latency: 50ms
    throughput: 5000 messages/second

  system_resources:
    cpu_usage: 70%
    memory_usage: 80%
    disk_io: 60%
    network_bandwidth: 80%
```

### Test Scenarios

#### 1. Load Testing - Baseline Performance

**Purpose**: Establish baseline performance under expected production load.

```javascript
// tests/performance/load-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const analysisLatency = new Trend('analysis_latency');
const requestCount = new Counter('requests');

// Test configuration
export const options = {
  stages: [
    { duration: '2m', target: 100 },   // Ramp up to 100 VUs
    { duration: '5m', target: 100 },   // Stay at 100 VUs
    { duration: '2m', target: 200 },   // Ramp to 200 VUs
    { duration: '5m', target: 200 },   // Stay at 200 VUs
    { duration: '2m', target: 500 },   // Ramp to 500 VUs
    { duration: '10m', target: 500 },  // Stay at 500 VUs (sustained load)
    { duration: '2m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<500'], // 95% < 200ms, 99% < 500ms
    http_req_failed: ['rate<0.01'],                // Error rate < 1%
    errors: ['rate<0.01'],
    requests: ['count>10000'],                     // Minimum request count
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://api.web-analyzer.dev/v1';

export default function () {
  const payload = JSON.stringify({
    url: `https://example.com/test-${__VU}-${__ITER}`,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${__ENV.API_TOKEN}`,
    },
  };

  // Submit analysis request
  const startTime = new Date();
  const response = http.post(`${BASE_URL}/analyze`, payload, params);
  const endTime = new Date();

  // Record custom metrics
  analysisLatency.add(endTime - startTime);
  requestCount.add(1);

  // Validate response
  const success = check(response, {
    'status is 202': (r) => r.status === 202,
    'has analysis_id': (r) => r.json('analysis_id') !== undefined,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  if (!success) {
    errorRate.add(1);
  }

  // Extract analysis ID for polling
  if (response.status === 202) {
    const analysisId = response.json('analysis_id');

    // Poll for completion (simplified - real implementation would use SSE)
    let completed = false;
    let attempts = 0;
    const maxAttempts = 10;

    while (!completed && attempts < maxAttempts) {
      sleep(2); // Wait 2 seconds between polls

      const statusResponse = http.get(
        `${BASE_URL}/analysis/${analysisId}`,
        params
      );

      check(statusResponse, {
        'status check succeeded': (r) => r.status === 200,
      });

      if (statusResponse.status === 200) {
        const status = statusResponse.json('status');
        completed = (status === 'completed' || status === 'failed');
      }

      attempts++;
    }
  }

  sleep(1); // Think time between iterations
}

// Teardown function
export function teardown(data) {
  console.log('Load test completed');
  console.log(`Total requests: ${requestCount.value}`);
}
```

#### 2. Stress Testing - Breaking Point Identification

**Purpose**: Identify system limits and failure modes.

```javascript
// tests/performance/stress-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 500 },    // Ramp to 500 VUs
    { duration: '5m', target: 500 },    // Stay at 500
    { duration: '2m', target: 1000 },   // Ramp to 1000
    { duration: '5m', target: 1000 },   // Stay at 1000
    { duration: '2m', target: 1500 },   // Ramp to 1500
    { duration: '5m', target: 1500 },   // Stay at 1500
    { duration: '2m', target: 2000 },   // Ramp to 2000 (stress point)
    { duration: '5m', target: 2000 },   // Stress load
    { duration: '5m', target: 0 },      // Recovery
  ],
  thresholds: {
    http_req_duration: ['p(99)<1000'],  // Relaxed threshold for stress
    http_req_failed: ['rate<0.05'],     // Allow 5% error rate
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://api.web-analyzer.dev/v1';

export default function () {
  const payload = JSON.stringify({
    url: `https://stress-test-${__VU}-${__ITER}.example.com`,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${__ENV.API_TOKEN}`,
    },
  };

  const response = http.post(`${BASE_URL}/analyze`, payload, params);

  check(response, {
    'status is 202 or 429': (r) => r.status === 202 || r.status === 429,
    'has valid response': (r) => r.body.length > 0,
  });

  sleep(0.5); // Minimal think time for stress
}
```

#### 3. Spike Testing - Sudden Traffic Surge Handling

**Purpose**: Validate auto-scaling and burst capacity.

```javascript
// tests/performance/spike-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 100 },     // Normal load
    { duration: '10s', target: 2000 },   // Sudden spike!
    { duration: '3m', target: 2000 },    // Sustain spike
    { duration: '10s', target: 100 },    // Drop back
    { duration: '2m', target: 100 },     // Recovery
    { duration: '10s', target: 0 },      // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.02'], // Allow 2% error during spike
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://api.web-analyzer.dev/v1';

export default function () {
  const payload = JSON.stringify({
    url: `https://spike-${__VU}-${__ITER}.example.com`,
  });

  const response = http.post(
    `${BASE_URL}/analyze`,
    payload,
    {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${__ENV.API_TOKEN}`,
      },
    }
  );

  check(response, {
    'handled gracefully': (r) => r.status === 202 || r.status === 429 || r.status === 503,
    'has retry-after on 429': (r) => r.status !== 429 || r.headers['Retry-After'] !== undefined,
  });

  sleep(1);
}
```

#### 4. Soak Testing - Long-Running Stability

**Purpose**: Detect memory leaks, resource exhaustion, and degradation over time.

```javascript
// tests/performance/soak-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const requestDuration = new Trend('request_duration_trend');
const errorCount = new Counter('error_count');

export const options = {
  stages: [
    { duration: '5m', target: 200 },     // Ramp to target
    { duration: '23h 50m', target: 200 }, // Soak for ~24 hours
    { duration: '5m', target: 0 },       // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<250', 'p(99)<600'],
    http_req_failed: ['rate<0.005'], // Very low error rate for soak
    request_duration_trend: ['p(95)<250'],
    error_count: ['count<1000'], // Max 1000 errors over 24h
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://api.web-analyzer.dev/v1';

export default function () {
  const start = new Date();

  const response = http.post(
    `${BASE_URL}/analyze`,
    JSON.stringify({
      url: `https://soak-test-${__VU}-${__ITER}.example.com`,
    }),
    {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${__ENV.API_TOKEN}`,
      },
    }
  );

  const duration = new Date() - start;
  requestDuration.add(duration);

  const success = check(response, {
    'status is 202': (r) => r.status === 202,
  });

  if (!success) {
    errorCount.add(1);
  }

  sleep(3); // Simulate realistic user behavior
}

// Custom summary to detect degradation
export function handleSummary(data) {
  const p95Duration = data.metrics.http_req_duration.values['p(95)'];
  const errorRate = data.metrics.http_req_failed.values.rate;

  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'soak-test-summary.json': JSON.stringify({
      timestamp: new Date().toISOString(),
      p95_latency_ms: p95Duration,
      error_rate: errorRate,
      degradation_detected: p95Duration > 250 || errorRate > 0.005,
    }),
  };
}
```

#### 5. SSE Connection Testing

**Purpose**: Validate Server-Sent Events connection handling and message delivery.

```javascript
// tests/performance/sse-test.js
import http from 'k6/http';
import { check } from 'k6';
import { WebSocket } from 'k6/ws';
import { Counter, Gauge } from 'k6/metrics';

const activeConnections = new Gauge('active_sse_connections');
const messagesReceived = new Counter('sse_messages_received');

export const options = {
  stages: [
    { duration: '2m', target: 1000 },   // Ramp to 1000 connections
    { duration: '10m', target: 1000 },  // Sustain
    { duration: '2m', target: 5000 },   // Ramp to 5000
    { duration: '10m', target: 5000 },  // Sustain high connection count
    { duration: '2m', target: 0 },      // Ramp down
  ],
  thresholds: {
    active_sse_connections: ['value<=10000'],
    sse_messages_received: ['count>1000'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'https://api.web-analyzer.dev/v1';

export default function () {
  // First, create an analysis to get events for
  const analysisResponse = http.post(
    `${BASE_URL}/analyze`,
    JSON.stringify({ url: 'https://example.com' }),
    {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${__ENV.API_TOKEN}`,
      },
    }
  );

  if (analysisResponse.status !== 202) {
    return;
  }

  const analysisId = analysisResponse.json('analysis_id');

  // Connect to SSE endpoint
  const response = http.get(
    `${BASE_URL}/analysis/${analysisId}/events`,
    {
      headers: {
        'Accept': 'text/event-stream',
        'Authorization': `Bearer ${__ENV.API_TOKEN}`,
      },
      responseType: 'text',
      timeout: '60s', // SSE connection timeout
    }
  );

  check(response, {
    'SSE connection established': (r) => r.status === 200,
    'content-type is text/event-stream': (r) =>
      r.headers['Content-Type'] === 'text/event-stream',
  });

  if (response.status === 200) {
    activeConnections.add(1);

    // Parse SSE messages (simplified)
    const messages = response.body.split('\n\n');
    messagesReceived.add(messages.length);

    activeConnections.add(-1);
  }
}
```

### CI/CD Integration

#### GitHub Actions Workflow

```yaml
# .github/workflows/performance-tests.yml
name: Performance Tests

on:
  pull_request:
    branches: [main]
    paths:
      - 'cmd/**'
      - 'internal/**'
      - 'tests/performance/**'
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
  workflow_dispatch:
    inputs:
      test_type:
        description: 'Type of performance test'
        required: true
        type: choice
        options:
          - load
          - stress
          - spike
          - soak

jobs:
  performance-test:
    name: Run k6 Performance Tests
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: web_analyzer_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

      rabbitmq:
        image: rabbitmq:3.12-management-alpine
        env:
          RABBITMQ_DEFAULT_USER: admin
          RABBITMQ_DEFAULT_PASS: admin
        ports:
          - 5672:5672

      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25.3'

      - name: Build application
        run: |
          make build

      - name: Start application services
        run: |
          ./bin/svc-web-analyzer &
          ./bin/publisher &
          ./bin/subscriber &

          # Wait for services to be ready
          timeout 60 bash -c 'until curl -f http://localhost:8080/v1/health; do sleep 2; done'

      - name: Install k6
        run: |
          sudo gpg -k
          sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
            --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
          echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | \
            sudo tee /etc/apt/sources.list.d/k6.list
          sudo apt-get update
          sudo apt-get install k6

      - name: Run load test
        if: github.event_name == 'pull_request' || github.event.inputs.test_type == 'load'
        env:
          BASE_URL: http://localhost:8080/v1
          API_TOKEN: ${{ secrets.TEST_API_TOKEN }}
        run: |
          k6 run --out json=load-test-results.json tests/performance/load-test.js

      - name: Run stress test
        if: github.event.inputs.test_type == 'stress'
        env:
          BASE_URL: http://localhost:8080/v1
          API_TOKEN: ${{ secrets.TEST_API_TOKEN }}
        run: |
          k6 run --out json=stress-test-results.json tests/performance/stress-test.js

      - name: Run spike test
        if: github.event.inputs.test_type == 'spike'
        env:
          BASE_URL: http://localhost:8080/v1
          API_TOKEN: ${{ secrets.TEST_API_TOKEN }}
        run: |
          k6 run --out json=spike-test-results.json tests/performance/spike-test.js

      - name: Check performance thresholds
        run: |
          # Parse results and fail if thresholds exceeded
          if grep -q '"thresholds".*"failed":true' *-test-results.json; then
            echo "Performance thresholds exceeded!"
            exit 1
          fi

      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: performance-test-results
          path: '*-test-results.json'
          retention-days: 30

      - name: Comment PR with results
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const results = JSON.parse(fs.readFileSync('load-test-results.json', 'utf8'));

            const metrics = results.metrics;
            const p95 = metrics.http_req_duration.values['p(95)'].toFixed(2);
            const p99 = metrics.http_req_duration.values['p(99)'].toFixed(2);
            const errorRate = (metrics.http_req_failed.values.rate * 100).toFixed(2);

            const comment = `## Performance Test Results

            | Metric | Value | Threshold | Status |
            |--------|-------|-----------|--------|
            | p95 Latency | ${p95}ms | <200ms | ${p95 < 200 ? '✅' : '❌'} |
            | p99 Latency | ${p99}ms | <500ms | ${p99 < 500 ? '✅' : '❌'} |
            | Error Rate | ${errorRate}% | <1% | ${errorRate < 1 ? '✅' : '❌'} |
            `;

            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.name,
              body: comment
            });
```

### Grafana + InfluxDB Integration

#### k6 Output to InfluxDB

```javascript
// Add to k6 test options
export const options = {
  // ... existing options ...

  // Send metrics to InfluxDB
  ext: {
    loadimpact: {
      projectID: 12345,
      name: 'Web Analyzer Performance Test',
    },
  },
};
```

#### InfluxDB Setup

```bash
# Start InfluxDB
docker run -d \
  --name influxdb \
  -p 8086:8086 \
  -e DOCKER_INFLUXDB_INIT_MODE=setup \
  -e DOCKER_INFLUXDB_INIT_USERNAME=admin \
  -e DOCKER_INFLUXDB_INIT_PASSWORD=admin123 \
  -e DOCKER_INFLUXDB_INIT_ORG=web-analyzer \
  -e DOCKER_INFLUXDB_INIT_BUCKET=k6 \
  influxdb:2.7

# Run k6 with InfluxDB output
k6 run --out influxdb=http://localhost:8086 \
  --tag testid=load-test-001 \
  tests/performance/load-test.js
```

#### Grafana Dashboard for k6 Metrics

```json
{
  "dashboard": {
    "title": "k6 Performance Testing Dashboard",
    "panels": [
      {
        "title": "Request Rate (RPS)",
        "targets": [
          {
            "query": "SELECT mean(\"value\") FROM \"http_reqs\" WHERE $timeFilter GROUP BY time(10s) fill(null)"
          }
        ]
      },
      {
        "title": "Response Time Percentiles",
        "targets": [
          {
            "query": "SELECT percentile(\"value\", 50) as p50, percentile(\"value\", 95) as p95, percentile(\"value\", 99) as p99 FROM \"http_req_duration\" WHERE $timeFilter GROUP BY time(10s)"
          }
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          {
            "query": "SELECT mean(\"value\") * 100 FROM \"http_req_failed\" WHERE $timeFilter GROUP BY time(10s)"
          }
        ]
      },
      {
        "title": "Virtual Users",
        "targets": [
          {
            "query": "SELECT max(\"value\") FROM \"vus\" WHERE $timeFilter GROUP BY time(10s)"
          }
        ]
      },
      {
        "title": "Checks Passed vs Failed",
        "targets": [
          {
            "query": "SELECT sum(\"value\") FROM \"checks\" WHERE \"passed\"='true' AND $timeFilter GROUP BY time(10s)",
            "alias": "Passed"
          },
          {
            "query": "SELECT sum(\"value\") FROM \"checks\" WHERE \"passed\"='false' AND $timeFilter GROUP BY time(10s)",
            "alias": "Failed"
          }
        ]
      }
    ]
  }
}
```

### Docker Test Environment

```yaml
# docker-compose.performance.yml
version: '3.8'

services:
  api:
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile
      target: api
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=keydb
      - RABBITMQ_HOST=rabbitmq
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - keydb
      - rabbitmq

  publisher:
    build:
      context: .
      target: publisher
    depends_on:
      - postgres
      - rabbitmq

  subscriber:
    build:
      context: .
      target: subscriber
    depends_on:
      - postgres
      - rabbitmq

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: web_analyzer_test
      POSTGRES_PASSWORD: postgres

  keydb:
    image: eqalpha/keydb:latest

  rabbitmq:
    image: rabbitmq:3.12-management-alpine
    environment:
      RABBITMQ_DEFAULT_USER: admin
      RABBITMQ_DEFAULT_PASS: admin

  influxdb:
    image: influxdb:2.7
    ports:
      - "8086:8086"
    environment:
      DOCKER_INFLUXDB_INIT_MODE: setup
      DOCKER_INFLUXDB_INIT_USERNAME: admin
      DOCKER_INFLUXDB_INIT_PASSWORD: admin123
      DOCKER_INFLUXDB_INIT_ORG: web-analyzer
      DOCKER_INFLUXDB_INIT_BUCKET: k6

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    volumes:
      - ./tests/performance/grafana-dashboards:/etc/grafana/provisioning/dashboards

  k6:
    image: grafana/k6:latest
    volumes:
      - ./tests/performance:/scripts
    depends_on:
      - api
      - influxdb
    command: run --out influxdb=http://influxdb:8086 /scripts/load-test.js
```

### Makefile Integration

```makefile
# Add to Makefile
.PHONY: perf-test
perf-test: ## Run performance tests locally
	@echo "🏃 Running performance tests..."
	@docker-compose -f docker-compose.performance.yml up -d
	@sleep 10 # Wait for services to start
	@k6 run --out json=performance-results.json tests/performance/load-test.js
	@docker-compose -f docker-compose.performance.yml down

.PHONY: perf-test-load
perf-test-load: ## Run load test
	@k6 run tests/performance/load-test.js

.PHONY: perf-test-stress
perf-test-stress: ## Run stress test
	@k6 run tests/performance/stress-test.js

.PHONY: perf-test-spike
perf-test-spike: ## Run spike test
	@k6 run tests/performance/spike-test.js

.PHONY: perf-test-soak
perf-test-soak: ## Run soak test (24 hours)
	@echo "⚠️  This will run for 24 hours. Press Ctrl+C to cancel."
	@k6 run tests/performance/soak-test.js

.PHONY: perf-test-all
perf-test-all: perf-test-load perf-test-stress perf-test-spike ## Run all performance tests
	@echo "✅ All performance tests completed"
```

### Implementation Checklist

- [ ] Install k6 locally for development
- [ ] Create test scenarios directory `tests/performance/`
- [ ] Implement load test script with SLO thresholds
- [ ] Implement stress test for breaking point identification
- [ ] Implement spike test for burst capacity validation
- [ ] Implement soak test for long-running stability (24h)
- [ ] Implement SSE connection test for real-time endpoints
- [ ] Create Docker Compose environment for performance testing
- [ ] Set up InfluxDB for metrics storage
- [ ] Create Grafana dashboards for k6 metrics visualization
- [ ] Configure GitHub Actions workflow for automated performance tests
- [ ] Add performance regression detection in CI/CD
- [ ] Create PR comment bot for performance results
- [ ] Document performance SLOs and acceptance criteria
- [ ] Add Makefile targets for easy local test execution
- [ ] Set up alerting for performance threshold violations
- [ ] Create performance test report templates
- [ ] Train team on performance testing best practices
- [ ] Establish baseline performance metrics
- [ ] Create performance improvement tracking dashboard

### Benefits

- **Automated Performance Validation**: Catch performance regressions before production
- **Concrete SLOs**: Specific latency, throughput, and error rate targets
- **Multiple Test Types**: Load, stress, spike, and soak testing for comprehensive coverage
- **CI/CD Integration**: Automated testing on every pull request
- **Real-time Metrics**: Grafana dashboards for live performance monitoring
- **Scalability Validation**: Verify auto-scaling and burst capacity
- **Cost Optimization**: Identify over-provisioning and resource waste
- **Capacity Planning**: Data-driven infrastructure sizing decisions
- **Developer-Friendly**: JavaScript-based tests easy for all developers
- **Trend Analysis**: Track performance improvements/degradations over time

---

## 5. End-to-End Testing with Playwright 🟡 **HIGH**

### Current State
- No E2E testing infrastructure
- Frontend changes tested manually
- No automated browser testing
- API contract validation missing
- No visual regression testing
- User workflows not covered by automated tests

### Overview

Implement comprehensive end-to-end testing using [Playwright](https://playwright.dev/), Microsoft's modern browser automation framework. Playwright provides reliable, fast, and capable automation for Chromium, Firefox, and WebKit with a single API.

### Fresh Playwright Setup

Complete Playwright configuration from scratch for the Web Analyzer project.

#### Installation

```bash
# In the web/ directory or project root
npm init playwright@latest

# Or manual installation
npm install -D @playwright/test
npm install -D @types/node

# Install browsers
npx playwright install
```

#### Project Structure

```
e2e/
├── tests/
│   ├── analysis-workflow.spec.ts       # Complete analysis flow
│   ├── auth-flow.spec.ts               # Authentication tests
│   ├── sse-realtime.spec.ts            # Server-Sent Events testing
│   ├── error-handling.spec.ts          # Error scenarios
│   ├── accessibility.spec.ts           # A11y tests
│   └── visual-regression.spec.ts       # Screenshot comparison
├── fixtures/
│   ├── test-data.json                  # Test data
│   ├── mock-responses.json             # API mocks
│   └── test-users.json                 # Test users
├── utils/
│   ├── api-helpers.ts                  # API interaction utilities
│   ├── auth-helpers.ts                 # Authentication helpers
│   └── wait-helpers.ts                 # Custom wait functions
├── screenshots/                        # Visual regression baselines
├── playwright.config.ts                # Main configuration
├── .env.test                           # Test environment variables
└── README.md                           # E2E testing documentation
```

### Playwright Configuration

```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';
import * as dotenv from 'dotenv';

// Load test environment variables
dotenv.config({ path: '.env.test' });

/**
 * Complete Playwright configuration for Web Analyzer E2E tests
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  // Test directory
  testDir: './e2e/tests',

  // Maximum time one test can run (30 seconds)
  timeout: 30 * 1000,

  // Maximum time for expect() to pass (5 seconds)
  expect: {
    timeout: 5000,
    toHaveScreenshot: {
      maxDiffPixels: 100,
      maxDiffPixelRatio: 0.01,
    },
  },

  // Run tests in files in parallel
  fullyParallel: true,

  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,

  // Retry on CI only
  retries: process.env.CI ? 2 : 0,

  // Opt out of parallel tests on CI
  workers: process.env.CI ? 1 : undefined,

  // Reporter to use
  reporter: [
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
    ['json', { outputFile: 'test-results.json' }],
    ['junit', { outputFile: 'junit.xml' }],
    ['list'],
    ...(process.env.CI ? [['github']] : []),
  ],

  // Shared settings for all projects
  use: {
    // Base URL for page.goto('/')
    baseURL: process.env.BASE_URL || 'https://web-analyzer.dev',

    // Collect trace when retrying the failed test
    trace: 'on-first-retry',

    // Screenshot on failure
    screenshot: 'only-on-failure',

    // Video on failure
    video: 'retain-on-failure',

    // Emulate timezone
    timezoneId: 'America/New_York',

    // Emulate locale
    locale: 'en-US',

    // Default navigation timeout
    navigationTimeout: 15 * 1000,

    // Default action timeout
    actionTimeout: 10 * 1000,
  },

  // Configure projects for major browsers
  projects: [
    // Setup project for authentication
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
    },

    // Chromium
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: 'e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Firefox
    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        storageState: 'e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // WebKit (Safari)
    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        storageState: 'e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Mobile Chrome
    {
      name: 'Mobile Chrome',
      use: {
        ...devices['Pixel 5'],
        storageState: 'e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Mobile Safari
    {
      name: 'Mobile Safari',
      use: {
        ...devices['iPhone 13'],
        storageState: 'e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    // Branded browsers
    {
      name: 'Microsoft Edge',
      use: {
        ...devices['Desktop Edge'],
        channel: 'msedge',
        storageState: 'e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },

    {
      name: 'Google Chrome',
      use: {
        ...devices['Desktop Chrome'],
        channel: 'chrome',
        storageState: 'e2e/.auth/user.json',
      },
      dependencies: ['setup'],
    },
  ],

  // Run local dev server before starting tests
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000,
  },
});
```

### Environment Configuration

```bash
# .env.test
BASE_URL=http://localhost:3000
API_BASE_URL=http://localhost:8080/v1
TEST_USER_EMAIL=test@example.com
TEST_USER_PASSWORD=TestPassword123!
API_TOKEN=test_token_here
HEADLESS=true
SLOWMO=0
```

### Authentication Setup

```typescript
// e2e/tests/auth.setup.ts
import { test as setup, expect } from '@playwright/test';
import * as path from 'path';

const authFile = path.join(__dirname, '../.auth/user.json');

setup('authenticate', async ({ page, context }) => {
  // Navigate to login page
  await page.goto('/login');

  // Fill in credentials
  await page.getByLabel('Email').fill(process.env.TEST_USER_EMAIL!);
  await page.getByLabel('Password').fill(process.env.TEST_USER_PASSWORD!);

  // Click login button
  await page.getByRole('button', { name: 'Log in' }).click();

  // Wait for successful redirect
  await page.waitForURL('/dashboard');

  // Verify logged in state
  await expect(page.getByRole('button', { name: 'Log out' })).toBeVisible();

  // Save storage state
  await context.storageState({ path: authFile });
});
```

### Test Scenarios

#### 1. Complete Analysis Workflow

```typescript
// e2e/tests/analysis-workflow.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Analysis Workflow', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should complete full analysis workflow', async ({ page }) => {
    // Navigate to analysis page
    await page.getByRole('link', { name: 'New Analysis' }).click();
    await expect(page).toHaveURL('/analyze');

    // Fill in URL to analyze
    const testUrl = 'https://example.com';
    const urlInput = page.getByLabel('URL to analyze');
    await urlInput.fill(testUrl);

    // Submit form
    const submitButton = page.getByRole('button', { name: 'Analyze' });
    await submitButton.click();

    // Wait for analysis to start
    await expect(page.getByText('Analysis started')).toBeVisible();

    // Extract analysis ID from URL
    await page.waitForURL(/\/analysis\/[a-f0-9-]+/);
    const analysisId = page.url().split('/').pop();
    expect(analysisId).toMatch(/^[a-f0-9-]+$/);

    // Wait for SSE connection and status updates
    await expect(page.getByText('Status:')).toBeVisible();

    // Wait for completion (with timeout)
    await expect(
      page.getByText('Status: completed'),
      { timeout: 60000 }
    ).toBeVisible();

    // Verify results are displayed
    await expect(page.getByRole('heading', { name: 'Analysis Results' })).toBeVisible();
    await expect(page.getByText(testUrl)).toBeVisible();

    // Check for specific result fields
    await expect(page.getByText(/HTML Version:/)).toBeVisible();
    await expect(page.getByText(/Links Found:/)).toBeVisible();
    await expect(page.getByText(/Images:/)).toBeVisible();

    // Verify export button is available
    await expect(page.getByRole('button', { name: /Export/ })).toBeVisible();
  });

  test('should handle invalid URL gracefully', async ({ page }) => {
    await page.goto('/analyze');

    // Enter invalid URL
    const urlInput = page.getByLabel('URL to analyze');
    await urlInput.fill('not-a-valid-url');

    // Submit form
    await page.getByRole('button', { name: 'Analyze' }).click();

    // Expect validation error
    await expect(page.getByText(/Invalid URL format/)).toBeVisible();

    // Form should not submit
    await expect(page).toHaveURL('/analyze');
  });

  test('should display real-time progress via SSE', async ({ page }) => {
    await page.goto('/analyze');

    // Submit valid analysis
    await page.getByLabel('URL to analyze').fill('https://example.com');
    await page.getByRole('button', { name: 'Analyze' }).click();

    // Wait for SSE connection
    await page.waitForURL(/\/analysis\/[a-f0-9-]+/);

    // Track status changes
    const statusUpdates: string[] = [];

    // Listen for status text changes
    const statusElement = page.getByText(/Status:/);

    // Verify at least these states appear
    await expect(statusElement).toContainText('pending', { timeout: 2000 });
    await expect(statusElement).toContainText('processing', { timeout: 10000 });
    await expect(statusElement).toContainText('completed', { timeout: 60000 });
  });
});
```

#### 2. Authentication Flow Testing

```typescript
// e2e/tests/auth-flow.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Authentication Flow', () => {
  test.use({ storageState: { cookies: [], origins: [] } }); // No auth

  test('should login successfully with valid credentials', async ({ page }) => {
    await page.goto('/login');

    await page.getByLabel('Email').fill('user@example.com');
    await page.getByLabel('Password').fill('ValidPassword123!');
    await page.getByRole('button', { name: 'Log in' }).click();

    // Should redirect to dashboard
    await expect(page).toHaveURL('/dashboard');
    await expect(page.getByRole('button', { name: 'Log out' })).toBeVisible();
  });

  test('should show error with invalid credentials', async ({ page }) => {
    await page.goto('/login');

    await page.getByLabel('Email').fill('user@example.com');
    await page.getByLabel('Password').fill('WrongPassword');
    await page.getByRole('button', { name: 'Log in' }).click();

    // Should show error message
    await expect(page.getByText(/Invalid credentials/i)).toBeVisible();

    // Should stay on login page
    await expect(page).toHaveURL('/login');
  });

  test('should logout successfully', async ({ page }) => {
    // First login
    await page.goto('/login');
    await page.getByLabel('Email').fill('user@example.com');
    await page.getByLabel('Password').fill('ValidPassword123!');
    await page.getByRole('button', { name: 'Log in' }).click();

    await expect(page).toHaveURL('/dashboard');

    // Then logout
    await page.getByRole('button', { name: 'Log out' }).click();

    // Should redirect to login
    await expect(page).toHaveURL('/login');

    // Should not be able to access protected routes
    await page.goto('/dashboard');
    await expect(page).toHaveURL('/login');
  });

  test('should handle token expiration', async ({ page, context }) => {
    // Set expired token in cookies
    await context.addCookies([
      {
        name: 'auth_token',
        value: 'expired_token_here',
        domain: 'localhost',
        path: '/',
        expires: Date.now() / 1000 - 1000, // Expired
      },
    ]);

    // Try to access protected route
    await page.goto('/dashboard');

    // Should redirect to login with session expired message
    await expect(page).toHaveURL('/login');
    await expect(page.getByText(/Session expired/i)).toBeVisible();
  });
});
```

#### 3. Server-Sent Events (SSE) Real-time Testing

```typescript
// e2e/tests/sse-realtime.spec.ts
import { test, expect } from '@playwright/test';

test.describe('SSE Real-time Updates', () => {
  test('should receive real-time status updates via SSE', async ({ page }) => {
    // Start an analysis
    await page.goto('/analyze');
    await page.getByLabel('URL to analyze').fill('https://example.com');
    await page.getByRole('button', { name: 'Analyze' }).click();

    await page.waitForURL(/\/analysis\/[a-f0-9-]+/);

    // Monitor network for SSE connection
    const sseRequest = page.waitForRequest(
      request => request.url().includes('/events') &&
                 request.headers()['accept'] === 'text/event-stream'
    );

    await expect(sseRequest).resolves.toBeTruthy();

    // Verify SSE messages are received
    const statusElement = page.locator('[data-testid="analysis-status"]');

    // Should see progression of states
    const states = ['pending', 'processing', 'completed'];
    for (const state of states) {
      await expect(statusElement).toContainText(state, { timeout: 30000 });
    }
  });

  test('should reconnect SSE on connection loss', async ({ page, context }) => {
    await page.goto('/analyze');
    await page.getByLabel('URL to analyze').fill('https://example.com');
    await page.getByRole('button', { name: 'Analyze' }).click();

    await page.waitForURL(/\/analysis\/[a-f0-9-]+/);

    // Wait for initial SSE connection
    await page.waitForTimeout(2000);

    // Simulate connection loss by going offline
    await context.setOffline(true);
    await page.waitForTimeout(1000);

    // Go back online
    await context.setOffline(false);

    // Should see reconnection indicator
    await expect(page.getByText(/Reconnected/i)).toBeVisible({ timeout: 5000 });

    // Should continue receiving updates
    await expect(
      page.locator('[data-testid="analysis-status"]')
    ).toContainText(/processing|completed/, { timeout: 30000 });
  });

  test('should handle SSE graceful closure on completion', async ({ page }) => {
    await page.goto('/analyze');
    await page.getByLabel('URL to analyze').fill('https://example.com');
    await page.getByRole('button', { name: 'Analyze' }).click();

    await page.waitForURL(/\/analysis\/[a-f0-9-]+/);

    // Wait for completion
    await expect(
      page.getByText('Status: completed'),
      { timeout: 60000 }
    ).toBeVisible();

    // SSE should close gracefully (check via network tab)
    const responses = await page.context().on('response', response => {
      if (response.url().includes('/events')) {
        expect(response.status()).toBe(200);
      }
    });

    // No error messages about SSE
    await expect(page.getByText(/SSE error|Connection failed/)).not.toBeVisible();
  });
});
```

#### 4. Error Handling and Edge Cases

```typescript
// e2e/tests/error-handling.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Error Handling', () => {
  test('should handle network errors gracefully', async ({ page, context }) => {
    await page.goto('/analyze');

    // Fill form
    await page.getByLabel('URL to analyze').fill('https://example.com');

    // Go offline before submitting
    await context.setOffline(true);

    await page.getByRole('button', { name: 'Analyze' }).click();

    // Should show network error
    await expect(page.getByText(/Network error|Unable to connect/i)).toBeVisible();

    // Go back online
    await context.setOffline(false);

    // Retry button should work
    await page.getByRole('button', { name: /Retry/i }).click();

    // Should now succeed
    await expect(page.getByText('Analysis started')).toBeVisible();
  });

  test('should handle API errors (500)', async ({ page }) => {
    // Mock API to return 500
    await page.route('**/v1/analyze', async route => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({
          error: 'Internal Server Error',
          code: 'INTERNAL_ERROR',
        }),
      });
    });

    await page.goto('/analyze');
    await page.getByLabel('URL to analyze').fill('https://example.com');
    await page.getByRole('button', { name: 'Analyze' }).click();

    // Should display error message
    await expect(page.getByText(/Internal Server Error/i)).toBeVisible();

    // Should not navigate away
    await expect(page).toHaveURL('/analyze');
  });

  test('should handle rate limiting (429)', async ({ page }) => {
    // Mock rate limit response
    await page.route('**/v1/analyze', async route => {
      await route.fulfill({
        status: 429,
        headers: {
          'Retry-After': '60',
          'X-RateLimit-Remaining': '0',
        },
        contentType: 'application/json',
        body: JSON.stringify({
          error: 'Rate limit exceeded',
          code: 'RATE_LIMIT_EXCEEDED',
        }),
      });
    });

    await page.goto('/analyze');
    await page.getByLabel('URL to analyze').fill('https://example.com');
    await page.getByRole('button', { name: 'Analyze' }).click();

    // Should show rate limit message with retry time
    await expect(page.getByText(/Rate limit exceeded/i)).toBeVisible();
    await expect(page.getByText(/Try again in 60 seconds/i)).toBeVisible();

    // Submit button should be disabled
    await expect(page.getByRole('button', { name: 'Analyze' })).toBeDisabled();
  });

  test('should validate form inputs', async ({ page }) => {
    await page.goto('/analyze');

    const urlInput = page.getByLabel('URL to analyze');
    const submitButton = page.getByRole('button', { name: 'Analyze' });

    // Test empty input
    await submitButton.click();
    await expect(page.getByText(/URL is required/i)).toBeVisible();

    // Test invalid URL format
    await urlInput.fill('not a url');
    await submitButton.click();
    await expect(page.getByText(/Invalid URL format/i)).toBeVisible();

    // Test invalid protocol
    await urlInput.fill('ftp://example.com');
    await submitButton.click();
    await expect(page.getByText(/Only HTTP and HTTPS URLs are supported/i)).toBeVisible();

    // Test valid URL (should not show errors)
    await urlInput.fill('https://example.com');
    await submitButton.click();
    await expect(page.getByText(/required|invalid/i)).not.toBeVisible();
  });
});
```

#### 5. API Contract Testing

```typescript
// e2e/tests/api-contract.spec.ts
import { test, expect } from '@playwright/test';
import OpenAPIParser from '@readme/openapi-parser';
import Ajv from 'ajv';
import addFormats from 'ajv-formats';

test.describe('API Contract Validation', () => {
  let openApiSpec: any;
  let ajv: Ajv;

  test.beforeAll(async () => {
    // Load OpenAPI specification
    openApiSpec = await OpenAPIParser.validate('./docs/openapi-spec/openapi.yaml');

    // Initialize JSON Schema validator
    ajv = new Ajv({ allErrors: true, strict: false });
    addFormats(ajv);
  });

  test('POST /v1/analyze should match OpenAPI spec', async ({ request }) => {
    const response = await request.post('/v1/analyze', {
      data: {
        url: 'https://example.com',
      },
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${process.env.API_TOKEN}`,
      },
    });

    expect(response.status()).toBe(202);

    // Validate response schema
    const responseData = await response.json();
    const schema = openApiSpec.paths['/v1/analyze'].post.responses['202'].content['application/json'].schema;

    const validate = ajv.compile(schema);
    const valid = validate(responseData);

    if (!valid) {
      console.error('Schema validation errors:', validate.errors);
    }

    expect(valid).toBe(true);
  });

  test('GET /v1/analysis/{id} should match OpenAPI spec', async ({ request }) => {
    // First create an analysis
    const createResponse = await request.post('/v1/analyze', {
      data: { url: 'https://example.com' },
    });
    const { analysis_id } = await createResponse.json();

    // Get analysis details
    const response = await request.get(`/v1/analysis/${analysis_id}`);

    expect(response.status()).toBe(200);

    // Validate response schema
    const responseData = await response.json();
    const schema = openApiSpec.paths['/v1/analysis/{analysisId}'].get.responses['200'].content['application/json'].schema;

    const validate = ajv.compile(schema);
    const valid = validate(responseData);

    expect(valid).toBe(true);
  });

  test('Error responses should match OpenAPI spec', async ({ request }) => {
    // Test 404 response
    const response = await request.get('/v1/analysis/nonexistent-id');

    expect(response.status()).toBe(404);

    const responseData = await response.json();

    // Validate error response structure
    expect(responseData).toHaveProperty('error');
    expect(responseData).toHaveProperty('code');
  });
});
```

#### 6. Visual Regression Testing

```typescript
// e2e/tests/visual-regression.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Visual Regression', () => {
  test('home page should match baseline', async ({ page }) => {
    await page.goto('/');

    // Wait for page to be fully loaded
    await page.waitForLoadState('networkidle');

    // Take screenshot and compare
    await expect(page).toHaveScreenshot('home-page.png', {
      fullPage: true,
      maxDiffPixels: 100,
    });
  });

  test('analysis form should match baseline', async ({ page }) => {
    await page.goto('/analyze');
    await page.waitForLoadState('networkidle');

    await expect(page).toHaveScreenshot('analyze-form.png');
  });

  test('analysis results should match baseline', async ({ page }) => {
    // Mock completed analysis
    await page.route('**/v1/analysis/*', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          analysis_id: 'test-id',
          url: 'https://example.com',
          status: 'completed',
          results: {
            html_version: 'HTML5',
            links_found: 25,
            images: 10,
          },
        }),
      });
    });

    await page.goto('/analysis/test-id');
    await page.waitForLoadState('networkidle');

    await expect(page).toHaveScreenshot('analysis-results.png', {
      fullPage: true,
    });
  });

  test('responsive design - mobile view', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 }); // iPhone SE
    await page.goto('/');

    await expect(page).toHaveScreenshot('home-mobile.png');
  });

  test('responsive design - tablet view', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 }); // iPad
    await page.goto('/');

    await expect(page).toHaveScreenshot('home-tablet.png');
  });
});
```

### CI/CD Integration

```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
  workflow_dispatch:

jobs:
  test:
    name: E2E Tests
    runs-on: ubuntu-latest
    timeout-minutes: 60

    strategy:
      fail-fast: false
      matrix:
        browser: [chromium, firefox, webkit]

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Install Playwright Browsers
        run: npx playwright install --with-deps ${{ matrix.browser }}

      - name: Start backend services
        run: |
          docker-compose up -d
          # Wait for services
          timeout 60 bash -c 'until curl -f http://localhost:8080/v1/health; do sleep 2; done'

      - name: Run Playwright tests
        run: npx playwright test --project=${{ matrix.browser }}
        env:
          BASE_URL: http://localhost:3000
          API_BASE_URL: http://localhost:8080/v1

      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report-${{ matrix.browser }}
          path: playwright-report/
          retention-days: 30

      - name: Upload test screenshots
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-screenshots-${{ matrix.browser }}
          path: e2e/screenshots/
          retention-days: 7

      - name: Comment PR with test results
        if: github.event_name == 'pull_request' && failure()
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const results = JSON.parse(fs.readFileSync('test-results.json', 'utf8'));

            const comment = `## E2E Test Results (${{ matrix.browser }})

            ❌ Tests Failed

            **Failed Tests:**
            ${results.suites.map(s => s.specs.filter(spec => spec.ok === false)
              .map(spec => `- ${spec.title}`).join('\n')).join('\n')}

            See [test report](https://github.com/${{github.repository}}/actions/runs/${{github.run_id}}) for details.
            `;

            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.name,
              body: comment
            });
```

### Docker Test Environment

```yaml
# docker-compose.e2e.yml
version: '3.8'

services:
  api:
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile
      target: api
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=keydb
      - RABBITMQ_HOST=rabbitmq
    depends_on:
      postgres:
        condition: service_healthy
      keydb:
        condition: service_started
      rabbitmq:
        condition: service_healthy

  publisher:
    build:
      context: .
      target: publisher
    depends_on:
      - postgres
      - rabbitmq

  subscriber:
    build:
      context: .
      target: subscriber
    depends_on:
      - postgres
      - rabbitmq

  web:
    build:
      context: ./web
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
    environment:
      - VITE_API_BASE_URL=http://api:8080/v1

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: web_analyzer_e2e
      POSTGRES_PASSWORD: postgres
    healthcheck:
      test: pg_isready -U postgres
      interval: 5s
      timeout: 3s
      retries: 5

  keydb:
    image: eqalpha/keydb:latest

  rabbitmq:
    image: rabbitmq:3.12-management-alpine
    environment:
      RABBITMQ_DEFAULT_USER: admin
      RABBITMQ_DEFAULT_PASS: admin
    healthcheck:
      test: rabbitmq-diagnostics -q ping
      interval: 5s
      timeout: 3s
      retries: 5
```

### Makefile Integration

```makefile
# Add to Makefile
.PHONY: e2e-test
e2e-test: ## Run E2E tests
	@echo "🎭 Running E2E tests..."
	@docker-compose -f docker-compose.e2e.yml up -d
	@sleep 10
	@npx playwright test
	@docker-compose -f docker-compose.e2e.yml down

.PHONY: e2e-test-ui
e2e-test-ui: ## Run E2E tests in UI mode
	@echo "🎭 Running E2E tests in UI mode..."
	@docker-compose -f docker-compose.e2e.yml up -d
	@sleep 10
	@npx playwright test --ui
	@docker-compose -f docker-compose.e2e.yml down

.PHONY: e2e-test-debug
e2e-test-debug: ## Debug E2E tests
	@npx playwright test --debug

.PHONY: e2e-test-report
e2e-test-report: ## Show E2E test report
	@npx playwright show-report

.PHONY: e2e-test-screenshots
e2e-test-screenshots: ## Update visual regression baselines
	@npx playwright test --update-snapshots
```

### Implementation Checklist

- [ ] Install Playwright and dependencies
- [ ] Create Playwright configuration file
- [ ] Set up test project structure
- [ ] Implement authentication setup test
- [ ] Create complete analysis workflow test
- [ ] Implement SSE real-time testing
- [ ] Add error handling tests
- [ ] Create API contract validation tests
- [ ] Implement visual regression tests
- [ ] Add accessibility testing
- [ ] Create test fixtures and helpers
- [ ] Set up Docker Compose for E2E environment
- [ ] Configure GitHub Actions workflow
- [ ] Add test reporting and artifacts
- [ ] Create PR comment bot for test results
- [ ] Add screenshot comparison for visual regression
- [ ] Configure multi-browser testing
- [ ] Add mobile device testing
- [ ] Create test data management strategy
- [ ] Document E2E testing practices
- [ ] Train team on Playwright usage

### Benefits

- **Comprehensive Coverage**: Full user workflow testing from browser to backend
- **Multi-Browser**: Chromium, Firefox, and WebKit support
- **Real-time Testing**: SSE and WebSocket connection validation
- **API Contract Validation**: Ensure API matches OpenAPI specification
- **Visual Regression**: Catch unintended UI changes
- **Accessibility**: Built-in a11y testing capabilities
- **Developer-Friendly**: TypeScript support with excellent autocomplete
- **Reliable**: Auto-wait and retry mechanisms reduce flakiness
- **Fast**: Parallel test execution across browsers
- **CI/CD Integration**: Automated testing on every commit
- **Debugging**: Excellent debugging tools and trace viewer
- **Mobile Testing**: Test on emulated mobile devices

---

## 6. Linting and Code Quality 🟡 **HIGH**

### Current State
- No `.golangci.yml` configuration file
- No pre-commit hooks
- Inconsistent code style enforcement

### Required Improvements
```yaml
.golangci.yml Configuration:
  linters:
    enable:
      - gofmt
      - golint
      - govet
      - gosec
      - ineffassign
      - misspell
      - unconvert
      - gocritic
      - gocyclo
      - dupl

  linters-settings:
    gocyclo:
      min-complexity: 15
    dupl:
      threshold: 100
```

### Pre-commit Hooks
```yaml
.pre-commit-config.yaml:
  - repo: local
    hooks:
      - id: go-fmt
      - id: go-vet
      - id: go-lint
      - id: go-test
```

---

## 7. Integration Testing 🟡 **HIGH**

### Current State
- Empty integration test files in `itest/` directory
- No end-to-end testing coverage
- No performance benchmarks

### Required Improvements

**Infrastructure**: Use [Testcontainers for Go](https://golang.testcontainers.org/) for integration tests to ensure real infrastructure dependencies (PostgreSQL, RabbitMQ, KeyDB) are available during testing without manual setup.

**Benefits:**
- Real infrastructure testing without mocking
- Isolated test environments per test run
- Automatic cleanup after tests complete
- Version pinning for consistent test environments
- Parallel test execution with container isolation

```go
// itest/api_integration_test.go
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

func TestAnalysisE2E(t *testing.T) {
    t.Parallel()

    // Setup testcontainers for PostgreSQL, RabbitMQ, KeyDB
    ctx := context.Background()

    pgContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
    )
    defer pgContainer.Terminate(ctx)

    // Test complete analysis flow:
    // 1. Submit URL for analysis
    // 2. Verify event published to queue
    // 3. Confirm processing completion
    // 4. Validate analysis results
}

func TestRabbitMQIntegration(t *testing.T) {
    t.Parallel()

    // Use testcontainers RabbitMQ module
    // Test message queue operations
}

func TestDatabaseIntegration(t *testing.T) {
    t.Parallel()

    // Use testcontainers PostgreSQL module
    // Test database transactions and migrations
}

func BenchmarkAnalysisPerformance(b *testing.B) {
    // Performance benchmarks for analysis workflow
}
```

### Repository Layer Testing with go-sqlmock

**Library**: Use [`github.com/DATA-DOG/go-sqlmock`](https://github.com/DATA-DOG/go-sqlmock) for unit testing repository layer SQL queries without requiring a real database connection.

**Why go-sqlmock:**
- Unit test SQL queries in isolation without database overhead
- Validate exact SQL statements and parameter binding
- Test error scenarios and edge cases easily
- Fast execution (no I/O, no container startup)
- Complement integration tests with testcontainers

**Testing Strategy:**
- **Unit Tests**: Use go-sqlmock for repository method validation (SQL correctness, parameter binding)
- **Integration Tests**: Use testcontainers for end-to-end database behavior (transactions, constraints, triggers)

#### Implementation Example

```go
// internal/adapters/repos/analysis_repository_test.go
package repos

import (
    "context"
    "database/sql"
    "testing"
    "time"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/architeacher/svc-web-analyzer/internal/domain"
)

func TestAnalysisRepository_Create(t *testing.T) {
    t.Parallel()

    analysisID := uuid.New()
    url := "https://example.com"
    status := domain.AnalysisStatusPending
    now := time.Now()

    t.Run("should execute correct INSERT query", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        expectedSQL := `
            INSERT INTO analysis
                \(id, url, status, created_at, expires_at\)
            VALUES
                \(\$1, \$2, \$3, \$4, \$5\)
        `

        mock.ExpectExec(expectedSQL).
            WithArgs(
                analysisID,
                url,
                status,
                sqlmock.AnyArg(), // created_at timestamp
                sqlmock.AnyArg(), // expires_at timestamp
            ).
            WillReturnResult(sqlmock.NewResult(1, 1))

        analysis := &domain.Analysis{
            ID:        analysisID,
            URL:       url,
            Status:    status,
            CreatedAt: now,
        }

        err = repo.Create(context.Background(), analysis)
        require.NoError(t, err)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err, "all SQL expectations should be met")
    })

    t.Run("should handle database errors", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        mock.ExpectExec("INSERT INTO analysis").
            WillReturnError(sql.ErrConnDone)

        analysis := &domain.Analysis{
            ID:     analysisID,
            URL:    url,
            Status: status,
        }

        err = repo.Create(context.Background(), analysis)
        assert.Error(t, err)
        assert.ErrorIs(t, err, sql.ErrConnDone)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })
}

func TestAnalysisRepository_GetByID(t *testing.T) {
    t.Parallel()

    analysisID := uuid.New()
    expectedURL := "https://example.com"
    expectedStatus := domain.AnalysisStatusCompleted

    t.Run("should execute correct SELECT query", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        expectedSQL := `
            SELECT
                id, url, status, content_hash, content_size,
                created_at, completed_at, archived_at, expires_at,
                duration, lock_version
            FROM analysis
            WHERE id = \$1 AND archived_at IS NULL
        `

        rows := sqlmock.NewRows([]string{
            "id", "url", "status", "content_hash", "content_size",
            "created_at", "completed_at", "archived_at", "expires_at",
            "duration", "lock_version",
        }).AddRow(
            analysisID,
            expectedURL,
            expectedStatus,
            nil,
            nil,
            time.Now(),
            nil,
            nil,
            nil,
            nil,
            1,
        )

        mock.ExpectQuery(expectedSQL).
            WithArgs(analysisID).
            WillReturnRows(rows)

        analysis, err := repo.GetByID(context.Background(), analysisID.String())
        require.NoError(t, err)
        assert.Equal(t, analysisID, analysis.ID)
        assert.Equal(t, expectedURL, analysis.URL)
        assert.Equal(t, expectedStatus, analysis.Status)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })

    t.Run("should return error when analysis not found", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        mock.ExpectQuery("SELECT (.+) FROM analysis").
            WithArgs(analysisID).
            WillReturnError(sql.ErrNoRows)

        analysis, err := repo.GetByID(context.Background(), analysisID.String())
        assert.Error(t, err)
        assert.Nil(t, analysis)
        assert.ErrorIs(t, err, sql.ErrNoRows)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })
}

func TestAnalysisRepository_Update(t *testing.T) {
    t.Parallel()

    t.Run("should execute optimistic locking UPDATE", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        analysisID := uuid.New()
        currentVersion := 1

        expectedSQL := `
            UPDATE analysis
            SET
                status = \$1,
                completed_at = \$2,
                lock_version = lock_version \+ 1
            WHERE
                id = \$3
                AND lock_version = \$4
        `

        mock.ExpectExec(expectedSQL).
            WithArgs(
                domain.AnalysisStatusCompleted,
                sqlmock.AnyArg(),
                analysisID,
                currentVersion,
            ).
            WillReturnResult(sqlmock.NewResult(0, 1))

        analysis := &domain.Analysis{
            ID:          analysisID,
            Status:      domain.AnalysisStatusCompleted,
            CompletedAt: ptrTime(time.Now()),
            LockVersion: currentVersion,
        }

        err = repo.Update(context.Background(), analysis)
        require.NoError(t, err)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })

    t.Run("should detect concurrent modification", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        mock.ExpectExec("UPDATE analysis").
            WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

        analysis := &domain.Analysis{
            ID:          uuid.New(),
            LockVersion: 1,
        }

        err = repo.Update(context.Background(), analysis)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "concurrent modification")

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })
}

func TestAnalysisRepository_FindExpiredBatch(t *testing.T) {
    t.Parallel()

    t.Run("should query expired records with correct filters", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        keepIfYounger := time.Now().Add(-24 * time.Hour)
        limit := 100

        expectedSQL := `
            SELECT id
            FROM analysis
            WHERE
                expires_at IS NOT NULL
                AND expires_at < NOW\(\)
                AND archived_at IS NULL
                AND created_at < \$1
            ORDER BY expires_at ASC
            LIMIT \$2
        `

        expiredID1 := uuid.New()
        expiredID2 := uuid.New()

        rows := sqlmock.NewRows([]string{"id"}).
            AddRow(expiredID1).
            AddRow(expiredID2)

        mock.ExpectQuery(expectedSQL).
            WithArgs(keepIfYounger, limit).
            WillReturnRows(rows)

        ids, err := repo.FindExpiredBatch(context.Background(), keepIfYounger, limit)
        require.NoError(t, err)
        assert.Len(t, ids, 2)
        assert.Contains(t, ids, expiredID1)
        assert.Contains(t, ids, expiredID2)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })
}

func TestAnalysisRepository_DeleteBatch(t *testing.T) {
    t.Parallel()

    t.Run("should execute batch delete with ANY operator", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        ids := []uuid.UUID{
            uuid.New(),
            uuid.New(),
            uuid.New(),
        }

        expectedSQL := `DELETE FROM analysis WHERE id = ANY\(\$1\)`

        mock.ExpectExec(expectedSQL).
            WithArgs(sqlmock.AnyArg()). // pq.Array(ids)
            WillReturnResult(sqlmock.NewResult(0, 3))

        err = repo.DeleteBatch(context.Background(), ids)
        require.NoError(t, err)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })

    t.Run("should handle empty batch gracefully", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        err = repo.DeleteBatch(context.Background(), []uuid.UUID{})
        require.NoError(t, err)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err, "should not execute any SQL for empty batch")
    })
}

func ptrTime(t time.Time) *time.Time {
    return &t
}
```

#### Transaction Testing

```go
func TestAnalysisRepository_CreateWithTransaction(t *testing.T) {
    t.Parallel()

    t.Run("should commit transaction on success", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        mock.ExpectBegin()
        mock.ExpectExec("INSERT INTO analysis").
            WillReturnResult(sqlmock.NewResult(1, 1))
        mock.ExpectCommit()

        tx, err := db.Begin()
        require.NoError(t, err)

        repoWithTx := repo.WithTx(tx)

        analysis := &domain.Analysis{
            ID:     uuid.New(),
            URL:    "https://example.com",
            Status: domain.AnalysisStatusPending,
        }

        err = repoWithTx.Create(context.Background(), analysis)
        require.NoError(t, err)

        err = tx.Commit()
        require.NoError(t, err)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })

    t.Run("should rollback transaction on error", func(t *testing.T) {
        t.Parallel()

        db, mock, err := sqlmock.New()
        require.NoError(t, err)
        defer db.Close()

        repo := NewAnalysisRepository(db)

        mock.ExpectBegin()
        mock.ExpectExec("INSERT INTO analysis").
            WillReturnError(sql.ErrConnDone)
        mock.ExpectRollback()

        tx, err := db.Begin()
        require.NoError(t, err)

        repoWithTx := repo.WithTx(tx)

        analysis := &domain.Analysis{
            ID:  uuid.New(),
            URL: "https://example.com",
        }

        err = repoWithTx.Create(context.Background(), analysis)
        require.Error(t, err)

        err = tx.Rollback()
        require.NoError(t, err)

        err = mock.ExpectationsWereMet()
        assert.NoError(t, err)
    })
}
```

#### Best Practices

**1. SQL Assertion Strategies:**
```go
// Strict matching - validates exact SQL with regex escaping
mock.ExpectQuery(`SELECT \* FROM analysis WHERE id = \$1`)

// Flexible matching - uses simple substring matching
mock.ExpectQuery("SELECT (.+) FROM analysis")

// Column validation - verify specific columns are selected
mock.ExpectQuery("SELECT id, url, status FROM analysis")
```

**2. Argument Matchers:**
```go
// Exact value matching
mock.ExpectExec("INSERT INTO analysis").
    WithArgs(uuid.MustParse("..."), "https://example.com", "pending")

// Any argument (useful for timestamps)
mock.ExpectExec("INSERT INTO analysis").
    WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg())

// Custom argument matcher
type timestampMatcher struct{}
func (m timestampMatcher) Match(v interface{}) bool {
    _, ok := v.(time.Time)
    return ok
}

mock.ExpectExec("INSERT INTO analysis").
    WithArgs(uuid.New(), "url", timestampMatcher{})
```

**3. Row Scanning:**
```go
// Test NULL column handling
rows := sqlmock.NewRows([]string{"id", "completed_at", "archived_at"}).
    AddRow(uuid.New(), nil, nil) // NULL values

mock.ExpectQuery("SELECT (.+) FROM analysis").
    WillReturnRows(rows)
```

**4. Error Scenarios:**
```go
// Database connection errors
mock.ExpectExec("INSERT INTO analysis").
    WillReturnError(sql.ErrConnDone)

// Constraint violations
mock.ExpectExec("INSERT INTO analysis").
    WillReturnError(&pq.Error{Code: "23505"}) // unique_violation

// Deadlock detection
mock.ExpectExec("UPDATE analysis").
    WillReturnError(&pq.Error{Code: "40P01"}) // deadlock_detected
```

#### Integration with Existing Tests

```go
// internal/adapters/repos/suite_test.go
package repos

import (
    "database/sql"
    "testing"

    "github.com/DATA-DOG/go-sqlmock"
)

// Test suite for all repository tests
type RepositoryTestSuite struct {
    db   *sql.DB
    mock sqlmock.Sqlmock
}

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
    t.Helper()

    db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
    if err != nil {
        t.Fatalf("failed to create sqlmock: %v", err)
    }

    return db, mock
}

// Example test using suite pattern
func TestRepositorySuite(t *testing.T) {
    t.Parallel()

    db, mock := setupMockDB(t)
    defer db.Close()

    suite := &RepositoryTestSuite{
        db:   db,
        mock: mock,
    }

    t.Run("AnalysisRepository", func(t *testing.T) {
        suite.testAnalysisRepository(t)
    })

    t.Run("OutboxRepository", func(t *testing.T) {
        suite.testOutboxRepository(t)
    })
}
```

#### Benefits Summary

- **SQL Validation**: Ensures queries match expected SQL syntax and structure
- **Parameter Verification**: Validates correct parameter binding and order
- **Error Simulation**: Test error handling without database failures
- **Fast Execution**: No database I/O, runs in milliseconds
- **Isolation**: Each test is completely independent
- **Complementary**: Works alongside testcontainers for full coverage

#### Implementation Checklist

- [ ] Add `github.com/DATA-DOG/go-sqlmock` dependency
- [ ] Create unit tests for all repository methods
- [ ] Test optimistic locking with lock_version
- [ ] Test transaction handling (commit/rollback)
- [ ] Test error scenarios (constraints, deadlocks, connection errors)
- [ ] Test batch operations (DeleteBatch, FindExpiredBatch)
- [ ] Test NULL column handling
- [ ] Test archived_at and expires_at filters
- [ ] Validate SQL query structure and parameter binding
- [ ] Integrate with existing test suite

---

## 8. Pipeline Pattern Implementation 🟢 **MEDIUM**

### Current State
- Pipeline code exists in `pkg/pipeline/` with workflow orchestration support
- Services don't leverage the pipeline pattern
- No integration with the subscriber service for analysis workflow

### Required Improvements
```go
// Integrate pipeline into subscriber analysis workflow
// Location: internal/service/analysis_service.go or internal/usecases/commands/

import (
    "github.com/architeacher/svc-web-analyzer/pkg/pipeline"
)

type AnalysisPipeline struct {
    workflow *pipeline.Workflow
}

// Stages to implement:
// 1. FetchStage - Retrieve web page content
// 2. ParseStage - Parse HTML document structure
// 3. AnalyzeStage - Extract comprehensive metrics:
//    - HTML structure (headings, forms, links)
//    - Media resources (images, videos, SVGs)
//    - Stylesheets (link tags with rel="stylesheet", inline styles)
//    - Scripts (external scripts, inline scripts)
//    - Metadata (title, description, Open Graph, Twitter Cards)
//    - Accessibility attributes (alt text, ARIA labels)
// 4. ResourceCheckStage - Validate resource availability:
//    - Image URLs (check for broken images)
//    - Video sources (validate media URLs)
//    - Stylesheet URLs (check CSS availability)
//    - Script URLs (check JavaScript availability)
// 5. LinkCheckStage - Validate internal/external links
// 6. StoreStage - Persist analysis results to PostgreSQL
// 7. NotifyStage - Emit SSE events for real-time updates

func NewAnalysisPipeline() *AnalysisPipeline {
    workflow := pipeline.NewWorkflow()

    // Configure stages with proper error handling
    workflow.AddStage(NewFetchStage())
    workflow.AddStage(NewParseStage())
    workflow.AddStage(NewAnalyzeStage())
    workflow.AddStage(NewResourceCheckStage())
    workflow.AddStage(NewLinkCheckStage())
    workflow.AddStage(NewStoreStage())
    workflow.AddStage(NewNotifyStage())

    return &AnalysisPipeline{workflow: workflow}
}

// Example AnalyzeStage implementation
type AnalyzeStage struct {
    logger Logger
}

type AnalysisResult struct {
    // HTML Structure
    HTMLVersion    string
    HeadingCounts  map[string]int  // h1, h2, h3, etc.
    FormCount      int

    // Media Resources
    Images         []ImageResource
    Videos         []VideoResource
    SVGs           []SVGResource

    // Stylesheets
    ExternalCSS    []StylesheetResource
    InlineStyles   int

    // Scripts
    ExternalJS     []ScriptResource
    InlineScripts  int

    // Links
    InternalLinks  []LinkResource
    ExternalLinks  []LinkResource

    // Metadata
    Title          string
    Description    string
    OpenGraph      map[string]string
    TwitterCards   map[string]string

    // Accessibility
    ImagesWithAlt  int
    ImagesWithoutAlt int
    ARIALabels     int
}

type ImageResource struct {
    URL         string
    Alt         string
    Width       string
    Height      string
    Loading     string  // lazy, eager
    Format      string  // jpg, png, webp, svg
}

type VideoResource struct {
    URL         string
    Sources     []string
    Poster      string
    Controls    bool
    Autoplay    bool
}

type SVGResource struct {
    Inline      bool
    URL         string
    Title       string
    Description string
}

type StylesheetResource struct {
    URL         string
    Media       string
    Integrity   string
    CrossOrigin string
}

type ScriptResource struct {
    URL         string
    Type        string  // module, text/javascript
    Async       bool
    Defer       bool
    Integrity   string
    CrossOrigin string
}

type LinkResource struct {
    URL         string
    Text        string
    Rel         string
    Title       string
    IsInternal  bool
}

func (s *AnalyzeStage) Execute(ctx context.Context, data *pipeline.DataTransfer) error {
    doc := data.Get("parsed_document").(*goquery.Document)
    result := &AnalysisResult{}

    // Analyze images
    doc.Find("img").Each(func(i int, sel *goquery.Selection) {
        src, _ := sel.Attr("src")
        alt, _ := sel.Attr("alt")
        width, _ := sel.Attr("width")
        height, _ := sel.Attr("height")
        loading, _ := sel.Attr("loading")

        result.Images = append(result.Images, ImageResource{
            URL:     src,
            Alt:     alt,
            Width:   width,
            Height:  height,
            Loading: loading,
            Format:  detectImageFormat(src),
        })

        if alt != "" {
            result.ImagesWithAlt++
        } else {
            result.ImagesWithoutAlt++
        }
    })

    // Analyze videos
    doc.Find("video").Each(func(i int, sel *goquery.Selection) {
        video := VideoResource{}
        if src, exists := sel.Attr("src"); exists {
            video.URL = src
        }
        if poster, exists := sel.Attr("poster"); exists {
            video.Poster = poster
        }
        video.Controls = sel.AttrOr("controls", "") != ""
        video.Autoplay = sel.AttrOr("autoplay", "") != ""

        sel.Find("source").Each(func(j int, source *goquery.Selection) {
            if src, exists := source.Attr("src"); exists {
                video.Sources = append(video.Sources, src)
            }
        })

        result.Videos = append(result.Videos, video)
    })

    // Analyze SVGs
    doc.Find("svg").Each(func(i int, sel *goquery.Selection) {
        result.SVGs = append(result.SVGs, SVGResource{
            Inline:      true,
            Title:       sel.Find("title").Text(),
            Description: sel.Find("desc").Text(),
        })
    })

    doc.Find("img[src$='.svg']").Each(func(i int, sel *goquery.Selection) {
        src, _ := sel.Attr("src")
        result.SVGs = append(result.SVGs, SVGResource{
            Inline: false,
            URL:    src,
        })
    })

    // Analyze stylesheets
    doc.Find("link[rel='stylesheet']").Each(func(i int, sel *goquery.Selection) {
        href, _ := sel.Attr("href")
        result.ExternalCSS = append(result.ExternalCSS, StylesheetResource{
            URL:         href,
            Media:       sel.AttrOr("media", "all"),
            Integrity:   sel.AttrOr("integrity", ""),
            CrossOrigin: sel.AttrOr("crossorigin", ""),
        })
    })

    result.InlineStyles = doc.Find("style").Length()

    // Analyze scripts
    doc.Find("script[src]").Each(func(i int, sel *goquery.Selection) {
        src, _ := sel.Attr("src")
        result.ExternalJS = append(result.ExternalJS, ScriptResource{
            URL:         src,
            Type:        sel.AttrOr("type", "text/javascript"),
            Async:       sel.AttrOr("async", "") != "",
            Defer:       sel.AttrOr("defer", "") != "",
            Integrity:   sel.AttrOr("integrity", ""),
            CrossOrigin: sel.AttrOr("crossorigin", ""),
        })
    })

    result.InlineScripts = doc.Find("script:not([src])").Length()

    data.Set("analysis_result", result)

    return nil
}

// Example ResourceCheckStage implementation
type ResourceCheckStage struct {
    httpClient *http.Client
    logger     Logger
    maxWorkers int
}

type ResourceStatus struct {
    URL        string
    Type       string  // image, video, css, js
    StatusCode int
    Available  bool
    Size       int64
    Error      error
}

func (s *ResourceCheckStage) Execute(ctx context.Context, data *pipeline.DataTransfer) error {
    result := data.Get("analysis_result").(*AnalysisResult)
    statuses := make([]ResourceStatus, 0)

    // Create worker pool for parallel resource checking
    resourceChan := make(chan string, 100)
    statusChan := make(chan ResourceStatus, 100)
    var wg sync.WaitGroup

    // Start workers
    for i := 0; i < s.maxWorkers; i++ {
        wg.Add(1)
        go s.checkResourceWorker(ctx, resourceChan, statusChan, &wg)
    }

    // Collect all URLs to check
    go func() {
        // Check images
        for _, img := range result.Images {
            if img.URL != "" {
                resourceChan <- fmt.Sprintf("image:%s", img.URL)
            }
        }

        // Check videos
        for _, video := range result.Videos {
            if video.URL != "" {
                resourceChan <- fmt.Sprintf("video:%s", video.URL)
            }
            for _, src := range video.Sources {
                resourceChan <- fmt.Sprintf("video:%s", src)
            }
        }

        // Check stylesheets
        for _, css := range result.ExternalCSS {
            if css.URL != "" {
                resourceChan <- fmt.Sprintf("css:%s", css.URL)
            }
        }

        // Check scripts
        for _, js := range result.ExternalJS {
            if js.URL != "" {
                resourceChan <- fmt.Sprintf("js:%s", js.URL)
            }
        }

        close(resourceChan)
    }()

    // Collect results
    go func() {
        wg.Wait()
        close(statusChan)
    }()

    for status := range statusChan {
        statuses = append(statuses, status)
    }

    data.Set("resource_statuses", statuses)

    return nil
}

func (s *ResourceCheckStage) checkResourceWorker(ctx context.Context, urls <-chan string, statuses chan<- ResourceStatus, wg *sync.WaitGroup) {
    defer wg.Done()

    for urlWithType := range urls {
        parts := strings.SplitN(urlWithType, ":", 2)
        if len(parts) != 2 {
            continue
        }

        resourceType := parts[0]
        url := parts[1]

        status := ResourceStatus{
            URL:  url,
            Type: resourceType,
        }

        req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
        if err != nil {
            status.Available = false
            status.Error = err
            statuses <- status
            continue
        }

        resp, err := s.httpClient.Do(req)
        if err != nil {
            status.Available = false
            status.Error = err
            statuses <- status
            continue
        }
        defer resp.Body.Close()

        status.StatusCode = resp.StatusCode
        status.Available = resp.StatusCode >= 200 && resp.StatusCode < 400
        status.Size = resp.ContentLength

        statuses <- status
    }
}
```

### Implementation Checklist

- [ ] Create domain models for all resource types (images, videos, SVGs, stylesheets, scripts)
- [ ] Implement FetchStage using existing web fetcher adapter
- [ ] Implement ParseStage using goquery for HTML parsing
- [ ] Implement AnalyzeStage with comprehensive resource extraction
- [ ] Implement ResourceCheckStage with parallel resource validation
- [ ] Implement LinkCheckStage for link validation
- [ ] Implement StoreStage to persist results to PostgreSQL
- [ ] Implement NotifyStage for SSE event emission
- [ ] Add database schema for storing resource analysis results
- [ ] Create migration for new analysis result tables
- [ ] Update OpenAPI specification with new analysis fields
- [ ] Add unit tests for each pipeline stage
- [ ] Add integration tests for complete pipeline flow
- [ ] Update subscriber service to use pipeline implementation

### Benefits
- **Modular processing stages**: Each stage has single responsibility
- **Parallel execution capability**: Leverage existing pipeline parallelism
- **Better error handling and recovery**: Stage-level error isolation
- **Easier testing of individual stages**: Unit test each stage independently
- **Reusable pipeline infrastructure**: Use existing `pkg/pipeline/` implementation
- **Workflow orchestration**: Leverage leader/follower pattern for complex flows

---

## 9. Kubernetes Deployment 🟢 **MEDIUM**

### Current State
- Only Docker Compose configuration exists
- No container orchestration for production
- No auto-scaling capabilities
- No GitOps workflow
- No infrastructure as code
- Limited observability

### Overview

Complete production-ready Kubernetes deployment stack:

- **Local Development**: Kind cluster with local registry
- **GitOps**: ArgoCD for declarative continuous delivery
- **Infrastructure as Code**: Crossplane for cloud-agnostic resource provisioning
- **Package Management**: Helm charts for application deployment
- **Service Mesh**: Istio for traffic management, security, and observability
- **Observability**: Full OTEL-based stack with Prometheus, Mimir, Loki, OpenSearch, Tempo, and Grafana
- **Ingress**: Traefik (maintaining compatibility with existing setup)

### Required Directory Structure

```yaml
k8s/
├── README.md                          # Deployment documentation
├── kind/                              # Kind cluster configurations
│   ├── kind-config.yaml              # Multi-node cluster with ingress
│   ├── registry.yaml                 # Local registry setup
│   └── setup.sh                      # Cluster creation script
├── argocd/                           # ArgoCD GitOps configurations
│   ├── install/                      # ArgoCD installation manifests
│   │   └── argocd-install.yaml
│   ├── projects/                     # ArgoCD projects
│   │   └── web-analyzer-project.yaml
│   ├── applications/                 # ArgoCD applications
│   │   ├── web-analyzer-app.yaml
│   │   ├── monitoring-app.yaml
│   │   ├── logging-app.yaml
│   │   └── crossplane-app.yaml
│   └── applicationsets/              # ApplicationSets for multi-env
│       └── web-analyzer-appset.yaml
├── crossplane/                       # Infrastructure as Code
│   ├── install/                      # Crossplane installation
│   │   ├── crossplane.yaml
│   │   └── providers.yaml
│   ├── compositions/                 # Composite resource definitions
│   │   ├── postgresql-composition.yaml
│   │   ├── redis-composition.yaml
│   │   ├── rabbitmq-composition.yaml
│   │   └── s3-composition.yaml
│   ├── claims/                       # Resource claims
│   │   ├── postgresql-claim.yaml
│   │   ├── redis-claim.yaml
│   │   ├── rabbitmq-claim.yaml
│   │   └── s3-claim.yaml
│   └── provider-configs/             # Provider configurations
│       ├── provider-aws.yaml
│       ├── provider-gcp.yaml
│       └── provider-azure.yaml
├── helm/                             # Helm charts
│   └── web-analyzer/                 # Main application chart
│       ├── Chart.yaml                # Chart metadata
│       ├── values.yaml               # Default values
│       ├── values-dev.yaml           # Development overrides
│       ├── values-staging.yaml       # Staging overrides
│       ├── values-prod.yaml          # Production overrides
│       ├── templates/                # Kubernetes manifests
│       │   ├── _helpers.tpl         # Template helpers
│       │   ├── namespace.yaml
│       │   ├── configmap.yaml
│       │   ├── secrets.yaml
│       │   ├── api-deployment.yaml
│       │   ├── api-service.yaml
│       │   ├── api-hpa.yaml
│       │   ├── publisher-deployment.yaml
│       │   ├── publisher-service.yaml
│       │   ├── publisher-hpa.yaml
│       │   ├── subscriber-deployment.yaml
│       │   ├── subscriber-service.yaml
│       │   ├── subscriber-hpa.yaml
│       │   ├── ingress.yaml
│       │   ├── networkpolicies.yaml
│       │   ├── servicemonitor.yaml  # Prometheus monitoring
│       │   ├── destinationrule.yaml # Istio traffic policy
│       │   ├── virtualservice.yaml  # Istio routing
│       │   └── peerauthentication.yaml # Istio mTLS
│       └── charts/                   # Subchart dependencies
├── istio/                            # Istio service mesh
│   ├── install/                      # Istio installation
│   │   ├── istio-operator.yaml
│   │   └── istio-profile.yaml
│   ├── gateway.yaml                  # Istio ingress gateway
│   ├── telemetry.yaml               # Observability config
│   ├── peerauthentication.yaml      # Global mTLS policy
│   └── authorizationpolicy.yaml     # RBAC policies
├── monitoring/                       # Observability stack
│   ├── otel-collector/              # OpenTelemetry Collector
│   │   ├── configmap.yaml           # OTEL config
│   │   ├── deployment.yaml
│   │   └── service.yaml
│   ├── prometheus/                  # Short-term metrics
│   │   ├── prometheus-operator.yaml
│   │   ├── prometheus.yaml
│   │   ├── servicemonitor.yaml
│   │   └── rules/
│   │       ├── alerts.yaml
│   │       └── recording-rules.yaml
│   ├── mimir/                       # Long-term metrics storage
│   │   ├── mimir-distributed.yaml   # Helm values
│   │   └── s3-config.yaml           # Object storage config
│   ├── loki/                        # Operational logs
│   │   ├── loki-stack.yaml          # Helm values
│   │   ├── promtail-configmap.yaml
│   │   └── retention-policy.yaml
│   ├── opensearch/                  # Compliance logs
│   │   ├── opensearch-cluster.yaml
│   │   ├── dashboards.yaml
│   │   └── index-templates/
│   │       ├── audit-logs.yaml
│   │       └── compliance-logs.yaml
│   ├── tempo/                       # Distributed tracing
│   │   ├── tempo.yaml               # Helm values
│   │   └── ingester-config.yaml
│   └── grafana/                     # Unified visualization
│       ├── grafana.yaml             # Helm values
│       ├── datasources.yaml         # Data sources config
│       └── dashboards/
│           ├── web-analyzer-overview.json
│           ├── api-metrics.json
│           ├── publisher-metrics.json
│           ├── subscriber-metrics.json
│           ├── istio-mesh.json
│           └── kubernetes-cluster.json
├── traefik/                         # Traefik ingress controller
│   ├── traefik-values.yaml          # Helm chart values
│   ├── middleware.yaml              # HTTP middleware
│   └── ingressroute.yaml            # Traefik routing
├── security/                        # Security scanning and policies
│   ├── trivy/                       # Trivy vulnerability scanning
│   │   ├── trivy-operator.yaml      # Trivy Operator installation
│   │   ├── trivy-config.yaml        # Scanner configuration
│   │   ├── scan-policies.yaml       # Scan policies and schedules
│   │   └── reports/                 # Vulnerability reports
│   ├── policies/                    # Security policies
│   │   ├── pod-security-policies.yaml
│   │   ├── network-policies.yaml
│   │   └── admission-policies.yaml
│   └── rbac/                        # RBAC configurations
│       ├── roles.yaml
│       └── rolebindings.yaml
└── base/                            # Base Kubernetes resources
    ├── namespaces.yaml
    ├── priorityclasses.yaml
    ├── resourcequotas.yaml
    └── limitranges.yaml
```

---

## 9.5. Trivy Security Scanning

### Overview

Integrate Trivy for comprehensive security scanning of container images, Kubernetes configurations, Infrastructure as Code (IaC), and running workloads. Trivy provides:

- **Container Image Scanning**: Detect vulnerabilities in Docker images
- **Kubernetes Configuration Scanning**: Identify misconfigurations in manifests
- **IaC Scanning**: Scan Helm charts, Terraform, and Crossplane configurations
- **SBOM Generation**: Software Bill of Materials for compliance
- **Continuous Monitoring**: Runtime vulnerability detection with Trivy Operator

### Trivy Operator Installation

```yaml
# k8s/security/trivy/trivy-operator.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: trivy-system
---
# Install using Helm
# helm repo add aqua https://aquasecurity.github.io/helm-charts/
# helm repo update
# helm install trivy-operator aqua/trivy-operator \
#   --namespace trivy-system \
#   --create-namespace \
#   --version 0.20.0

apiVersion: v1
kind: ConfigMap
metadata:
  name: trivy-operator-config
  namespace: trivy-system
data:
  # Scanner configuration
  trivy.severity: "CRITICAL,HIGH,MEDIUM"
  trivy.ignoreUnfixed: "false"
  trivy.timeout: "5m"
  trivy.dbRepository: "ghcr.io/aquasecurity/trivy-db"
  trivy.javaDbRepository: "ghcr.io/aquasecurity/trivy-java-db"

  # Vulnerability reporting
  vulnerabilityReports.scanner: "Trivy"

  # SBOM generation
  trivy.sbomSources: "oci,rekor"

  # Resource limits
  trivy.resources.requests.cpu: "100m"
  trivy.resources.requests.memory: "100Mi"
  trivy.resources.limits.cpu: "500m"
  trivy.resources.limits.memory: "500Mi"

  # Scanning scope
  compliance.failEntriesLimit: "10"
  scanJob.tolerations: |
    - key: node-role.kubernetes.io/master
      effect: NoSchedule
---
apiVersion: v1
kind: Secret
metadata:
  name: trivy-operator-private-registry
  namespace: trivy-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <base64-encoded-docker-config>
```

### Trivy Configuration

```yaml
# k8s/security/trivy/trivy-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: trivy-custom-policies
  namespace: trivy-system
data:
  # Custom vulnerability exclusions
  trivyignore: |
    # Ignore specific CVEs with justification
    # CVE-2023-12345  # Fixed in next release, low risk

  # Scanner options
  scanner-config.yaml: |
    cache:
      backend: redis
      redis:
        addr: redis:6379
        db: 0

    vulnerability:
      type:
        - os
        - library
      scanners:
        - vuln
        - secret
        - config

    secret:
      config: /etc/trivy/secret-patterns.yaml

    severity:
      - CRITICAL
      - HIGH
      - MEDIUM

    output:
      format: json
      template: "@/etc/trivy/html.tpl"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: trivy-secret-patterns
  namespace: trivy-system
data:
  secret-patterns.yaml: |
    rules:
      - id: aws-access-key
        category: AWS
        title: AWS Access Key
        regex: "(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}"

      - id: github-token
        category: GitHub
        title: GitHub Token
        regex: "ghp_[0-9a-zA-Z]{36}"

      - id: private-key
        category: Crypto
        title: Private Key
        regex: "-----BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY-----"
```

### Scan Policies and Schedules

```yaml
# k8s/security/trivy/scan-policies.yaml
apiVersion: aquasecurity.github.io/v1alpha1
kind: ClusterComplianceReport
metadata:
  name: web-analyzer-compliance
spec:
  reportType: summary
  compliance:
    id: nsa-cisa-v1.0
  schedule: "0 */6 * * *"  # Every 6 hours
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: trivy-image-scan
  namespace: trivy-system
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: trivy
            image: aquasec/trivy:latest
            args:
              - image
              - --format
              - json
              - --output
              - /reports/image-scan-report.json
              - --severity
              - CRITICAL,HIGH,MEDIUM
              - --ignore-unfixed
              - localhost:5001/web-analyzer/api:latest
            volumeMounts:
              - name: reports
                mountPath: /reports
          volumes:
            - name: reports
              persistentVolumeClaim:
                claimName: trivy-reports
          restartPolicy: OnFailure
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: trivy-k8s-scan
  namespace: trivy-system
spec:
  schedule: "0 3 * * *"  # Daily at 3 AM
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: trivy-scanner
          containers:
          - name: trivy
            image: aquasec/trivy:latest
            args:
              - k8s
              - --report
              - summary
              - --severity
              - CRITICAL,HIGH,MEDIUM
              - --output
              - /reports/k8s-scan-report.json
              - --format
              - json
              - cluster
            volumeMounts:
              - name: reports
                mountPath: /reports
          volumes:
            - name: reports
              persistentVolumeClaim:
                claimName: trivy-reports
          restartPolicy: OnFailure
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: trivy-reports
  namespace: trivy-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: standard
```

### RBAC for Trivy Scanner

```yaml
# k8s/security/trivy/rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: trivy-scanner
  namespace: trivy-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: trivy-scanner
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
    verbs: ["get", "list"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list"]
  - apiGroups: ["aquasecurity.github.io"]
    resources: ["vulnerabilityreports", "configauditreports", "clusterconfigauditreports"]
    verbs: ["get", "list", "create", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: trivy-scanner
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: trivy-scanner
subjects:
  - kind: ServiceAccount
    name: trivy-scanner
    namespace: trivy-system
```

### CI/CD Integration

```yaml
# .github/workflows/trivy-scan.yml
name: Trivy Security Scan

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 0 * * *'  # Daily scan

jobs:
  image-scan:
    name: Scan Docker Images
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Build image
        run: |
          docker build -t web-analyzer:${{ github.sha }} \
            -f deployments/docker/Dockerfile .

      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: web-analyzer:${{ github.sha }}
          format: 'sarif'
          output: 'trivy-results.sarif'
          severity: 'CRITICAL,HIGH,MEDIUM'
          exit-code: '1'  # Fail on vulnerabilities

      - name: Upload Trivy results to GitHub Security
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: 'trivy-results.sarif'

      - name: Generate SBOM
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: web-analyzer:${{ github.sha }}
          format: 'cyclonedx'
          output: 'sbom.json'

      - name: Upload SBOM
        uses: actions/upload-artifact@v4
        with:
          name: sbom
          path: sbom.json

  k8s-manifest-scan:
    name: Scan Kubernetes Manifests
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Run Trivy config scanner
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'config'
          scan-ref: 'k8s/'
          format: 'sarif'
          output: 'trivy-k8s-results.sarif'
          severity: 'CRITICAL,HIGH,MEDIUM'
          exit-code: '1'

      - name: Upload K8s scan results
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: 'trivy-k8s-results.sarif'

  iac-scan:
    name: Scan Infrastructure as Code
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Run Trivy IaC scanner
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'config'
          scan-ref: '.'
          format: 'table'
          severity: 'CRITICAL,HIGH,MEDIUM'
          skip-dirs: 'vendor,node_modules'
          exit-code: '1'
```

### Makefile Integration

```makefile
# Add to Makefile
.PHONY: trivy-scan
trivy-scan: ## Run Trivy security scans
	@echo "🔒 Running Trivy security scans..."
	@$(MAKE) trivy-image-scan
	@$(MAKE) trivy-k8s-scan
	@$(MAKE) trivy-iac-scan

.PHONY: trivy-image-scan
trivy-image-scan: ## Scan Docker images for vulnerabilities
	@echo "🐳 Scanning Docker images..."
	@trivy image --severity CRITICAL,HIGH,MEDIUM \
		--ignore-unfixed \
		--format table \
		localhost:5001/web-analyzer/api:latest
	@trivy image --severity CRITICAL,HIGH,MEDIUM \
		--ignore-unfixed \
		--format table \
		localhost:5001/web-analyzer/publisher:latest
	@trivy image --severity CRITICAL,HIGH,MEDIUM \
		--ignore-unfixed \
		--format table \
		localhost:5001/web-analyzer/subscriber:latest

.PHONY: trivy-k8s-scan
trivy-k8s-scan: ## Scan Kubernetes manifests
	@echo "☸️  Scanning Kubernetes manifests..."
	@trivy config --severity CRITICAL,HIGH,MEDIUM \
		--format table \
		k8s/

.PHONY: trivy-iac-scan
trivy-iac-scan: ## Scan Infrastructure as Code
	@echo "📋 Scanning Infrastructure as Code..."
	@trivy config --severity CRITICAL,HIGH,MEDIUM \
		--format table \
		--skip-dirs vendor,node_modules \
		.

.PHONY: trivy-sbom
trivy-sbom: ## Generate Software Bill of Materials
	@echo "📦 Generating SBOM..."
	@trivy image --format cyclonedx \
		--output sbom.json \
		localhost:5001/web-analyzer/api:latest

.PHONY: trivy-report
trivy-report: ## Generate comprehensive security report
	@echo "📊 Generating security report..."
	@mkdir -p reports
	@trivy image --format json \
		--output reports/image-vulnerabilities.json \
		localhost:5001/web-analyzer/api:latest
	@trivy k8s --report summary \
		--format json \
		--output reports/k8s-audit.json \
		cluster
```

### Monitoring and Alerting

```yaml
# k8s/monitoring/prometheus/rules/trivy-alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: trivy-vulnerability-alerts
  namespace: monitoring
spec:
  groups:
    - name: trivy.vulnerabilities
      interval: 5m
      rules:
        - alert: CriticalVulnerabilitiesDetected
          expr: |
            sum(trivy_vulnerability_id{severity="CRITICAL"}) by (namespace, pod) > 0
          for: 5m
          labels:
            severity: critical
            team: security
          annotations:
            summary: "Critical vulnerabilities detected in {{ $labels.namespace }}/{{ $labels.pod }}"
            description: "Pod {{ $labels.pod }} in namespace {{ $labels.namespace }} has critical vulnerabilities"

        - alert: HighVulnerabilitiesDetected
          expr: |
            sum(trivy_vulnerability_id{severity="HIGH"}) by (namespace, pod) > 5
          for: 15m
          labels:
            severity: warning
            team: security
          annotations:
            summary: "High vulnerabilities detected in {{ $labels.namespace }}/{{ $labels.pod }}"
            description: "Pod {{ $labels.pod }} has {{ $value }} high severity vulnerabilities"

        - alert: MisconfigurationDetected
          expr: |
            sum(trivy_resource_configaudits{status="fail"}) by (namespace) > 0
          for: 10m
          labels:
            severity: warning
            team: platform
          annotations:
            summary: "Misconfigurations detected in {{ $labels.namespace }}"
            description: "Namespace {{ $labels.namespace }} has {{ $value }} failed configuration audits"
```

### Grafana Dashboard for Trivy

```json
// k8s/monitoring/grafana/dashboards/trivy-security.json
{
  "dashboard": {
    "title": "Trivy Security Dashboard",
    "panels": [
      {
        "title": "Critical Vulnerabilities",
        "targets": [
          {
            "expr": "sum(trivy_vulnerability_id{severity=\"CRITICAL\"}) by (namespace)"
          }
        ]
      },
      {
        "title": "Vulnerability Trend",
        "targets": [
          {
            "expr": "sum(trivy_vulnerability_id) by (severity)"
          }
        ]
      },
      {
        "title": "Configuration Audit Status",
        "targets": [
          {
            "expr": "sum(trivy_resource_configaudits) by (status)"
          }
        ]
      }
    ]
  }
}
```

### Implementation Checklist

- [ ] Install Trivy CLI locally for development
- [ ] Set up Trivy Operator in Kubernetes cluster
- [ ] Configure automated image scanning on push
- [ ] Integrate Trivy scans in CI/CD pipeline
- [ ] Set up scheduled scans for running workloads
- [ ] Configure vulnerability report storage
- [ ] Create Prometheus alerts for critical vulnerabilities
- [ ] Set up Grafana dashboards for security metrics
- [ ] Generate and track SBOM for compliance
- [ ] Configure custom vulnerability exclusions (.trivyignore)
- [ ] Set up secret scanning patterns
- [ ] Integrate with GitHub Security tab (SARIF upload)
- [ ] Document remediation procedures
- [ ] Train team on Trivy usage and report interpretation

### Benefits

- **Comprehensive Vulnerability Detection**: Scan images, configs, and IaC
- **Continuous Monitoring**: Runtime vulnerability detection with operator
- **Compliance**: SBOM generation and compliance reporting
- **Early Detection**: Find vulnerabilities in CI/CD before deployment
- **Automated Remediation**: Integration with dependency updates
- **Cost-Effective**: Free, open-source security scanning
- **Kubernetes-Native**: Deep integration with K8s resources

---

## 9.1. Kind Setup (Local Development)

### Kind Cluster Configuration

```yaml
# k8s/kind/kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: web-analyzer-local

# Multi-node cluster for testing HA
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
      - containerPort: 15021  # Istio health check
        hostPort: 15021
        protocol: TCP
  - role: worker
  - role: worker

# Enable feature gates
featureGates:
  "EphemeralContainers": true
  "CSIStorageCapacity": true

# Networking configuration
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 6443
  podSubnet: "10.244.0.0/16"
  serviceSubnet: "10.96.0.0/12"

# Container runtime configuration
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:5001"]
      endpoint = ["http://kind-registry:5000"]
```

### Local Registry Setup

```yaml
# k8s/kind/registry.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5001"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
```

### Cluster Creation Script

```bash
# k8s/kind/setup.sh
#!/bin/bash
set -euo pipefail

CLUSTER_NAME="web-analyzer-local"
REGISTRY_NAME="kind-registry"
REGISTRY_PORT="5001"

echo "🚀 Creating Kind cluster with local registry..."

# Create registry container if it doesn't exist
if [ "$(docker inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)" != 'true' ]; then
  echo "📦 Creating local registry..."
  docker run -d --restart=always \
    -p "127.0.0.1:${REGISTRY_PORT}:5000" \
    --name "${REGISTRY_NAME}" \
    registry:2
fi

# Create Kind cluster
echo "🎯 Creating Kind cluster..."
kind create cluster --config=kind-config.yaml

# Connect registry to cluster network
echo "🔗 Connecting registry to cluster network..."
docker network connect "kind" "${REGISTRY_NAME}" 2>/dev/null || true

# Document local registry
echo "📝 Documenting local registry..."
kubectl apply -f registry.yaml

# Install MetalLB for LoadBalancer support
echo "⚖️  Installing MetalLB..."
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml
kubectl wait --namespace metallb-system \
  --for=condition=ready pod \
  --selector=app=metallb \
  --timeout=90s

# Configure MetalLB IP pool
SUBNET=$(docker network inspect kind -f '{{(index .IPAM.Config 0).Subnet}}')
IP_PREFIX=$(echo ${SUBNET} | sed 's/\.[0-9]*\/.*/./')
cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - ${IP_PREFIX}200-${IP_PREFIX}250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
  - default
EOF

echo "✅ Kind cluster '${CLUSTER_NAME}' created successfully!"
echo "📌 Registry available at localhost:${REGISTRY_PORT}"
echo ""
echo "Next steps:"
echo "  1. Install ArgoCD: kubectl apply -k k8s/argocd/install/"
echo "  2. Install Crossplane: kubectl apply -k k8s/crossplane/install/"
echo "  3. Install Istio: istioctl install --set profile=demo -y"
echo "  4. Deploy monitoring stack: kubectl apply -k k8s/monitoring/"
```

### Usage Commands

```bash
# Create cluster
cd k8s/kind
chmod +x setup.sh
./setup.sh

# Build and push images to local registry
docker build -t localhost:5001/web-analyzer/api:latest -f deployments/docker/Dockerfile .
docker push localhost:5001/web-analyzer/api:latest

# Delete cluster
kind delete cluster --name web-analyzer-local

# Export kubeconfig
kind export kubeconfig --name web-analyzer-local
```

---

## 9.2. ArgoCD Installation & Configuration

### ArgoCD Installation

```yaml
# k8s/argocd/install/argocd-install.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: argocd
---
# Install ArgoCD using official manifests
# kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Custom configuration for local development
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  # Enable local development mode
  server.insecure: "true"

  # Git repository access
  repositories: |
    - url: https://github.com/architeacher/svc-web-analyzer.git
      name: web-analyzer-repo

  # Resource customizations
  resource.customizations: |
    argoproj.io/Application:
      health.lua: |
        hs = {}
        hs.status = "Progressing"
        hs.message = ""
        if obj.status ~= nil then
          if obj.status.health ~= nil then
            hs.status = obj.status.health.status
            if obj.status.health.message ~= nil then
              hs.message = obj.status.health.message
            end
          end
        end
        return hs
```

### ArgoCD Project

```yaml
# k8s/argocd/projects/web-analyzer-project.yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: web-analyzer
  namespace: argocd
spec:
  description: Web Analyzer Service Project

  # Source repositories
  sourceRepos:
    - 'https://github.com/architeacher/svc-web-analyzer.git'
    - 'https://charts.bitnami.com/bitnami'
    - 'https://grafana.github.io/helm-charts'
    - 'https://prometheus-community.github.io/helm-charts'
    - 'https://opensearch-project.github.io/helm-charts'

  # Allowed destinations
  destinations:
    - namespace: 'web-analyzer-*'
      server: https://kubernetes.default.svc
    - namespace: 'monitoring'
      server: https://kubernetes.default.svc
    - namespace: 'logging'
      server: https://kubernetes.default.svc
    - namespace: 'istio-system'
      server: https://kubernetes.default.svc

  # Cluster resource whitelist
  clusterResourceWhitelist:
    - group: '*'
      kind: '*'

  # Namespace resource whitelist
  namespaceResourceWhitelist:
    - group: '*'
      kind: '*'

  # Roles for RBAC
  roles:
    - name: developer
      description: Developer access
      policies:
        - p, proj:web-analyzer:developer, applications, get, web-analyzer/*, allow
        - p, proj:web-analyzer:developer, applications, sync, web-analyzer/*, allow
      groups:
        - web-analyzer-developers

    - name: operator
      description: Operator access with full control
      policies:
        - p, proj:web-analyzer:operator, applications, *, web-analyzer/*, allow
      groups:
        - web-analyzer-operators
```

### ArgoCD Application

```yaml
# k8s/argocd/applications/web-analyzer-app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: web-analyzer
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: web-analyzer

  # Source configuration
  source:
    repoURL: https://github.com/architeacher/svc-web-analyzer.git
    targetRevision: HEAD
    path: k8s/helm/web-analyzer

    # Helm configuration
    helm:
      releaseName: web-analyzer
      valueFiles:
        - values.yaml
        - values-dev.yaml

      # Override values
      parameters:
        - name: image.tag
          value: latest
        - name: replicaCount.api
          value: "2"
        - name: replicaCount.publisher
          value: "1"
        - name: replicaCount.subscriber
          value: "2"

  # Destination cluster
  destination:
    server: https://kubernetes.default.svc
    namespace: web-analyzer-dev

  # Sync policy
  syncPolicy:
    automated:
      prune: true        # Delete resources not in Git
      selfHeal: true     # Force sync when drift detected
      allowEmpty: false  # Prevent deletion of all resources

    syncOptions:
      - CreateNamespace=true
      - PrunePropagationPolicy=foreground
      - PruneLast=true

    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m

  # Ignore differences
  ignoreDifferences:
    - group: apps
      kind: Deployment
      jsonPointers:
        - /spec/replicas  # Ignore HPA-managed replicas

  # Health assessment
  revisionHistoryLimit: 10
```

### ApplicationSet for Multi-Environment

```yaml
# k8s/argocd/applicationsets/web-analyzer-appset.yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: web-analyzer-environments
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - env: dev
            namespace: web-analyzer-dev
            replicas: "2"
            autoSync: "true"
          - env: staging
            namespace: web-analyzer-staging
            replicas: "3"
            autoSync: "true"
          - env: prod
            namespace: web-analyzer-prod
            replicas: "5"
            autoSync: "false"  # Manual approval for prod

  template:
    metadata:
      name: 'web-analyzer-{{env}}'
      namespace: argocd
    spec:
      project: web-analyzer
      source:
        repoURL: https://github.com/architeacher/svc-web-analyzer.git
        targetRevision: HEAD
        path: k8s/helm/web-analyzer
        helm:
          releaseName: web-analyzer
          valueFiles:
            - values.yaml
            - 'values-{{env}}.yaml'
          parameters:
            - name: replicaCount.api
              value: '{{replicas}}'
      destination:
        server: https://kubernetes.default.svc
        namespace: '{{namespace}}'
      syncPolicy:
        automated:
          prune: true
          selfHeal: '{{autoSync}}'
        syncOptions:
          - CreateNamespace=true
```

### ArgoCD CLI Commands

```bash
# Install ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Get initial admin password
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d

# Port forward to access UI
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Login with CLI
argocd login localhost:8080

# Create project and applications
kubectl apply -f k8s/argocd/projects/
kubectl apply -f k8s/argocd/applications/

# Sync application
argocd app sync web-analyzer

# Watch sync status
argocd app wait web-analyzer --health
```

---

## 9.3. Crossplane Setup (Provider-Agnostic Infrastructure)

### Crossplane Installation

```yaml
# k8s/crossplane/install/crossplane.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: crossplane-system
---
# Install using Helm
# helm repo add crossplane-stable https://charts.crossplane.io/stable
# helm repo update
# helm install crossplane --namespace crossplane-system crossplane-stable/crossplane

apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-aws
spec:
  package: crossplaneio/provider-aws:v0.47.0
  packagePullPolicy: IfNotPresent
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-gcp
spec:
  package: crossplaneio/provider-gcp:v0.28.0
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-azure
spec:
  package: crossplaneio/provider-azure:v0.19.0
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-helm
spec:
  package: crossplaneio/provider-helm:v0.18.0
---
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-kubernetes
spec:
  package: crossplaneio/provider-kubernetes:v0.14.0
```

### PostgreSQL Composition (Provider-Agnostic)

```yaml
# k8s/crossplane/compositions/postgresql-composition.yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xpostgresqlinstances.database.webanalyzer.dev
spec:
  group: database.webanalyzer.dev
  names:
    kind: XPostgreSQLInstance
    plural: xpostgresqlinstances
  claimNames:
    kind: PostgreSQLInstance
    plural: postgresqlinstances
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                parameters:
                  type: object
                  properties:
                    size:
                      type: string
                      enum: [small, medium, large]
                    version:
                      type: string
                      default: "16"
                    storageGB:
                      type: integer
                      default: 20
                    provider:
                      type: string
                      enum: [aws, gcp, azure, local]
                      default: local
                  required:
                    - size
              required:
                - parameters
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: postgresql-local
  labels:
    provider: local
spec:
  compositeTypeRef:
    apiVersion: database.webanalyzer.dev/v1alpha1
    kind: XPostgreSQLInstance

  resources:
    # Deploy PostgreSQL using Helm provider
    - name: postgresql-helm-release
      base:
        apiVersion: helm.crossplane.io/v1beta1
        kind: Release
        spec:
          forProvider:
            chart:
              name: postgresql
              repository: https://charts.bitnami.com/bitnami
              version: "15.2.0"
            namespace: web-analyzer-dev
            values:
              auth:
                database: web_analyzer
                username: postgres
              primary:
                persistence:
                  size: 10Gi
              metrics:
                enabled: true
                serviceMonitor:
                  enabled: true
      patches:
        - type: FromCompositeFieldPath
          fromFieldPath: spec.parameters.storageGB
          toFieldPath: spec.forProvider.values.primary.persistence.size
          transforms:
            - type: string
              string:
                fmt: "%dGi"
        - type: FromCompositeFieldPath
          fromFieldPath: spec.parameters.version
          toFieldPath: spec.forProvider.values.image.tag
        - type: FromCompositeFieldPath
          fromFieldPath: spec.parameters.size
          toFieldPath: spec.forProvider.values.primary.resources
          transforms:
            - type: map
              map:
                small:
                  requests:
                    cpu: 250m
                    memory: 512Mi
                  limits:
                    cpu: 500m
                    memory: 1Gi
                medium:
                  requests:
                    cpu: 500m
                    memory: 1Gi
                  limits:
                    cpu: 1000m
                    memory: 2Gi
                large:
                  requests:
                    cpu: 1000m
                    memory: 2Gi
                  limits:
                    cpu: 2000m
                    memory: 4Gi

    # Create secret with connection details
    - name: postgresql-secret
      base:
        apiVersion: kubernetes.crossplane.io/v1alpha1
        kind: Object
        spec:
          forProvider:
            manifest:
              apiVersion: v1
              kind: Secret
              metadata:
                namespace: web-analyzer-dev
              type: Opaque
      connectionDetails:
        - fromConnectionSecretKey: postgresql-password
```

### Redis Composition

```yaml
# k8s/crossplane/compositions/redis-composition.yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xredisinstances.cache.webanalyzer.dev
spec:
  group: cache.webanalyzer.dev
  names:
    kind: XRedisInstance
    plural: xredisinstances
  claimNames:
    kind: RedisInstance
    plural: redisinstances
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                parameters:
                  type: object
                  properties:
                    size:
                      type: string
                      enum: [small, medium, large]
                    persistence:
                      type: boolean
                      default: true
                  required:
                    - size
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: redis-local
spec:
  compositeTypeRef:
    apiVersion: cache.webanalyzer.dev/v1alpha1
    kind: XRedisInstance

  resources:
    - name: redis-helm-release
      base:
        apiVersion: helm.crossplane.io/v1beta1
        kind: Release
        spec:
          forProvider:
            chart:
              name: redis
              repository: https://charts.bitnami.com/bitnami
              version: "19.0.0"
            namespace: web-analyzer-dev
            values:
              architecture: standalone
              auth:
                enabled: true
              metrics:
                enabled: true
                serviceMonitor:
                  enabled: true
      patches:
        - type: FromCompositeFieldPath
          fromFieldPath: spec.parameters.persistence
          toFieldPath: spec.forProvider.values.master.persistence.enabled
```

### RabbitMQ Composition

```yaml
# k8s/crossplane/compositions/rabbitmq-composition.yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xrabbitmqinstances.messaging.webanalyzer.dev
spec:
  group: messaging.webanalyzer.dev
  names:
    kind: XRabbitMQInstance
    plural: xrabbitmqinstances
  claimNames:
    kind: RabbitMQInstance
    plural: rabbitmqinstances
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                parameters:
                  type: object
                  properties:
                    replicas:
                      type: integer
                      default: 1
                    storageGB:
                      type: integer
                      default: 10
---
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: rabbitmq-local
spec:
  compositeTypeRef:
    apiVersion: messaging.webanalyzer.dev/v1alpha1
    kind: XRabbitMQInstance

  resources:
    - name: rabbitmq-helm-release
      base:
        apiVersion: helm.crossplane.io/v1beta1
        kind: Release
        spec:
          forProvider:
            chart:
              name: rabbitmq
              repository: https://charts.bitnami.com/bitnami
              version: "14.0.0"
            namespace: web-analyzer-dev
            values:
              auth:
                username: admin
              metrics:
                enabled: true
                serviceMonitor:
                  enabled: true
              persistence:
                enabled: true
      patches:
        - type: FromCompositeFieldPath
          fromFieldPath: spec.parameters.replicas
          toFieldPath: spec.forProvider.values.replicaCount
        - type: FromCompositeFieldPath
          fromFieldPath: spec.parameters.storageGB
          toFieldPath: spec.forProvider.values.persistence.size
          transforms:
            - type: string
              string:
                fmt: "%dGi"
```

### Resource Claims

```yaml
# k8s/crossplane/claims/postgresql-claim.yaml
apiVersion: database.webanalyzer.dev/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: web-analyzer-db
  namespace: web-analyzer-dev
spec:
  parameters:
    size: medium
    version: "16"
    storageGB: 50
    provider: local
  compositionSelector:
    matchLabels:
      provider: local
  writeConnectionSecretToRef:
    name: postgres-connection
---
# k8s/crossplane/claims/redis-claim.yaml
apiVersion: cache.webanalyzer.dev/v1alpha1
kind: RedisInstance
metadata:
  name: web-analyzer-cache
  namespace: web-analyzer-dev
spec:
  parameters:
    size: small
    persistence: true
  writeConnectionSecretToRef:
    name: redis-connection
---
# k8s/crossplane/claims/rabbitmq-claim.yaml
apiVersion: messaging.webanalyzer.dev/v1alpha1
kind: RabbitMQInstance
metadata:
  name: web-analyzer-queue
  namespace: web-analyzer-dev
spec:
  parameters:
    replicas: 1
    storageGB: 20
  writeConnectionSecretToRef:
    name: rabbitmq-connection
```

---

## 9.4. Helm Charts for Services

### Chart.yaml

```yaml
# k8s/helm/web-analyzer/Chart.yaml
apiVersion: v2
name: web-analyzer
description: A Helm chart for Web Analyzer Service
type: application
version: 1.0.0
appVersion: "1.0.0"
keywords:
  - web-analyzer
  - analysis
  - microservices
maintainers:
  - name: Web Analyzer Team
    email: team@webanalyzer.dev
dependencies:
  - name: postgresql
    version: "15.2.0"
    repository: https://charts.bitnami.com/bitnami
    condition: postgresql.enabled
  - name: redis
    version: "19.0.0"
    repository: https://charts.bitnami.com/bitnami
    condition: redis.enabled
  - name: rabbitmq
    version: "14.0.0"
    repository: https://charts.bitnami.com/bitnami
    condition: rabbitmq.enabled
```

### values.yaml (Default Values)

```yaml
# k8s/helm/web-analyzer/values.yaml
# Global settings
global:
  storageClass: "standard"
  imagePullSecrets: []

# Image configuration
image:
  registry: localhost:5001
  repository: web-analyzer
  pullPolicy: IfNotPresent
  tag: "latest"

# Service-specific images
services:
  api:
    image: api
  publisher:
    image: publisher
  subscriber:
    image: subscriber

# Replica counts
replicaCount:
  api: 2
  publisher: 1
  subscriber: 2

# Resource limits
resources:
  api:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi
  publisher:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi
  subscriber:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi

# Horizontal Pod Autoscaling
autoscaling:
  api:
    enabled: true
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80
  publisher:
    enabled: false
  subscriber:
    enabled: true
    minReplicas: 2
    maxReplicas: 20
    targetCPUUtilizationPercentage: 75

# Service configuration
service:
  type: ClusterIP
  api:
    port: 8080
    targetPort: 8080
  publisher:
    port: 8081
  subscriber:
    port: 8082

# Ingress configuration
ingress:
  enabled: true
  className: traefik
  annotations:
    traefik.ingress.kubernetes.io/router.tls: "true"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: api.web-analyzer.dev
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: web-analyzer-tls
      hosts:
        - api.web-analyzer.dev

# Environment variables
env:
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
  OTEL_ENABLED: "true"
  OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector.monitoring:4317"

# Database configuration (from Crossplane)
database:
  existingSecret: "postgres-connection"
  secretKeys:
    host: "host"
    port: "port"
    database: "database"
    username: "username"
    password: "password"

# Cache configuration
redis:
  enabled: false  # Managed by Crossplane
  existingSecret: "redis-connection"

# Message queue configuration
rabbitmq:
  enabled: false  # Managed by Crossplane
  existingSecret: "rabbitmq-connection"

# Health checks
livenessProbe:
  httpGet:
    path: /health
    port: http
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health
    port: http
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3

# Pod disruption budget
podDisruptionBudget:
  enabled: true
  minAvailable: 1

# Security context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  capabilities:
    drop:
      - ALL

# Service mesh (Istio)
istio:
  enabled: true
  virtualService:
    enabled: true
    gateways:
      - istio-system/web-analyzer-gateway
  destinationRule:
    enabled: true
    trafficPolicy:
      connectionPool:
        tcp:
          maxConnections: 100
        http:
          http1MaxPendingRequests: 100
          http2MaxRequests: 100
      outlierDetection:
        consecutive5xxErrors: 5
        interval: 30s
        baseEjectionTime: 30s

# Monitoring
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
    scrapeTimeout: 10s

# Network policies
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
```

### API Deployment Template

```yaml
# k8s/helm/web-analyzer/templates/api-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "web-analyzer.fullname" . }}-api
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "web-analyzer.labels" . | nindent 4 }}
    app.kubernetes.io/component: api
    version: {{ .Chart.AppVersion }}
spec:
  replicas: {{ .Values.replicaCount.api }}
  selector:
    matchLabels:
      {{- include "web-analyzer.selectorLabels" . | nindent 6 }}
      app.kubernetes.io/component: api
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
        sidecar.istio.io/inject: "{{ .Values.istio.enabled }}"
      labels:
        {{- include "web-analyzer.selectorLabels" . | nindent 8 }}
        app.kubernetes.io/component: api
        version: {{ .Chart.AppVersion }}
    spec:
      serviceAccountName: {{ include "web-analyzer.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.securityContext | nindent 8 }}
      containers:
        - name: api
          image: "{{ .Values.image.registry }}/{{ .Values.image.repository }}/{{ .Values.services.api.image }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ .Values.service.api.targetPort }}
              protocol: TCP
          env:
            - name: SERVICE_NAME
              value: "api"
            - name: LOG_LEVEL
              value: {{ .Values.env.LOG_LEVEL }}
            - name: LOG_FORMAT
              value: {{ .Values.env.LOG_FORMAT }}
            - name: OTEL_SERVICE_NAME
              value: "web-analyzer-api"
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: {{ .Values.env.OTEL_EXPORTER_OTLP_ENDPOINT }}
            - name: DB_HOST
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.existingSecret }}
                  key: {{ .Values.database.secretKeys.host }}
            - name: DB_PORT
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.existingSecret }}
                  key: {{ .Values.database.secretKeys.port }}
            - name: DB_NAME
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.existingSecret }}
                  key: {{ .Values.database.secretKeys.database }}
            - name: DB_USER
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.existingSecret }}
                  key: {{ .Values.database.secretKeys.username }}
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.database.existingSecret }}
                  key: {{ .Values.database.secretKeys.password }}
          livenessProbe:
            {{- toYaml .Values.livenessProbe | nindent 12 }}
          readinessProbe:
            {{- toYaml .Values.readinessProbe | nindent 12 }}
          resources:
            {{- toYaml .Values.resources.api | nindent 12 }}
```

### HPA Template

```yaml
# k8s/helm/web-analyzer/templates/api-hpa.yaml
{{- if .Values.autoscaling.api.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "web-analyzer.fullname" . }}-api
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "web-analyzer.labels" . | nindent 4 }}
    app.kubernetes.io/component: api
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "web-analyzer.fullname" . }}-api
  minReplicas: {{ .Values.autoscaling.api.minReplicas }}
  maxReplicas: {{ .Values.autoscaling.api.maxReplicas }}
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.api.targetCPUUtilizationPercentage }}
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: {{ .Values.autoscaling.api.targetMemoryUtilizationPercentage }}
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Percent
          value: 50
          periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
        - type: Percent
          value: 100
          periodSeconds: 15
        - type: Pods
          value: 4
          periodSeconds: 15
      selectPolicy: Max
{{- end }}
```

---

## 9.5. Istio Service Mesh

### Istio Installation

```yaml
# k8s/istio/install/istio-operator.yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: web-analyzer-istio
  namespace: istio-system
spec:
  profile: default

  # Control plane configuration
  components:
    pilot:
      k8s:
        resources:
          requests:
            cpu: 200m
            memory: 512Mi

    ingressGateways:
      - name: istio-ingressgateway
        enabled: true
        k8s:
          service:
            type: LoadBalancer
          resources:
            requests:
              cpu: 100m
              memory: 128Mi

    egressGateways:
      - name: istio-egressgateway
        enabled: true

  # Mesh configuration
  meshConfig:
    # Enable access logging
    accessLogFile: /dev/stdout
    accessLogEncoding: JSON

    # Tracing configuration
    enableTracing: true
    defaultConfig:
      tracing:
        sampling: 100.0
        zipkin:
          address: tempo-distributor.monitoring:9411

    # Metrics configuration
    enablePrometheusMerge: true

  # Values configuration
  values:
    global:
      proxy:
        resources:
          requests:
            cpu: 10m
            memory: 64Mi

    # Telemetry
    telemetry:
      enabled: true
      v2:
        enabled: true
        prometheus:
          enabled: true
```

### Istio Gateway

```yaml
# k8s/istio/gateway.yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: web-analyzer-gateway
  namespace: istio-system
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "*.web-analyzer.dev"
      tls:
        httpsRedirect: true
    - port:
        number: 443
        name: https
        protocol: HTTPS
      hosts:
        - "*.web-analyzer.dev"
      tls:
        mode: SIMPLE
        credentialName: web-analyzer-tls
```

### VirtualService

```yaml
# k8s/helm/web-analyzer/templates/virtualservice.yaml
{{- if and .Values.istio.enabled .Values.istio.virtualService.enabled }}
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: {{ include "web-analyzer.fullname" . }}
  namespace: {{ .Release.Namespace }}
spec:
  hosts:
    - api.web-analyzer.dev
  gateways:
    {{- toYaml .Values.istio.virtualService.gateways | nindent 4 }}
  http:
    # Retry policy
    - match:
        - uri:
            prefix: "/v1/analyze"
      retries:
        attempts: 3
        perTryTimeout: 10s
        retryOn: 5xx,reset,connect-failure,refused-stream
      route:
        - destination:
            host: {{ include "web-analyzer.fullname" . }}-api
            port:
              number: {{ .Values.service.api.port }}
          weight: 100

    # Timeout for SSE endpoint
    - match:
        - uri:
            prefix: "/v1/analysis/"
            regex: ".*\\/events$"
      timeout: 3600s
      route:
        - destination:
            host: {{ include "web-analyzer.fullname" . }}-api
            port:
              number: {{ .Values.service.api.port }}

    # Default route
    - route:
        - destination:
            host: {{ include "web-analyzer.fullname" . }}-api
            port:
              number: {{ .Values.service.api.port }}
{{- end }}
```

### DestinationRule

```yaml
# k8s/helm/web-analyzer/templates/destinationrule.yaml
{{- if and .Values.istio.enabled .Values.istio.destinationRule.enabled }}
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: {{ include "web-analyzer.fullname" . }}-api
  namespace: {{ .Release.Namespace }}
spec:
  host: {{ include "web-analyzer.fullname" . }}-api
  trafficPolicy:
    {{- toYaml .Values.istio.destinationRule.trafficPolicy | nindent 4 }}
    loadBalancer:
      consistentHash:
        httpHeaderName: "x-user-id"
    tls:
      mode: ISTIO_MUTUAL
  subsets:
    - name: v1
      labels:
        version: {{ .Chart.AppVersion }}
{{- end }}
```

### PeerAuthentication (mTLS)

```yaml
# k8s/istio/peerauthentication.yaml
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: istio-system
spec:
  mtls:
    mode: STRICT
---
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: web-analyzer
  namespace: web-analyzer-dev
spec:
  mtls:
    mode: STRICT
```

---

## 9.6. OpenTelemetry Instrumentation

### Go Service Instrumentation

```go
// internal/infrastructure/tracing/otel.go
package tracing

import (
    "context"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type OTELConfig struct {
    ServiceName    string
    ServiceVersion string
    Environment    string
    Endpoint       string  // OTEL Collector endpoint
}

func InitOTEL(ctx context.Context, cfg OTELConfig) (func(), error) {
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(cfg.ServiceName),
            semconv.ServiceVersion(cfg.ServiceVersion),
            semconv.DeploymentEnvironment(cfg.Environment),
            attribute.String("service.namespace", "web-analyzer"),
        ),
    )
    if err != nil {
        return nil, err
    }

    // Initialize trace provider
    traceShutdown, err := initTraceProvider(ctx, res, cfg.Endpoint)
    if err != nil {
        return nil, err
    }

    // Initialize metric provider
    metricShutdown, err := initMetricProvider(ctx, res, cfg.Endpoint)
    if err != nil {
        traceShutdown(ctx)

        return nil, err
    }

    // Set global propagator
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    shutdown := func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        _ = traceShutdown(ctx)
        _ = metricShutdown(ctx)
    }

    return shutdown, nil
}

func initTraceProvider(ctx context.Context, res *resource.Resource, endpoint string) (func(context.Context) error, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(res),
        trace.WithSampler(trace.AlwaysSample()),
    )

    otel.SetTracerProvider(tp)

    return tp.Shutdown, nil
}

func initMetricProvider(ctx context.Context, res *resource.Resource, endpoint string) (func(context.Context) error, error) {
    exporter, err := otlpmetricgrpc.New(ctx,
        otlpmetricgrpc.WithEndpoint(endpoint),
        otlpmetricgrpc.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    mp := metric.NewMeterProvider(
        metric.WithResource(res),
        metric.WithReader(metric.NewPeriodicReader(exporter,
            metric.WithInterval(30*time.Second),
        )),
    )

    otel.SetMeterProvider(mp)

    return mp.Shutdown, nil
}
```

### HTTP Middleware

```go
// internal/adapters/middleware/otel_http.go
package middleware

import (
    "net/http"

    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func OTELHTTPMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return otelhttp.NewHandler(next, "http-server",
            otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
                return r.Method + " " + r.URL.Path
            }),
        )
    }
}
```

---

## 9.7. Monitoring Stack (Prometheus + Mimir)

### OpenTelemetry Collector

```yaml
# k8s/monitoring/otel-collector/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
  namespace: monitoring
data:
  otel-collector-config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318

    processors:
      batch:
        timeout: 10s
        send_batch_size: 1024

      memory_limiter:
        check_interval: 1s
        limit_mib: 512

      attributes:
        actions:
          - key: cluster
            value: kind-local
            action: insert

    exporters:
      # Prometheus exporter (short-term metrics)
      prometheus:
        endpoint: "0.0.0.0:8889"
        namespace: webanalyzer
        const_labels:
          cluster: kind-local

      # Prometheus remote write to Mimir (long-term storage)
      prometheusremotewrite:
        endpoint: http://mimir-distributor.monitoring:9009/api/v1/push
        tls:
          insecure: true

      # Tempo exporter for traces
      otlp/tempo:
        endpoint: tempo-distributor.monitoring:4317
        tls:
          insecure: true

      # Jaeger exporter (multi-backend)
      jaeger:
        endpoint: jaeger-collector.monitoring:14250
        tls:
          insecure: true

      # Logging exporter for debugging
      logging:
        loglevel: info

    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch, attributes]
          exporters: [otlp/tempo, jaeger, logging]

        metrics:
          receivers: [otlp]
          processors: [memory_limiter, batch, attributes]
          exporters: [prometheus, prometheusremotewrite]
```

### OTEL Collector Deployment

```yaml
# k8s/monitoring/otel-collector/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
  namespace: monitoring
spec:
  replicas: 2
  selector:
    matchLabels:
      app: otel-collector
  template:
    metadata:
      labels:
        app: otel-collector
    spec:
      containers:
        - name: otel-collector
          image: otel/opentelemetry-collector-contrib:0.97.0
          args:
            - --config=/conf/otel-collector-config.yaml
          ports:
            - name: otlp-grpc
              containerPort: 4317
              protocol: TCP
            - name: otlp-http
              containerPort: 4318
              protocol: TCP
            - name: prometheus
              containerPort: 8889
              protocol: TCP
          volumeMounts:
            - name: config
              mountPath: /conf
          resources:
            requests:
              cpu: 200m
              memory: 512Mi
            limits:
              cpu: 1000m
              memory: 1Gi
      volumes:
        - name: config
          configMap:
            name: otel-collector-config
```

### Prometheus Operator

```yaml
# k8s/monitoring/prometheus/prometheus.yaml
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus
  namespace: monitoring
spec:
  replicas: 2
  serviceAccountName: prometheus
  serviceMonitorSelector:
    matchLabels:
      prometheus: web-analyzer
  podMonitorSelector:
    matchLabels:
      prometheus: web-analyzer
  retention: 7d
  retentionSize: "50GB"
  resources:
    requests:
      cpu: 500m
      memory: 2Gi
    limits:
      cpu: 2000m
      memory: 4Gi

  # Remote write to Mimir for long-term storage
  remoteWrite:
    - url: http://mimir-distributor.monitoring:9009/api/v1/push
      queueConfig:
        capacity: 10000
        maxShards: 50
        minShards: 1
        maxSamplesPerSend: 5000
        batchSendDeadline: 5s

  # Thanos sidecar for HA
  thanos:
    version: v0.34.0
    objectStorageConfig:
      key: thanos.yaml
      name: thanos-objstore-config
```

### Mimir (Long-term Metrics Storage)

```yaml
# k8s/monitoring/mimir/mimir-distributed.yaml
# Use Grafana Mimir Helm chart
# helm repo add grafana https://grafana.github.io/helm-charts
# helm install mimir grafana/mimir-distributed -n monitoring -f mimir-values.yaml

mimir:
  structuredConfig:
    multitenancy_enabled: false

    blocks_storage:
      backend: s3
      s3:
        endpoint: minio.monitoring:9000
        bucket_name: mimir-blocks
        access_key_id: minioadmin
        secret_access_key: minioadmin
        insecure: true

    limits:
      max_global_series_per_user: 1000000
      ingestion_rate: 50000
      ingestion_burst_size: 100000

# Component replicas
distributor:
  replicas: 2
  resources:
    requests:
      cpu: 100m
      memory: 512Mi

ingester:
  replicas: 3
  persistentVolume:
    enabled: true
    size: 10Gi
  resources:
    requests:
      cpu: 500m
      memory: 1Gi

querier:
  replicas: 2
  resources:
    requests:
      cpu: 250m
      memory: 512Mi

compactor:
  replicas: 1
  persistentVolume:
    enabled: true
    size: 20Gi

# MinIO for object storage (development)
minio:
  enabled: true
  replicas: 1
  persistence:
    enabled: true
    size: 50Gi
```

### ServiceMonitor

```yaml
# k8s/helm/web-analyzer/templates/servicemonitor.yaml
{{- if .Values.monitoring.enabled }}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ include "web-analyzer.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "web-analyzer.labels" . | nindent 4 }}
    prometheus: web-analyzer
spec:
  selector:
    matchLabels:
      {{- include "web-analyzer.selectorLabels" . | nindent 6 }}
  endpoints:
    - port: http
      path: /metrics
      interval: {{ .Values.monitoring.serviceMonitor.interval }}
      scrapeTimeout: {{ .Values.monitoring.serviceMonitor.scrapeTimeout }}
{{- end }}
```

---

## 9.8. Logging Stack (Loki + OpenSearch)

### Loki Stack

```yaml
# k8s/monitoring/loki/loki-stack.yaml
# Install using Helm
# helm repo add grafana https://grafana.github.io/helm-charts
# helm install loki grafana/loki-stack -n monitoring -f loki-values.yaml

loki:
  enabled: true
  isDefault: true
  replicas: 2

  config:
    auth_enabled: false

    ingester:
      chunk_idle_period: 3m
      chunk_block_size: 262144
      chunk_retain_period: 1m
      max_transfer_retries: 0
      lifecycler:
        ring:
          kvstore:
            store: inmemory
          replication_factor: 1

    limits_config:
      enforce_metric_name: false
      reject_old_samples: true
      reject_old_samples_max_age: 168h
      max_entries_limit_per_query: 5000
      max_query_series: 1000

    schema_config:
      configs:
        - from: 2024-01-01
          store: boltdb-shipper
          object_store: s3
          schema: v12
          index:
            prefix: loki_index_
            period: 24h

    storage_config:
      boltdb_shipper:
        active_index_directory: /data/loki/boltdb-shipper-active
        cache_location: /data/loki/boltdb-shipper-cache
        cache_ttl: 24h
        shared_store: s3
      aws:
        s3: s3://minioadmin:minioadmin@minio.monitoring:9000/loki
        s3forcepathstyle: true

  persistence:
    enabled: true
    size: 10Gi

promtail:
  enabled: true
  config:
    clients:
      - url: http://loki:3100/loki/api/v1/push

    snippets:
      scrapeConfigs: |
        # Kubernetes pod logs
        - job_name: kubernetes-pods
          pipeline_stages:
            - cri: {}
            - json:
                expressions:
                  level: level
                  msg: msg
                  trace_id: trace_id
            - labels:
                level:
                trace_id:
          kubernetes_sd_configs:
            - role: pod
          relabel_configs:
            - source_labels: [__meta_kubernetes_pod_controller_name]
              regex: ([0-9a-z-.]+?)(-[0-9a-f]{8,10})?
              action: replace
              target_label: __tmp_controller_name
            - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
              action: replace
              target_label: app
            - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_component]
              action: replace
              target_label: component
            - source_labels: [__meta_kubernetes_pod_node_name]
              action: replace
              target_label: node
            - source_labels: [__meta_kubernetes_namespace]
              action: replace
              target_label: namespace
            - source_labels: [__meta_kubernetes_pod_name]
              action: replace
              target_label: pod
            - source_labels: [__meta_kubernetes_pod_container_name]
              action: replace
              target_label: container
```

### OpenSearch for Compliance Logs

```yaml
# k8s/monitoring/opensearch/opensearch-cluster.yaml
# Install using Helm
# helm repo add opensearch https://opensearch-project.github.io/helm-charts/
# helm install opensearch opensearch/opensearch -n monitoring -f opensearch-values.yaml

replicas: 3

roles:
  - master
  - ingest
  - data

resources:
  requests:
    cpu: 500m
    memory: 2Gi
  limits:
    cpu: 2000m
    memory: 4Gi

persistence:
  enabled: true
  size: 50Gi

opensearchJavaOpts: "-Xmx1g -Xms1g"

config:
  opensearch.yml: |
    cluster.name: web-analyzer-logs
    network.host: 0.0.0.0
    plugins:
      security:
        disabled: false
        ssl:
          http:
            enabled: true
          transport:
            enabled: true

# Index templates for compliance logs
extraInitContainers:
  - name: create-index-templates
    image: curlimages/curl:8.6.0
    command:
      - sh
      - -c
      - |
        # Wait for OpenSearch to be ready
        until curl -k -u admin:admin https://opensearch-cluster-master:9200/_cluster/health; do
          echo "Waiting for OpenSearch..."
          sleep 5
        done

        # Create audit logs index template
        curl -k -u admin:admin -X PUT \
          "https://opensearch-cluster-master:9200/_index_template/audit-logs" \
          -H 'Content-Type: application/json' \
          -d '{
            "index_patterns": ["audit-logs-*"],
            "template": {
              "settings": {
                "number_of_shards": 2,
                "number_of_replicas": 1,
                "index.lifecycle.name": "audit-logs-policy"
              },
              "mappings": {
                "properties": {
                  "timestamp": {"type": "date"},
                  "user_id": {"type": "keyword"},
                  "action": {"type": "keyword"},
                  "resource": {"type": "keyword"},
                  "ip_address": {"type": "ip"},
                  "details": {"type": "object"}
                }
              }
            }
          }'
```

### Fluent Bit for OpenSearch Shipping

```yaml
# k8s/monitoring/opensearch/fluent-bit.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
  namespace: monitoring
data:
  fluent-bit.conf: |
    [SERVICE]
        Flush        5
        Daemon       Off
        Log_Level    info

    [INPUT]
        Name              tail
        Path              /var/log/containers/*web-analyzer*.log
        Parser            docker
        Tag               kube.*
        Refresh_Interval  5

    [FILTER]
        Name                kubernetes
        Match               kube.*
        Kube_URL            https://kubernetes.default.svc:443
        Kube_CA_File        /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        Kube_Token_File     /var/run/secrets/kubernetes.io/serviceaccount/token

    [FILTER]
        Name    grep
        Match   kube.*
        Regex   log .*"audit":true.*

    [OUTPUT]
        Name            opensearch
        Match           kube.*
        Host            opensearch-cluster-master
        Port            9200
        Index           audit-logs
        Type            _doc
        Logstash_Format On
        Logstash_Prefix audit-logs
        TLS             On
        TLS.Verify      Off
```

---

## 9.9. Tracing (Tempo + Multi-Backend)

### Grafana Tempo

```yaml
# k8s/monitoring/tempo/tempo.yaml
# Install using Helm
# helm repo add grafana https://grafana.github.io/helm-charts
# helm install tempo grafana/tempo-distributed -n monitoring -f tempo-values.yaml

tempo:
  structuredConfig:
    compactor:
      compaction:
        block_retention: 168h  # 7 days

    distributor:
      receivers:
        otlp:
          protocols:
            grpc:
              endpoint: 0.0.0.0:4317
            http:
              endpoint: 0.0.0.0:4318
        jaeger:
          protocols:
            thrift_http:
              endpoint: 0.0.0.0:14268
            grpc:
              endpoint: 0.0.0.0:14250

    storage:
      trace:
        backend: s3
        s3:
          bucket: tempo-traces
          endpoint: minio.monitoring:9000
          access_key: minioadmin
          secret_key: minioadmin
          insecure: true

distributor:
  replicas: 2
  resources:
    requests:
      cpu: 100m
      memory: 256Mi

ingester:
  replicas: 3
  persistence:
    enabled: true
    size: 10Gi
  resources:
    requests:
      cpu: 250m
      memory: 512Mi

querier:
  replicas: 2

compactor:
  replicas: 1
```

### Jaeger (Alternative Backend)

```yaml
# k8s/monitoring/tempo/jaeger.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jaeger-config
  namespace: monitoring
data:
  jaeger.yaml: |
    storage:
      type: opensearch
      options:
        opensearch:
          server-urls: https://opensearch-cluster-master:9200
          index-prefix: jaeger
          username: admin
          password: admin
          tls:
            enabled: true
            skip-host-verify: true
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
        - name: jaeger
          image: jaegertracing/all-in-one:1.55
          env:
            - name: SPAN_STORAGE_TYPE
              value: opensearch
          ports:
            - containerPort: 16686  # UI
            - containerPort: 14250  # gRPC
            - containerPort: 14268  # HTTP
          volumeMounts:
            - name: config
              mountPath: /etc/jaeger
      volumes:
        - name: config
          configMap:
            name: jaeger-config
```

---

## 9.10. Grafana Dashboards

### Grafana Installation

```yaml
# k8s/monitoring/grafana/grafana.yaml
# Install using Helm
# helm repo add grafana https://grafana.github.io/helm-charts
# helm install grafana grafana/grafana -n monitoring -f grafana-values.yaml

adminPassword: admin

datasources:
  datasources.yaml:
    apiVersion: 1
    datasources:
      # Prometheus (short-term metrics)
      - name: Prometheus
        type: prometheus
        url: http://prometheus-operated:9090
        access: proxy
        isDefault: true

      # Mimir (long-term metrics)
      - name: Mimir
        type: prometheus
        url: http://mimir-query-frontend:8080/prometheus
        access: proxy

      # Loki (operational logs)
      - name: Loki
        type: loki
        url: http://loki:3100
        access: proxy

      # Tempo (distributed tracing)
      - name: Tempo
        type: tempo
        url: http://tempo-query-frontend:3100
        access: proxy

      # Jaeger (alternative tracing)
      - name: Jaeger
        type: jaeger
        url: http://jaeger:16686
        access: proxy

      # OpenSearch (compliance logs)
      - name: OpenSearch
        type: opensearch
        url: https://opensearch-cluster-master:9200
        access: proxy
        basicAuth: true
        basicAuthUser: admin
        secureJsonData:
          basicAuthPassword: admin

dashboardProviders:
  dashboardproviders.yaml:
    apiVersion: 1
    providers:
      - name: 'default'
        orgId: 1
        folder: ''
        type: file
        disableDeletion: false
        editable: true
        options:
          path: /var/lib/grafana/dashboards/default

dashboards:
  default:
    web-analyzer-overview:
      gnetId: 0
      file: dashboards/web-analyzer-overview.json
    istio-mesh:
      gnetId: 7639
      revision: 183
      datasource: Prometheus
    kubernetes-cluster:
      gnetId: 7249
      revision: 1
      datasource: Prometheus
```

### Custom Dashboard (Web Analyzer Overview)

```json
{
  "dashboard": {
    "title": "Web Analyzer - Service Overview",
    "panels": [
      {
        "title": "Request Rate (RPS)",
        "targets": [
          {
            "expr": "rate(http_requests_total{service=\"web-analyzer-api\"}[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Analysis Duration",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, rate(analysis_duration_seconds_bucket[5m]))",
            "legendFormat": "p99"
          },
          {
            "expr": "histogram_quantile(0.95, rate(analysis_duration_seconds_bucket[5m]))",
            "legendFormat": "p95"
          },
          {
            "expr": "histogram_quantile(0.50, rate(analysis_duration_seconds_bucket[5m]))",
            "legendFormat": "p50"
          }
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total{service=\"web-analyzer-api\",status=~\"5..\"}[5m])",
            "legendFormat": "5xx errors"
          }
        ]
      },
      {
        "title": "Queue Depth",
        "targets": [
          {
            "expr": "rabbitmq_queue_messages{queue=\"analysis_queue\"}",
            "legendFormat": "Pending analyses"
          }
        ]
      },
      {
        "title": "Recent Logs",
        "type": "logs",
        "targets": [
          {
            "expr": "{namespace=\"web-analyzer-dev\", app=\"web-analyzer\"}"
          }
        ]
      },
      {
        "title": "Distributed Traces",
        "type": "traces",
        "datasource": "Tempo"
      }
    ]
  }
}
```

---

## 9.11. Traefik Ingress Controller

### Traefik Helm Values

```yaml
# k8s/traefik/traefik-values.yaml
# Install using Helm
# helm repo add traefik https://traefik.github.io/charts
# helm install traefik traefik/traefik -n kube-system -f traefik-values.yaml

deployment:
  replicas: 2

ingressClass:
  enabled: true
  isDefaultClass: true

service:
  type: LoadBalancer

ports:
  web:
    port: 80
    redirectTo:
      port: websecure
  websecure:
    port: 443
    tls:
      enabled: true

additionalArguments:
  - "--metrics.prometheus=true"
  - "--tracing.opentelemetry=true"
  - "--tracing.opentelemetry.address=otel-collector.monitoring:4317"
  - "--accesslog=true"
  - "--accesslog.format=json"

metrics:
  prometheus:
    serviceMonitor:
      enabled: true
```

---

## 9.12. Implementation Checklist

### Phase 1: Local Development Environment
- [ ] Create Kind cluster with local registry
- [ ] Install MetalLB for LoadBalancer support
- [ ] Configure local DNS (`*.web-analyzer.dev`)
- [ ] Install cert-manager for TLS certificates

### Phase 2: GitOps Foundation
- [ ] Install ArgoCD
- [ ] Create ArgoCD projects and applications
- [ ] Configure ApplicationSets for multi-environment
- [ ] Set up Git repository structure

### Phase 3: Infrastructure Provisioning
- [ ] Install Crossplane and providers
- [ ] Create composite resource definitions
- [ ] Deploy infrastructure compositions
- [ ] Create resource claims for databases, cache, queues

### Phase 4: Service Mesh
- [ ] Install Istio control plane
- [ ] Configure Istio gateways
- [ ] Enable mTLS with PeerAuthentication
- [ ] Create VirtualServices and DestinationRules

### Phase 5: Observability - Metrics
- [ ] Deploy OpenTelemetry Collector
- [ ] Install Prometheus Operator
- [ ] Deploy Mimir for long-term storage
- [ ] Configure ServiceMonitors for scraping
- [ ] Instrument Go services with OTEL SDK

### Phase 6: Observability - Logging
- [ ] Deploy Loki stack with Promtail
- [ ] Install OpenSearch cluster
- [ ] Configure Fluent Bit for audit logs
- [ ] Create index templates

### Phase 7: Observability - Tracing
- [ ] Deploy Grafana Tempo
- [ ] Install Jaeger (optional)
- [ ] Configure OTEL Collector trace pipeline
- [ ] Add trace context propagation

### Phase 8: Visualization
- [ ] Deploy Grafana
- [ ] Configure data sources
- [ ] Import dashboards
- [ ] Create custom Web Analyzer dashboards

### Phase 9: Application Deployment
- [ ] Build and push Docker images to local registry
- [ ] Create Helm chart for web-analyzer services
- [ ] Configure environment-specific values
- [ ] Deploy via ArgoCD

### Phase 10: Testing & Validation
- [ ] Verify all services are running
- [ ] Test ingress routing
- [ ] Validate metrics collection
- [ ] Check log aggregation
- [ ] Verify distributed tracing
- [ ] Load testing with auto-scaling

### Deployment Commands Summary

```bash
# 1. Create Kind cluster
cd k8s/kind && ./setup.sh

# 2. Install ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl apply -f k8s/argocd/

# 3. Install Crossplane
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm install crossplane --namespace crossplane-system crossplane-stable/crossplane --create-namespace
kubectl apply -f k8s/crossplane/install/
kubectl apply -f k8s/crossplane/compositions/
kubectl apply -f k8s/crossplane/claims/

# 4. Install Istio
istioctl install --set profile=demo -y
kubectl apply -f k8s/istio/

# 5. Install monitoring stack
kubectl create namespace monitoring
helm install otel-collector open-telemetry/opentelemetry-collector -n monitoring
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring
helm install mimir grafana/mimir-distributed -n monitoring -f k8s/monitoring/mimir/mimir-values.yaml
helm install loki grafana/loki-stack -n monitoring -f k8s/monitoring/loki/loki-values.yaml
helm install tempo grafana/tempo-distributed -n monitoring -f k8s/monitoring/tempo/tempo-values.yaml
helm install grafana grafana/grafana -n monitoring -f k8s/monitoring/grafana/grafana-values.yaml

# 6. Install Traefik
helm install traefik traefik/traefik -n kube-system -f k8s/traefik/traefik-values.yaml

# 7. Deploy application
kubectl create namespace web-analyzer-dev
helm install web-analyzer k8s/helm/web-analyzer -n web-analyzer-dev -f k8s/helm/web-analyzer/values-dev.yaml

# 8. Access services
kubectl port-forward -n argocd svc/argocd-server 8080:443
kubectl port-forward -n monitoring svc/grafana 3000:80
kubectl port-forward -n monitoring svc/jaeger 16686:16686
```

---

## 10. Error Recovery and Circuit Breaker 🟢 **MEDIUM**

### Current State
- Basic error handling without resilience patterns
- No circuit breaker implementation
- Limited retry mechanisms

### Required Improvements
```go
// Implement circuit breaker for external calls
import "github.com/sony/gobreaker"

type ResilientFetcher struct {
    breaker *gobreaker.CircuitBreaker
    fetcher WebFetcher
}

// Configuration:
// - Max failures: 5
// - Timeout: 60 seconds
// - Half-open requests: 1
```

### Additional Patterns
- Exponential backoff for retries
- Dead letter queue processing
- Graceful degradation
- Fallback mechanisms

---

## 11. API Rate Limiting Enhancement - Per-Client Configuration 🟢 **MEDIUM**

### Current State
- Basic rate limiting middleware exists
- No distributed rate limiting
- Limited configurability
- No per-client customization
- Missing rate limit tier management

### Overview

Implement comprehensive per-client rate limiting with configurable tiers, distributed storage, and flexible algorithm support. This allows different clients to have different rate limits based on their subscription tier, API key, or custom configuration.

### Required Improvements

#### 1. Rate Limit Configuration Structure

```yaml
# Configuration file: config/rate-limits.yaml
rate_limits:
  # Default tier for unauthenticated requests
  default:
    requests_per_second: 10
    requests_per_minute: 100
    requests_per_hour: 1000
    burst: 20

  # Tier-based limits
  tiers:
    free:
      requests_per_second: 5
      requests_per_minute: 50
      requests_per_hour: 500
      burst: 10

    basic:
      requests_per_second: 20
      requests_per_minute: 500
      requests_per_hour: 10000
      burst: 50

    premium:
      requests_per_second: 100
      requests_per_minute: 5000
      requests_per_hour: 100000
      burst: 200

    enterprise:
      requests_per_second: 500
      requests_per_minute: 25000
      requests_per_hour: 500000
      burst: 1000

  # Per-client custom overrides (database-backed)
  custom_limits:
    enabled: true
    storage: "postgres"  # Store in database for dynamic updates

  # Per-endpoint multipliers
  endpoint_multipliers:
    "/v1/analyze":
      multiplier: 2.0  # Analysis is more expensive
    "/v1/analysis/{id}":
      multiplier: 0.5  # Reading is cheaper
    "/v1/health":
      exempt: true     # No rate limiting on health checks

  # Algorithm configuration
  algorithm: "token_bucket"  # Options: token_bucket, sliding_window, fixed_window
  storage_backend: "keydb"   # Options: keydb, redis, memory

  # Headers to include in response
  headers:
    enabled: true
    prefix: "X-RateLimit-"
    include:
      - limit
      - remaining
      - reset
      - retry-after  # On 429 responses
```

#### 2. Rate Limit Domain Models

```go
// internal/domain/rate_limit.go
package domain

import (
	"time"
)

// RateLimitTier represents a predefined rate limit tier
type RateLimitTier string

const (
	RateLimitTierDefault    RateLimitTier = "default"
	RateLimitTierFree       RateLimitTier = "free"
	RateLimitTierBasic      RateLimitTier = "basic"
	RateLimitTierPremium    RateLimitTier = "premium"
	RateLimitTierEnterprise RateLimitTier = "enterprise"
)

// RateLimitConfig defines rate limit configuration for a client
type RateLimitConfig struct {
	ClientID           string        `json:"client_id"`
	Tier               RateLimitTier `json:"tier"`
	RequestsPerSecond  int           `json:"requests_per_second"`
	RequestsPerMinute  int           `json:"requests_per_minute"`
	RequestsPerHour    int           `json:"requests_per_hour"`
	Burst              int           `json:"burst"`
	CustomLimits       bool          `json:"custom_limits"`
	ExemptEndpoints    []string      `json:"exempt_endpoints,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed       bool      `json:"allowed"`
	Limit         int64     `json:"limit"`
	Remaining     int64     `json:"remaining"`
	ResetAt       time.Time `json:"reset_at"`
	RetryAfter    int64     `json:"retry_after,omitempty"`  // Seconds until retry
	ClientID      string    `json:"client_id"`
	Tier          string    `json:"tier"`
}

// ClientRateLimit tracks current usage for a client
type ClientRateLimit struct {
	ClientID        string    `json:"client_id"`
	Window          string    `json:"window"`      // "second", "minute", "hour"
	Count           int64     `json:"count"`
	WindowStart     time.Time `json:"window_start"`
	LastRequest     time.Time `json:"last_request"`
}
```

#### 3. Distributed Rate Limiter Implementation

```go
// internal/infrastructure/rate_limiter.go
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

// DistributedRateLimiter implements rate limiting with KeyDB/Redis backend
type DistributedRateLimiter struct {
	client       *redis.Client
	config       *RateLimitConfig
	tierConfigs  map[domain.RateLimitTier]*domain.RateLimitConfig
	customLimits RateLimitRepository
}

// RateLimitConfig for the rate limiter
type RateLimitConfig struct {
	Algorithm     string
	StoragePrefix string
	DefaultTier   domain.RateLimitTier
}

func NewDistributedRateLimiter(
	client *redis.Client,
	config *RateLimitConfig,
	tierConfigs map[domain.RateLimitTier]*domain.RateLimitConfig,
	customLimits RateLimitRepository,
) *DistributedRateLimiter {
	return &DistributedRateLimiter{
		client:       client,
		config:       config,
		tierConfigs:  tierConfigs,
		customLimits: customLimits,
	}
}

// Allow checks if a request should be allowed based on rate limits
func (r *DistributedRateLimiter) Allow(
	ctx context.Context,
	clientID string,
	endpoint string,
) (*domain.RateLimitResult, error) {
	// Get rate limit configuration for client
	limitConfig, err := r.getLimitConfig(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get limit config: %w", err)
	}

	// Check if endpoint is exempt
	if r.isExempt(endpoint, limitConfig) {
		return &domain.RateLimitResult{
			Allowed:   true,
			Limit:     -1,
			Remaining: -1,
			ClientID:  clientID,
			Tier:      string(limitConfig.Tier),
		}, nil
	}

	// Apply endpoint multiplier
	adjustedLimit := r.applyEndpointMultiplier(endpoint, limitConfig.RequestsPerSecond)

	// Execute rate limiting algorithm
	switch r.config.Algorithm {
	case "token_bucket":
		return r.tokenBucketAlgorithm(ctx, clientID, limitConfig, adjustedLimit)
	case "sliding_window":
		return r.slidingWindowAlgorithm(ctx, clientID, limitConfig, adjustedLimit)
	case "fixed_window":
		return r.fixedWindowAlgorithm(ctx, clientID, limitConfig, adjustedLimit)
	default:
		return nil, fmt.Errorf("unknown algorithm: %s", r.config.Algorithm)
	}
}

// Token Bucket Algorithm Implementation
func (r *DistributedRateLimiter) tokenBucketAlgorithm(
	ctx context.Context,
	clientID string,
	config *domain.RateLimitConfig,
	limit int,
) (*domain.RateLimitResult, error) {
	key := fmt.Sprintf("%s:token_bucket:%s", r.config.StoragePrefix, clientID)
	now := time.Now()

	// Lua script for atomic token bucket implementation
	script := redis.NewScript(`
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local cost = 1

		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1])
		local last_refill = tonumber(bucket[2])

		if tokens == nil then
			tokens = capacity
			last_refill = now
		end

		-- Refill tokens based on time elapsed
		local elapsed = now - last_refill
		local refilled = elapsed * refill_rate
		tokens = math.min(capacity, tokens + refilled)

		local allowed = 0
		if tokens >= cost then
			tokens = tokens - cost
			allowed = 1
		end

		-- Update state
		redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
		redis.call('EXPIRE', key, 3600)

		return {allowed, math.floor(tokens), last_refill}
	`)

	result, err := script.Run(
		ctx,
		r.client,
		[]string{key},
		config.Burst,                              // capacity
		float64(config.RequestsPerSecond),         // refill_rate
		now.Unix(),                                // now
	).Result()

	if err != nil {
		return nil, fmt.Errorf("token bucket script failed: %w", err)
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	remaining := resultSlice[1].(int64)
	lastRefill := resultSlice[2].(int64)

	resetAt := time.Unix(lastRefill, 0).Add(time.Second)
	retryAfter := int64(0)
	if !allowed {
		retryAfter = int64(resetAt.Sub(now).Seconds())
	}

	return &domain.RateLimitResult{
		Allowed:    allowed,
		Limit:      int64(config.Burst),
		Remaining:  remaining,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
		ClientID:   clientID,
		Tier:       string(config.Tier),
	}, nil
}

// Sliding Window Algorithm Implementation
func (r *DistributedRateLimiter) slidingWindowAlgorithm(
	ctx context.Context,
	clientID string,
	config *domain.RateLimitConfig,
	limit int,
) (*domain.RateLimitResult, error) {
	key := fmt.Sprintf("%s:sliding_window:%s", r.config.StoragePrefix, clientID)
	now := time.Now()
	windowSize := time.Minute // Use 1-minute windows

	// Lua script for atomic sliding window implementation
	script := redis.NewScript(`
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_size = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])

		local window_start = now - window_size

		-- Remove old entries outside the window
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

		-- Count requests in current window
		local current_count = redis.call('ZCARD', key)

		local allowed = 0
		if current_count < limit then
			-- Add current request
			redis.call('ZADD', key, now, now)
			redis.call('EXPIRE', key, window_size * 2)
			allowed = 1
			current_count = current_count + 1
		end

		return {allowed, current_count, limit}
	`)

	result, err := script.Run(
		ctx,
		r.client,
		[]string{key},
		now.Unix(),
		windowSize.Seconds(),
		limit,
	).Result()

	if err != nil {
		return nil, fmt.Errorf("sliding window script failed: %w", err)
	}

	resultSlice := result.([]interface{})
	allowed := resultSlice[0].(int64) == 1
	currentCount := resultSlice[1].(int64)
	limitValue := resultSlice[2].(int64)

	resetAt := now.Add(windowSize)
	retryAfter := int64(0)
	if !allowed {
		retryAfter = int64(windowSize.Seconds())
	}

	return &domain.RateLimitResult{
		Allowed:    allowed,
		Limit:      limitValue,
		Remaining:  limitValue - currentCount,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
		ClientID:   clientID,
		Tier:       string(config.Tier),
	}, nil
}

// Fixed Window Algorithm Implementation
func (r *DistributedRateLimiter) fixedWindowAlgorithm(
	ctx context.Context,
	clientID string,
	config *domain.RateLimitConfig,
	limit int,
) (*domain.RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Truncate(time.Minute)
	key := fmt.Sprintf("%s:fixed_window:%s:%d", r.config.StoragePrefix, clientID, windowStart.Unix())

	// Increment counter atomically
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to increment counter: %w", err)
	}

	// Set expiration on first request in window
	if count == 1 {
		r.client.Expire(ctx, key, time.Minute*2)
	}

	allowed := count <= int64(limit)
	resetAt := windowStart.Add(time.Minute)
	retryAfter := int64(0)
	if !allowed {
		retryAfter = int64(resetAt.Sub(now).Seconds())
	}

	return &domain.RateLimitResult{
		Allowed:    allowed,
		Limit:      int64(limit),
		Remaining:  max(0, int64(limit)-count),
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
		ClientID:   clientID,
		Tier:       string(config.Tier),
	}, nil
}

// getLimitConfig retrieves rate limit configuration for a client
func (r *DistributedRateLimiter) getLimitConfig(
	ctx context.Context,
	clientID string,
) (*domain.RateLimitConfig, error) {
	// Check for custom limits first
	customLimit, err := r.customLimits.GetByClientID(ctx, clientID)
	if err == nil && customLimit != nil {
		return customLimit, nil
	}

	// Fall back to tier-based limits
	// In production, fetch client's tier from database
	tier := r.config.DefaultTier

	config, exists := r.tierConfigs[tier]
	if !exists {
		return nil, fmt.Errorf("no configuration for tier: %s", tier)
	}

	return config, nil
}

// isExempt checks if an endpoint is exempt from rate limiting
func (r *DistributedRateLimiter) isExempt(endpoint string, config *domain.RateLimitConfig) bool {
	for _, exempt := range config.ExemptEndpoints {
		if endpoint == exempt {
			return true
		}
	}

	return false
}

// applyEndpointMultiplier adjusts the limit based on endpoint cost
func (r *DistributedRateLimiter) applyEndpointMultiplier(endpoint string, baseLimit int) int {
	// Endpoint multipliers from configuration
	multipliers := map[string]float64{
		"/v1/analyze":        2.0,  // More expensive
		"/v1/analysis/{id}":  0.5,  // Cheaper
	}

	if multiplier, exists := multipliers[endpoint]; exists {
		return int(float64(baseLimit) * multiplier)
	}

	return baseLimit
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}

	return b
}
```

#### 4. Rate Limit Repository (Database-backed Custom Limits)

```go
// internal/adapters/repos/rate_limit_repository.go
package repos

import (
	"context"
	"database/sql"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/domain"
)

type RateLimitRepository struct {
	db *sql.DB
}

func NewRateLimitRepository(db *sql.DB) *RateLimitRepository {
	return &RateLimitRepository{db: db}
}

func (r *RateLimitRepository) GetByClientID(ctx context.Context, clientID string) (*domain.RateLimitConfig, error) {
	query := `
		SELECT
			client_id, tier, requests_per_second, requests_per_minute,
			requests_per_hour, burst, custom_limits, exempt_endpoints,
			created_at, updated_at
		FROM rate_limit_configs
		WHERE client_id = $1 AND archived_at IS NULL
	`

	var config domain.RateLimitConfig
	var exemptEndpoints sql.NullString

	err := r.db.QueryRowContext(ctx, query, clientID).Scan(
		&config.ClientID,
		&config.Tier,
		&config.RequestsPerSecond,
		&config.RequestsPerMinute,
		&config.RequestsPerHour,
		&config.Burst,
		&config.CustomLimits,
		&exemptEndpoints,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	// Parse exempt endpoints if present
	if exemptEndpoints.Valid {
		// Assuming JSON array stored as string
		// Parse accordingly
	}

	return &config, nil
}

func (r *RateLimitRepository) Upsert(ctx context.Context, config *domain.RateLimitConfig) error {
	query := `
		INSERT INTO rate_limit_configs
			(client_id, tier, requests_per_second, requests_per_minute,
			 requests_per_hour, burst, custom_limits, exempt_endpoints, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (client_id)
		DO UPDATE SET
			tier = EXCLUDED.tier,
			requests_per_second = EXCLUDED.requests_per_second,
			requests_per_minute = EXCLUDED.requests_per_minute,
			requests_per_hour = EXCLUDED.requests_per_hour,
			burst = EXCLUDED.burst,
			custom_limits = EXCLUDED.custom_limits,
			exempt_endpoints = EXCLUDED.exempt_endpoints,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query,
		config.ClientID,
		config.Tier,
		config.RequestsPerSecond,
		config.RequestsPerMinute,
		config.RequestsPerHour,
		config.Burst,
		config.CustomLimits,
		config.ExemptEndpoints,
	)

	return err
}
```

#### 5. Rate Limiting Middleware

```go
// internal/adapters/middleware/rate_limit.go
package middleware

import (
	"net/http"
	"strconv"

	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
)

func RateLimitMiddleware(limiter *infrastructure.DistributedRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client identifier (from auth token, API key, or IP)
			clientID := extractClientID(r)
			endpoint := r.URL.Path

			// Check rate limit
			result, err := limiter.Allow(r.Context(), clientID, endpoint)
			if err != nil {
				http.Error(w, "Rate limit check failed", http.StatusInternalServerError)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
			w.Header().Set("X-RateLimit-Tier", result.Tier)

			if !result.Allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(result.RetryAfter, 10))
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractClientID(r *http.Request) string {
	// Priority order:
	// 1. Authenticated user ID from context
	// 2. API key from header
	// 3. Client IP address

	if userID := r.Context().Value("user_id"); userID != nil {
		return userID.(string)
	}

	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return apiKey
	}

	// Fall back to IP address
	return r.RemoteAddr
}
```

#### 6. Database Migration for Rate Limit Configs

```sql
-- migrations/YYYYMMDDHHMMSS_create_rate_limit_configs_table.up.sql
CREATE TABLE IF NOT EXISTS rate_limit_configs (
    client_id VARCHAR(255) PRIMARY KEY,
    tier VARCHAR(50) NOT NULL DEFAULT 'default',
    requests_per_second INT NOT NULL DEFAULT 10,
    requests_per_minute INT NOT NULL DEFAULT 100,
    requests_per_hour INT NOT NULL DEFAULT 1000,
    burst INT NOT NULL DEFAULT 20,
    custom_limits BOOLEAN NOT NULL DEFAULT FALSE,
    exempt_endpoints TEXT[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    archived_at TIMESTAMP
);

CREATE INDEX idx_rate_limit_configs_tier ON rate_limit_configs(tier);
CREATE INDEX idx_rate_limit_configs_archived ON rate_limit_configs(archived_at) WHERE archived_at IS NULL;

-- Insert default tier configurations
INSERT INTO rate_limit_configs (client_id, tier, requests_per_second, requests_per_minute, requests_per_hour, burst)
VALUES
    ('tier:free', 'free', 5, 50, 500, 10),
    ('tier:basic', 'basic', 20, 500, 10000, 50),
    ('tier:premium', 'premium', 100, 5000, 100000, 200),
    ('tier:enterprise', 'enterprise', 500, 25000, 500000, 1000);

-- migrations/YYYYMMDDHHMMSS_create_rate_limit_configs_table.down.sql
DROP TABLE IF EXISTS rate_limit_configs;
```

#### 7. Monitoring and Dashboard Requirements

```yaml
# Prometheus metrics for rate limiting
rate_limit_requests_total:
  type: counter
  labels: [client_id, tier, endpoint, allowed]
  description: "Total rate limit checks"

rate_limit_exceeded_total:
  type: counter
  labels: [client_id, tier, endpoint]
  description: "Total rate limit violations"

rate_limit_remaining:
  type: gauge
  labels: [client_id, tier]
  description: "Current remaining rate limit tokens"

rate_limit_usage_percentage:
  type: gauge
  labels: [client_id, tier]
  description: "Current usage as percentage of limit"
```

**Grafana Dashboard Panels:**
- Real-time rate limit usage per client
- Top clients by request volume
- Rate limit violations timeline
- Tier distribution chart
- Client usage trends (hourly/daily/weekly)
- Alert on limit breaches (>90% usage, violations)

#### 8. API Endpoints for Rate Limit Management

```yaml
# OpenAPI spec additions
/v1/admin/rate-limits:
  get:
    summary: List all client rate limits
    responses:
      200:
        description: List of rate limit configurations

/v1/admin/rate-limits/{clientId}:
  get:
    summary: Get rate limit for specific client
  put:
    summary: Update rate limit configuration
  delete:
    summary: Remove custom rate limit (revert to tier default)

/v1/rate-limit/status:
  get:
    summary: Get current rate limit status for authenticated client
    responses:
      200:
        schema:
          type: object
          properties:
            tier: string
            limit: integer
            remaining: integer
            reset_at: string (timestamp)
```

#### 9. Implementation Checklist

- [ ] Define rate limit tiers and default configurations
- [ ] Create database migration for rate_limit_configs table
- [ ] Implement domain models for rate limiting
- [ ] Implement distributed rate limiter with KeyDB backend
- [ ] Implement token bucket algorithm
- [ ] Implement sliding window algorithm
- [ ] Implement fixed window algorithm
- [ ] Create rate limit repository for custom limits
- [ ] Implement rate limiting middleware
- [ ] Add rate limit headers to responses
- [ ] Create admin API endpoints for rate limit management
- [ ] Add Prometheus metrics for rate limiting
- [ ] Create Grafana dashboards for monitoring
- [ ] Add alerts for rate limit violations
- [ ] Write unit tests for all algorithms
- [ ] Write integration tests with KeyDB
- [ ] Update OpenAPI specification
- [ ] Document rate limiting behavior for API consumers
- [ ] Add configuration examples to documentation

#### 10. Benefits

- **Flexible Configuration**: Per-client, per-tier, and per-endpoint limits
- **Distributed**: Works across multiple API server instances with KeyDB
- **Multiple Algorithms**: Token bucket, sliding window, and fixed window support
- **Real-time**: Immediate enforcement with atomic operations
- **Observable**: Comprehensive metrics and monitoring
- **Scalable**: Redis/KeyDB backend scales horizontally
- **Fair**: Prevents abuse while allowing legitimate bursts
- **Transparent**: Clear headers inform clients of their limits
- **Dynamic**: Database-backed custom limits can be updated without deployment

---

## 12. Documentation Improvements 🔵 **LOW**

### Current State
- Good architectural documentation
- Missing operational guides
- No troubleshooting documentation

### Required Improvements
```markdown
docs/
├── operations/
│   ├── runbook.md           # Common operational tasks
│   ├── troubleshooting.md   # Problem diagnosis guide
│   ├── performance.md       # Tuning guidelines
│   └── monitoring.md        # Monitoring setup
├── api/
│   ├── sdk-examples/        # Client SDK examples
│   └── postman/             # Postman collections
└── development/
    ├── contributing.md      # Contribution guidelines
    └── architecture.md      # Detailed architecture
```

---

## 13. Security Enhancements - RFC 9421 HTTP Message Signatures 🟡 **HIGH**

### Current State
- PASETO authentication implemented
- Basic security headers
- No request/response signing
- ADR mentions future consideration for signed requests/responses

### RFC 9421 Implementation (HTTP Message Signatures)

[RFC 9421](https://datatracker.ietf.org/doc/html/rfc9421) provides a standardized mechanism for creating and verifying digital signatures for HTTP messages, offering superior security compared to custom implementations. This aligns with the **Architecture Decision Records (ADRs)** which mention "Signed Requests and Responses" as a future consideration for enhanced security.

#### Why RFC 9421 Matters
- **Data Integrity**: Ensures messages haven't been tampered with in transit
- **Authentication**: Cryptographically proves the sender's identity
- **Non-repudiation**: Provides undeniable proof of message origin
- **Standards-based**: Avoids proprietary solutions, ensuring interoperability
- **Modern Security**: Replaces older, less secure methods like basic HMAC

#### Key Benefits
- **Standards Compliance**: Industry-standard HTTP message signing
- **Flexible Algorithms**: Support for RSA-PSS, ECDSA, HMAC-SHA256, EdDSA
- **Selective Signing**: Choose which HTTP fields to include in signature
- **Replay Protection**: Built-in timestamp and nonce support
- **Non-repudiation**: Cryptographic proof of message origin

### Required Improvements

#### Implementation Architecture
```go
// RFC 9421 compliant HTTP signature implementation
package httpsig

import (
    "github.com/dunglas/httpsfv"  // Structured Field Values (RFC 8941)
)

type HTTPSignature struct {
    Algorithm    string              // rsa-pss-sha512, ecdsa-p256-sha256, hmac-sha256, ed25519
    KeyID        string              // Key identifier for verification
    Created      int64               // Unix timestamp
    Expires      int64               // Signature expiration
    Nonce        string              // Unique value for replay protection
    CoveredComponents []string       // HTTP fields included in signature
}

// Signature-Input header (RFC 9421 Section 4.2)
type SignatureInput struct {
    SignatureParams map[string]interface{}
    Components      []Component
}

// Signature header (RFC 9421 Section 4.3)
type Signature struct {
    SignatureBase64 string
    Parameters      SignatureInput
}
```

#### Signing Components Configuration
```yaml
# Components to sign for requests
request_components:
  - "@method"           # HTTP method
  - "@target-uri"       # Full URI
  - "@authority"        # Host header
  - "content-type"      # Content type
  - "content-digest"    # SHA-256 of body (RFC 9530)
  - "authorization"     # Include auth token
  - "x-request-id"      # Request tracking

# Components to sign for responses
response_components:
  - "@status"           # HTTP status code
  - "content-type"
  - "content-digest"
  - "x-request-id"
  - "etag"             # For cache validation
```

#### Implementation Example
```go
// Middleware for request signing
func HTTPSignatureMiddleware(signer Signer) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Create signature base per RFC 9421 Section 2.3
            signatureBase := createSignatureBase(r, coveredComponents)

            // Generate signature
            signature := signer.Sign(signatureBase)

            // Add Signature-Input header (RFC 9421 Section 4.2)
            r.Header.Set("Signature-Input", formatSignatureInput(components, params))

            // Add Signature header (RFC 9421 Section 4.3)
            r.Header.Set("Signature", signature)

            next.ServeHTTP(w, r)
        })
    }
}

// Middleware for signature verification
func VerifyHTTPSignature(verifier Verifier) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract signature headers
            signatureInput := r.Header.Get("Signature-Input")
            signature := r.Header.Get("Signature")

            if signature == "" {
                http.Error(w, "Missing HTTP signature", http.StatusUnauthorized)
                return
            }

            // Verify signature per RFC 9421 Section 3.2
            if err := verifier.Verify(r, signature, signatureInput); err != nil {
                http.Error(w, "Invalid signature", http.StatusUnauthorized)
                return
            }

            // Check timestamp freshness (prevent replay attacks)
            if err := verifyTimestamp(signatureInput, maxAge); err != nil {
                http.Error(w, "Signature expired", http.StatusUnauthorized)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

#### Key Management
```go
// Key rotation and management
type KeyManager struct {
    ActiveKeyID   string
    Keys          map[string]SigningKey
    RotationPeriod time.Duration
}

// Support for multiple key types
type SigningKey interface {
    Sign(data []byte) ([]byte, error)
    Verify(data, signature []byte) error
    Algorithm() string
}

// Implementations for different algorithms
type RSAKey struct { /* RSA-PSS implementation */ }
type ECDSAKey struct { /* ECDSA implementation */ }
type Ed25519Key struct { /* EdDSA implementation */ }
type HMACKey struct { /* HMAC shared secret */ }
```

#### Content Digest (RFC 9530)
```go
// Implement content digest for body integrity
func AddContentDigest(r *http.Request) error {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        return err
    }
    r.Body = io.NopCloser(bytes.NewReader(body))

    // Calculate SHA-256 digest
    hash := sha256.Sum256(body)
    digest := base64.StdEncoding.EncodeToString(hash[:])

    // Set Content-Digest header per RFC 9530
    r.Header.Set("Content-Digest", fmt.Sprintf("sha-256=:%s:", digest))

    return nil
}
```

### Additional Security Enhancements
- **API Key Rotation**: Automated key rotation with zero-downtime migration
- **Audit Logging**: Comprehensive audit trail for all signed requests
- **Security Event Monitoring**: Real-time alerting for signature failures
- **Rate Limiting by Signature**: Track and limit requests per signing key

### Security Scanning
- Integration with Snyk or Dependabot
- SAST (Static Application Security Testing)
- DAST (Dynamic Application Security Testing)
- Container vulnerability scanning

### Libraries and Tools
```yaml
dependencies:
  - github.com/dunglas/httpsfv      # Structured Field Values (RFC 8941)
  - golang.org/x/crypto/ed25519     # EdDSA support
  - github.com/go-jose/go-jose/v3   # JOSE/JWK for key management
```

### Compliance and Standards
- **RFC 9421**: HTTP Message Signatures
- **RFC 9530**: Digest Fields (Content-Digest header)
- **RFC 8941**: Structured Field Values for HTTP
- **FIPS 186-5**: Digital Signature Standard (for RSA and ECDSA)

---

## 14. CSRF Protection for Forms 🟡 **HIGH**

### Current State
- No CSRF token implementation in the frontend form submission
- Missing CSRF validation middleware in the API
- Vulnerable to cross-site request forgery attacks

### Required Improvements

#### Backend Implementation
```go
// CSRF middleware implementation
package middleware

import (
    "crypto/rand"
    "encoding/base64"
    "github.com/gorilla/csrf"
)

// CSRF token configuration
type CSRFConfig struct {
    Secret     []byte
    CookieName string
    HeaderName string
    Secure     bool  // Use secure cookies in production
    HttpOnly   bool
    SameSite   http.SameSite
}

// CSRF middleware factory
func NewCSRFMiddleware(config CSRFConfig) func(http.Handler) http.Handler {
    return csrf.Protect(
        config.Secret,
        csrf.Secure(config.Secure),
        csrf.HttpOnly(config.HttpOnly),
        csrf.SameSite(config.SameSite),
        csrf.Path("/"),
        csrf.CookieName(config.CookieName),
        csrf.RequestHeader(config.HeaderName),
        csrf.ErrorHandler(http.HandlerFunc(csrfErrorHandler)),
    )
}

// Custom error handler for CSRF failures
func csrfErrorHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusForbidden)
    json.NewEncoder(w).Encode(map[string]string{
        "error": "CSRF token validation failed",
        "code":  "CSRF_VALIDATION_ERROR",
    })
}
```

#### Token Generation Endpoint
```go
// GET /v1/csrf-token - Endpoint to retrieve CSRF token
func (h *Handler) GetCSRFToken(w http.ResponseWriter, r *http.Request) {
    token := csrf.Token(r)

    response := struct {
        Token string `json:"csrf_token"`
        Header string `json:"header_name"`
    }{
        Token: token,
        Header: "X-CSRF-Token",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

#### Frontend Integration
```javascript
// Fetch CSRF token before form submission
class CSRFProtection {
    constructor() {
        this.token = null;
        this.headerName = 'X-CSRF-Token';
    }

    async fetchToken() {
        try {
            const response = await fetch('/v1/csrf-token', {
                credentials: 'include'
            });
            const data = await response.json();
            this.token = data.csrf_token;
            this.headerName = data.header_name;

            // Store in meta tag for form submissions
            this.updateMetaTag();

            return this.token;
        } catch (error) {
            console.error('Failed to fetch CSRF token:', error);
            throw error;
        }
    }

    updateMetaTag() {
        let meta = document.querySelector('meta[name="csrf-token"]');
        if (!meta) {
            meta = document.createElement('meta');
            meta.name = 'csrf-token';
            document.head.appendChild(meta);
        }
        meta.content = this.token;
    }

    // Add token to fetch requests
    async securedFetch(url, options = {}) {
        if (!this.token) {
            await this.fetchToken();
        }

        const headers = {
            ...options.headers,
            [this.headerName]: this.token,
        };

        return fetch(url, {
            ...options,
            headers,
            credentials: 'include'
        });
    }
}

// Usage in form submission
async function submitAnalysisForm(formData) {
    const csrf = new CSRFProtection();

    try {
        const response = await csrf.securedFetch('/v1/analyze', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(formData)
        });

        if (!response.ok) {
            if (response.status === 403) {
                // CSRF token might be expired, refresh and retry
                await csrf.fetchToken();
                return submitAnalysisForm(formData);
            }
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        return await response.json();
    } catch (error) {
        console.error('Form submission failed:', error);
        throw error;
    }
}
```

#### Double Submit Cookie Pattern (Alternative)
```go
// Alternative: Double submit cookie pattern for stateless CSRF protection
type DoubleSubmitCSRF struct {
    CookieName string
    HeaderName string
}

func (d *DoubleSubmitCSRF) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // For state-changing methods, validate token
        if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
            cookie, err := r.Cookie(d.CookieName)
            if err != nil {
                http.Error(w, "Missing CSRF cookie", http.StatusForbidden)
                return
            }

            headerToken := r.Header.Get(d.HeaderName)
            if headerToken == "" || headerToken != cookie.Value {
                http.Error(w, "Invalid CSRF token", http.StatusForbidden)
                return
            }
        }

        // Generate new token for GET requests
        if r.Method == "GET" {
            token := generateSecureToken(32)
            http.SetCookie(w, &http.Cookie{
                Name:     d.CookieName,
                Value:    token,
                Path:     "/",
                HttpOnly: false, // Must be readable by JavaScript
                Secure:   true,  // HTTPS only
                SameSite: http.SameSiteStrictMode,
            })
        }

        next.ServeHTTP(w, r)
    })
}
```

#### Configuration
```yaml
# config/security.yaml
csrf:
  enabled: true
  secret_key: "${CSRF_SECRET_KEY}"  # From environment/Vault
  cookie_name: "__csrf"
  header_name: "X-CSRF-Token"
  token_length: 32
  expiry: 4h
  secure: true  # HTTPS only in production
  same_site: "strict"
```

### Testing Strategy
```go
// Test CSRF protection
func TestCSRFProtection(t *testing.T) {
    t.Run("POST without CSRF token should fail", func(t *testing.T) {
        req := httptest.NewRequest("POST", "/v1/analyze", nil)
        rec := httptest.NewRecorder()

        handler.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusForbidden, rec.Code)
    })

    t.Run("POST with valid CSRF token should succeed", func(t *testing.T) {
        // First get CSRF token
        getReq := httptest.NewRequest("GET", "/v1/csrf-token", nil)
        getRec := httptest.NewRecorder()
        handler.ServeHTTP(getRec, getReq)

        var tokenResp struct {
            Token string `json:"csrf_token"`
        }
        json.NewDecoder(getRec.Body).Decode(&tokenResp)

        // Make POST with token
        postReq := httptest.NewRequest("POST", "/v1/analyze", nil)
        postReq.Header.Set("X-CSRF-Token", tokenResp.Token)
        postRec := httptest.NewRecorder()

        handler.ServeHTTP(postRec, postReq)

        assert.NotEqual(t, http.StatusForbidden, postRec.Code)
    })
}
```

### Benefits
- **Security**: Prevents CSRF attacks on form submissions
- **User Experience**: Transparent to end users
- **Standards Compliance**: Follows OWASP recommendations
- **SPA Compatible**: Works with single-page applications
- **Stateless Option**: Double submit cookie for horizontal scaling

### Implementation Checklist
- [ ] Add CSRF middleware to HTTP API service
- [ ] Create `/v1/csrf-token` endpoint
- [ ] Update OpenAPI specification with CSRF endpoint
- [ ] Implement frontend CSRF token handling
- [ ] Add CSRF token to all form submissions
- [ ] Configure secure cookie settings
- [ ] Add comprehensive tests for CSRF protection
- [ ] Document CSRF implementation in API docs
- [ ] Add monitoring for CSRF validation failures

---

## 15. Analysis Data Lifecycle Management - Janitor Service 🟡 **HIGH**

### Current State
- Database schema has `archived_at` and `expires_at` columns with indexes
- No application-level implementation exists
- No cleanup processes for old/expired data
- Hard delete only, no soft delete/archival capability

### Architectural Decision: Hybrid Approach

**Decision**: Keep `archived_at` and `expires_at` in the database layer while adding business logic to the application layer.

#### Rationale

**Why NOT move to virtual/computed fields:**

1. **Performance**: Partial indexes require physical columns. These critical optimizations would be lost:
   ```sql
   -- Efficient cleanup queries depend on DB columns
   CREATE INDEX idx_analysis_expired ON analysis(expires_at)
       WHERE expires_at IS NOT NULL AND archived_at IS NULL;
   CREATE INDEX idx_analysis_archived ON analysis(archived_at)
       WHERE archived_at IS NOT NULL;
   ```

2. **Multi-Service Consistency**: With 3 services (HTTP API, Publisher, Subscriber), database columns guarantee all services see identical archival state. Virtual fields could be computed differently across services.

3. **Data Lifecycle vs Business Logic**: Archival is a **data management concern** (like `created_at`, `updated_at`), not pure business logic. The state naturally belongs with the data.

4. **Migration Risk**: Removing columns requires dropping indexes, rewriting cleanup queries, and migrating existing archived records—high risk with minimal benefit.

**Why ADD application layer logic:**

1. **Business Rules**: WHEN and WHY to archive should be in testable, flexible application code
2. **Policy Configuration**: Move hardcoded "90 days" retention to configurable policies
3. **Domain Methods**: Add `analysis.CanBeArchived()`, `analysis.IsExpired()`, `analysis.ShouldRetain()`
4. **Use Cases**: Implement `ArchiveAnalysisCommand`, `CleanupExpiredRecordsCommand` with proper validation
5. **Testing**: Business logic becomes unit-testable without database overhead

**Hybrid Benefits:**
- ✅ Database columns provide performance and consistency (data state)
- ✅ Application layer provides flexibility and testability (business rules)
- ✅ Clear separation: DB handles "what is the state" / App handles "when to change state"
- ✅ No breaking changes to existing migrations or indexes
- ✅ Best of both worlds: efficiency + maintainability

#### Application-Layer Business Logic Example

```go
// Add to internal/domain/analysis.go

// Domain methods for lifecycle management
func (a *Analysis) IsExpired() bool {
    if a.ExpiresAt == nil {
        return false
    }

    return time.Now().After(*a.ExpiresAt)
}

func (a *Analysis) IsArchived() bool {
    return a.ArchivedAt != nil
}

func (a *Analysis) CanBeArchived() bool {
    // Business rules for archival eligibility
    if a.IsArchived() {
        return false // Already archived
    }

    // Only completed or failed analyses can be archived
    if a.Status != StatusCompleted && a.Status != StatusFailed {
        return false
    }

    return true
}

func (a *Analysis) ShouldRetain() bool {
    // Override expiration for important analyses
    // Example: Retain high-priority or recently accessed records
    return false // Implement custom retention logic
}

// Add to internal/usecases/commands/archive_analysis.go

type ArchiveAnalysisCommand struct {
    AnalysisID uuid.UUID
    Reason     string
}

type ArchiveAnalysisHandler struct {
    repo   ports.AnalysisRepository
    logger Logger
}

func (h *ArchiveAnalysisHandler) Handle(ctx context.Context, cmd ArchiveAnalysisCommand) error {
    analysis, err := h.repo.GetByID(ctx, cmd.AnalysisID.String())
    if err != nil {

        return fmt.Errorf("analysis not found: %w", err)
    }

    // Business logic validation
    if !analysis.CanBeArchived() {
        return fmt.Errorf("analysis cannot be archived: status=%s", analysis.Status)
    }

    // Repository operation
    if err := h.repo.Archive(ctx, cmd.AnalysisID.String()); err != nil {
        return fmt.Errorf("failed to archive: %w", err)
    }

    h.logger.Info("Analysis archived", "id", cmd.AnalysisID, "reason", cmd.Reason)

    return nil
}
```

### Janitor Service Design (Inspired by Ory Hydra)

Taking inspiration from [Ory Hydra's janitor pattern](https://www.ory.com/docs/hydra/cli/hydra-janitor), this service provides safe, batch-based cleanup of stale analysis records with multiple safety mechanisms.

### Required Improvements

#### CLI Tool Implementation (`cmd/janitor/main.go`)
```go
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "time"

    "github.com/architeacher/svc-web-analyzer/internal/config"
    "github.com/architeacher/svc-web-analyzer/internal/adapters/repos"
)

type JanitorFlags struct {
    // Database connection
    DSN            string
    ReadFromEnv    bool
    ConfigFile     string

    // Safety controls
    KeepIfYounger  time.Duration
    DryRun         bool
    Force          bool

    // Processing controls
    BatchSize      int
    QueryLimit     int
    SleepBetween   time.Duration

    // Selective cleanup
    CleanExpired   bool
    CleanArchived  bool
    AutoArchive    bool
}

func main() {
    flags := parseFlags()

    if !flags.Force && !flags.DryRun {
        fmt.Println("⚠️  WARNING: This operation is IRREVERSIBLE!")
        fmt.Println("Records will be permanently deleted from the database.")
        fmt.Println("Use --dry-run first to preview changes.")
        fmt.Print("\nType 'yes' to continue: ")

        var confirm string
        fmt.Scanln(&confirm)
        if confirm != "yes" {
            log.Fatal("Operation cancelled")
        }
    }

    janitor := NewJanitor(flags)
    stats := janitor.Run(context.Background())

    fmt.Printf("\n✅ Janitor completed:\n")
    fmt.Printf("  - Records examined: %d\n", stats.Examined)
    fmt.Printf("  - Records deleted: %d\n", stats.Deleted)
    fmt.Printf("  - Records archived: %d\n", stats.Archived)
    fmt.Printf("  - Errors: %d\n", stats.Errors)
}
```

#### Janitor Service Logic
```go
type Janitor struct {
    repo   *repos.AnalysisRepository
    config JanitorConfig
}

type JanitorConfig struct {
    KeepIfYounger   time.Duration
    BatchSize       int
    QueryLimit      int
    DryRun          bool
    SleepBetween    time.Duration
}

func (j *Janitor) CleanExpired(ctx context.Context) error {
    cutoffTime := time.Now().Add(-j.config.KeepIfYounger)

    for {
        // Query batch of expired records
        records, err := j.repo.FindExpiredBatch(ctx,
            cutoffTime,
            j.config.QueryLimit,
        )

        if err != nil {
            return fmt.Errorf("failed to query expired records: %w", err)
        }

        if len(records) == 0 {
            break // No more records to process
        }

        // Process in smaller batches to avoid table locks
        for i := 0; i < len(records); i += j.config.BatchSize {
            end := min(i+j.config.BatchSize, len(records))
            batch := records[i:end]

            if j.config.DryRun {
                log.Printf("[DRY RUN] Would delete %d records", len(batch))
                continue
            }

            if err := j.repo.DeleteBatch(ctx, batch); err != nil {
                log.Printf("Failed to delete batch: %v", err)
                continue
            }

            log.Printf("Deleted %d expired records", len(batch))

            // Sleep between batches to reduce database load
            if j.config.SleepBetween > 0 {
                time.Sleep(j.config.SleepBetween)
            }
        }
    }

    return nil
}

func (j *Janitor) AutoArchiveOldAnalyses(ctx context.Context) error {
    // Archive completed analyses older than threshold
    threshold := time.Now().Add(-30 * 24 * time.Hour) // 30 days

    query := `
        UPDATE analysis
        SET archived_at = NOW()
        WHERE status = 'completed'
        AND completed_at < $1
        AND archived_at IS NULL
        LIMIT $2
    `

    // Archive in batches
    for {
        result, err := j.repo.DB.ExecContext(ctx, query, threshold, j.config.BatchSize)
        if err != nil {
            return err
        }

        affected, _ := result.RowsAffected()
        if affected == 0 {
            break
        }

        log.Printf("Auto-archived %d old analyses", affected)
        time.Sleep(j.config.SleepBetween)
    }

    return nil
}
```

#### Repository Methods
```go
// Add to internal/adapters/repos/analysis_repository.go

func (r *AnalysisRepository) Archive(ctx context.Context, analysisID string) error {
    query := `
        UPDATE analysis
        SET archived_at = NOW()
        WHERE id = $1 AND archived_at IS NULL
    `
    _, err := r.conn.ExecContext(ctx, query, analysisID)
    return err
}

func (r *AnalysisRepository) Unarchive(ctx context.Context, analysisID string) error {
    query := `
        UPDATE analysis
        SET archived_at = NULL
        WHERE id = $1
    `
    _, err := r.conn.ExecContext(ctx, query, analysisID)
    return err
}

func (r *AnalysisRepository) SetExpiration(ctx context.Context, analysisID string, expiresAt time.Time) error {
    query := `
        UPDATE analysis
        SET expires_at = $2
        WHERE id = $1
    `
    _, err := r.conn.ExecContext(ctx, query, analysisID, expiresAt)
    return err
}

func (r *AnalysisRepository) FindExpiredBatch(ctx context.Context, keepIfYounger time.Time, limit int) ([]uuid.UUID, error) {
    query := `
        SELECT id FROM analysis
        WHERE expires_at IS NOT NULL
        AND expires_at < NOW()
        AND archived_at IS NULL
        AND created_at < $1
        ORDER BY expires_at ASC
        LIMIT $2
    `

    var ids []uuid.UUID
    rows, err := r.conn.QueryContext(ctx, query, keepIfYounger, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }

    return ids, nil
}

func (r *AnalysisRepository) DeleteBatch(ctx context.Context, ids []uuid.UUID) error {
    if len(ids) == 0 {
        return nil
    }

    query := `DELETE FROM analysis WHERE id = ANY($1)`
    _, err := r.conn.ExecContext(ctx, query, pq.Array(ids))
    return err
}
```

#### CLI Flags and Configuration
```yaml
# Example usage patterns

# Dry run to preview what would be deleted
./janitor --dry-run --keep-if-younger=24h

# Clean expired records only
./janitor --clean-expired --batch-size=50 --keep-if-younger=7d

# Auto-archive old completed analyses
./janitor --auto-archive --keep-if-younger=30d

# Full cleanup with confirmation
./janitor --clean-expired --clean-archived --force

# Read config from environment
./janitor --read-from-env --config=/etc/janitor/config.yaml
```

#### Configuration Structure
```go
// Add to internal/config/settings.go
type JanitorConfig struct {
    Enabled         bool          `envconfig:"JANITOR_ENABLED" default:"false"`
    Schedule        string        `envconfig:"JANITOR_SCHEDULE" default:"0 2 * * *"` // 2 AM daily
    KeepIfYounger   time.Duration `envconfig:"JANITOR_KEEP_IF_YOUNGER" default:"168h"` // 7 days
    BatchSize       int           `envconfig:"JANITOR_BATCH_SIZE" default:"100"`
    QueryLimit      int           `envconfig:"JANITOR_QUERY_LIMIT" default:"10000"`
    SleepBetween    time.Duration `envconfig:"JANITOR_SLEEP_BETWEEN" default:"100ms"`
    AutoArchiveAge  time.Duration `envconfig:"JANITOR_AUTO_ARCHIVE_AGE" default:"720h"` // 30 days
    DryRun          bool          `envconfig:"JANITOR_DRY_RUN" default:"false"`
}
```

#### API Endpoints for Manual Archival
```go
// Add to request_handler.go

// Archive an analysis (soft delete)
func (h *Handler) ArchiveAnalysis(w http.ResponseWriter, r *http.Request) {
    analysisID := chi.URLParam(r, "analysisId")

    if err := h.repo.Archive(r.Context(), analysisID); err != nil {
        http.Error(w, "Failed to archive analysis", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// Restore archived analysis
func (h *Handler) UnarchiveAnalysis(w http.ResponseWriter, r *http.Request) {
    analysisID := chi.URLParam(r, "analysisId")

    if err := h.repo.Unarchive(r.Context(), analysisID); err != nil {
        http.Error(w, "Failed to unarchive analysis", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// Set custom expiration
func (h *Handler) SetAnalysisExpiration(w http.ResponseWriter, r *http.Request) {
    analysisID := chi.URLParam(r, "analysisId")

    var req struct {
        ExpiresAt time.Time `json:"expires_at"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    if err := h.repo.SetExpiration(r.Context(), analysisID, req.ExpiresAt); err != nil {
        http.Error(w, "Failed to set expiration", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

#### Domain Model Updates
```go
// Update internal/domain/analysis.go
type Analysis struct {
    ID          uuid.UUID      `json:"analysis_id"`
    URL         string         `json:"url"`
    Status      AnalysisStatus `json:"status"`
    ContentHash string         `json:"content_hash,omitempty"`
    ContentSize int64          `json:"content_size,omitempty"`
    CreatedAt   time.Time      `json:"created_at"`
    CompletedAt *time.Time     `json:"completed_at,omitempty"`
    ArchivedAt  *time.Time     `json:"archived_at,omitempty"`  // NEW
    ExpiresAt   *time.Time     `json:"expires_at,omitempty"`   // NEW
    Duration    *time.Duration `json:"duration,omitempty"`
    Results     *AnalysisData  `json:"results,omitempty"`
    Error       *AnalysisError `json:"error,omitempty"`
    LockVersion int            `json:"-"`
}
```

### Safety Features (Inspired by Hydra)

1. **Irreversibility Warning**: Prominent warnings before destructive operations
2. **Dry Run Mode**: Preview what would be deleted without actual deletion
3. **Keep-If-Younger**: Never delete records newer than threshold
4. **Batch Processing**: Prevent table locks on large databases
5. **Audit Logging**: Log all deletion operations for compliance
6. **Confirmation Required**: Interactive confirmation for non-dry-run operations
7. **Selective Cleanup**: Target specific record types (expired vs archived)

### Deployment Options

#### Option 1: Cron Job
```yaml
# Kubernetes CronJob
apiVersion: batch/v1
kind: CronJob
metadata:
  name: analysis-janitor
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: janitor
            image: web-analyzer/janitor:latest
            command:
              - /janitor
              - --read-from-env
              - --keep-if-younger=7d
              - --clean-expired
              - --auto-archive
```

#### Option 2: Systemd Timer
```ini
# /etc/systemd/system/analysis-janitor.timer
[Unit]
Description=Daily Analysis Cleanup
Requires=analysis-janitor.service

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

#### Option 3: Docker Compose Service
```yaml
janitor:
  image: web-analyzer/janitor:latest
  environment:
    - POSTGRES_HOST=postgres
    - JANITOR_KEEP_IF_YOUNGER=168h
    - JANITOR_BATCH_SIZE=100
  command: |
    sh -c 'while true; do
      /janitor --clean-expired --auto-archive;
      sleep 86400;
    done'
```

### Monitoring and Metrics

```go
// Prometheus metrics for janitor operations
var (
    janitorRunsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "janitor_runs_total",
            Help: "Total number of janitor runs",
        },
        []string{"status"},
    )

    janitorRecordsDeleted = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "janitor_records_deleted_total",
            Help: "Total records deleted by janitor",
        },
    )

    janitorDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name: "janitor_duration_seconds",
            Help: "Duration of janitor runs",
        },
    )
)
```

### Testing Strategy

```go
func TestJanitor(t *testing.T) {
    t.Run("Should not delete recent records", func(t *testing.T) {
        // Create record with expires_at = now - 1 hour
        // Run janitor with keep-if-younger = 2 hours
        // Verify record still exists
    })

    t.Run("Should delete expired old records", func(t *testing.T) {
        // Create record with expires_at = now - 1 day
        // Run janitor with keep-if-younger = 1 hour
        // Verify record deleted
    })

    t.Run("Should respect batch size", func(t *testing.T) {
        // Create 1000 expired records
        // Run janitor with batch-size = 10
        // Verify deletion happens in batches
    })

    t.Run("Dry run should not delete", func(t *testing.T) {
        // Run janitor with --dry-run
        // Verify no records deleted
    })
}
```

### Benefits

- **Production Safety**: Multiple safeguards prevent accidental data loss
- **Performance**: Batch processing prevents database lock issues
- **Flexibility**: CLI tool can run as cron job, systemd service, or container
- **Compliance**: Audit trail for all deletions
- **Operations**: Easy to test with dry-run mode
- **Scalability**: Handles millions of records without blocking operations

### Implementation Checklist

**Database Layer (Keep existing):**
- [x] Database columns `archived_at` and `expires_at` already exist
- [x] Partial indexes for performance already created
- [ ] No changes needed to migrations (hybrid approach preserves DB layer)

**Domain Layer (Add business logic):**
- [ ] Update domain model `Analysis` struct with `ArchivedAt` and `ExpiresAt` fields
- [ ] Add domain methods to `Analysis`:
  - [ ] `CanBeArchived() bool` - business rules for archival eligibility
  - [ ] `IsExpired() bool` - check if record has passed expiration
  - [ ] `ShouldRetain() bool` - determine if record should be kept despite expiration
  - [ ] `MarkAsArchived(reason string)` - domain operation with validation
- [ ] Create archival value objects for type safety (`ArchivalReason`, `RetentionPolicy`)

**Use Cases (CQRS commands):**
- [ ] Implement `ArchiveAnalysisCommand` with validation and business rules
- [ ] Implement `UnarchiveAnalysisCommand` with authorization checks
- [ ] Implement `SetExpirationCommand` with policy validation
- [ ] Implement `CleanupExpiredRecordsCommand` with safety controls
- [ ] Add command decorators (logging, metrics, validation)

**Repository Layer:**
- [ ] Add `Archive(ctx, analysisID)` method
- [ ] Add `Unarchive(ctx, analysisID)` method
- [ ] Add `SetExpiration(ctx, analysisID, expiresAt)` method
- [ ] Add `FindExpiredBatch(ctx, keepIfYounger, limit)` method
- [ ] Add `DeleteBatch(ctx, ids)` method
- [ ] Update existing queries to filter `archived_at IS NULL` where appropriate

**Configuration:**
- [ ] Add `JanitorConfig` to settings with configurable retention policies
- [ ] Move hardcoded "90 days" to environment configuration
- [ ] Add policy-based retention rules (by status, priority, etc.)

**Janitor Service:**
- [ ] Create `cmd/janitor/main.go` with CLI interface
- [ ] Implement safety features (dry-run, confirmation, keep-if-younger)
- [ ] Add batch processing with configurable limits
- [ ] Implement auto-archival logic for old completed analyses

**API Endpoints:**
- [ ] `POST /v1/analysis/{analysisId}/archive` - Manual archival
- [ ] `POST /v1/analysis/{analysisId}/unarchive` - Restore archived record
- [ ] `PUT /v1/analysis/{analysisId}/expiration` - Set custom expiration
- [ ] Update OpenAPI specification with new endpoints

**Monitoring & Observability:**
- [ ] Add Prometheus metrics for janitor operations
- [ ] Add metrics for archival actions (manual, auto, batch)
- [ ] Create alerts for cleanup failures
- [ ] Add structured logging for all lifecycle events

**Testing:**
- [ ] Unit tests for domain methods (CanBeArchived, IsExpired, etc.)
- [ ] Unit tests for use case commands with mocked repositories
- [ ] Integration tests for janitor with testcontainers
- [ ] Test safety features (dry-run, batch processing, keep-if-younger)
- [ ] Test multi-service consistency scenarios

**Deployment:**
- [ ] Create deployment manifests (K8s CronJob, Docker Compose, systemd)
- [ ] Add janitor to CI/CD pipeline
- [ ] Document janitor usage and safety procedures
- [ ] Create runbook for operational procedures

---

## 16. API Gateway with Full HTTP→gRPC Protocol Conversion 🟡 **HIGH**

### Current State
- Three separate services (HTTP API, Publisher, Subscriber)
- HTTP API service handles cross-cutting concerns (auth, rate limiting, validation)
- No centralized gateway for traffic management
- No protocol conversion capabilities
- No load shedding or advanced resilience patterns

### Proposed Architecture

```
Client (HTTP/SSE) → API Gateway → Backend Services
                    ├─ HTTP→gRPC conversion
                    ├─ 19 Gateway Features
                    └─ Centralized management
                                    ↓
                            gRPC API Service
                            ├─ Business logic
                            ├─ CQRS handlers
                            └─ Database operations
```

### Gateway Features (19)

Based on industry best practices from ByteByteGo's API Gateway patterns, this implementation includes:

#### 1. Request Processing & Routing
1. **Parameter Validation** - Deep request validation using OpenAPI 3.0.3 schema (query params, path params, request body)
2. **Dynamic Request Routing** - Path-based routing with pattern matching and service discovery integration
3. **Protocol Conversion** - HTTP/1.1 → gRPC translation with bidirectional message mapping
4. **SSE→gRPC Streaming** - Convert gRPC server streams to Server-Sent Events for real-time updates

#### 2. Security & Access Control
5. **Authentication** - PASETO v4 token validation and claims extraction
6. **Authorization** - Role-based access control (RBAC) and policy-based authorization with fine-grained permissions
7. **Request Signing** - HTTP message signature verification (RFC 9421) for request integrity and non-repudiation
8. **Allow/Deny List (ACL)** - IP/CIDR/user-agent filtering with whitelist/blacklist support
9. **Data Encryption** - TLS 1.3 for client connections, mTLS for backend service communication
10. **Security Headers** - CORS, CSP, HSTS, X-Frame-Options, X-Content-Type-Options, Referrer-Policy

#### 3. Traffic Management & Resilience
11. **Rate Limiting** - Token bucket algorithm with KeyDB backend (per client/endpoint/global limits)
12. **Load Shedding** ⭐ NEW - Adaptive probabilistic request dropping under overload
13. **Circuit Breaker** - Per-service failure detection with half-open state recovery patterns
14. **Load Balancing** - Multiple strategies (round-robin, weighted, least-connections, consistent hashing)
15. **Service Discovery** - Dynamic backend service registration and health-aware routing

#### 4. Performance Optimization
16. **Response Caching** - KeyDB-backed caching with Cache-Control/ETag support and cache invalidation
17. **API Composition** - Combine multiple gRPC calls into single HTTP response (scatter-gather pattern)

#### 5. Observability & Operations
18. **Error Handling** - Retry logic with exponential backoff, fallback responses, error translation
19. **Logging & Monitoring** - Structured access logs, OpenTelemetry metrics (latency, throughput, errors), distributed tracing with Jaeger

**References:**
- [ByteByteGo: What Does API Gateway Do?](https://blog.bytebytego.com/i/154117333/what-does-api-gateway-do)
- [ByteByteGo: API Gateway 101](https://bytebytego.com/guides/api-gateway-101/)

### Required Improvements

#### Project Structure Changes

```
docs/
  ├── openapi-spec/           # Existing: OpenAPI 3.0.3 specification
  └── proto/                  # NEW: Protobuf definitions (API contracts)
      └── analysis/
          └── v1/
              ├── analysis.proto
              ├── health.proto
              └── events.proto

cmd/
  ├── gateway/                # NEW: API Gateway service
  │   └── main.go
  ├── svc-web-analyzer/       # MODIFIED: Now gRPC service
  ├── publisher/              # Unchanged
  └── subscriber/             # Unchanged

internal/
  ├── gateway/                # NEW: Gateway implementation
  │   ├── middleware/         # All 19 gateway features
  │   │   ├── auth.go                    # Authentication (PASETO validation)
  │   │   ├── authz.go                   # Authorization (RBAC/policy-based)
  │   │   ├── signing.go                 # Request signing (RFC 9421)
  │   │   ├── ratelimit.go               # Rate limiting (token bucket)
  │   │   ├── loadshedding.go            # Load shedding (adaptive)
  │   │   ├── circuitbreaker.go          # Circuit breaker pattern
  │   │   ├── cache.go                   # Response caching
  │   │   ├── acl.go                     # Allow/Deny list filtering
  │   │   ├── security.go                # Security headers (CORS, CSP, etc.)
  │   │   ├── tls.go                     # TLS/mTLS configuration
  │   │   ├── errorhandler.go            # Error handling & retry logic
  │   │   ├── logger.go                  # Access logging & tracing
  │   │   └── validator.go               # Parameter validation (OpenAPI)
  │   ├── router/             # Service discovery & dynamic routing
  │   │   ├── discovery.go               # Service registry and health checks
  │   │   ├── loadbalancer.go            # Load balancing strategies
  │   │   └── router.go                  # Dynamic request routing
  │   ├── proxy/              # Protocol conversion & composition
  │   │   ├── http_grpc.go               # HTTP→gRPC protocol conversion
  │   │   ├── streaming.go               # SSE→gRPC streaming
  │   │   └── composer.go                # API composition (scatter-gather)
  │   └── config/
  │       └── gateway_config.go
  ├── adapters/
  │   ├── grpc/               # NEW: gRPC handlers (replaces http/)
  │   │   ├── server.go
  │   │   ├── handlers.go
  │   │   ├── streaming.go
  │   │   └── pb/             # Generated protobuf code
  │   ├── middleware/         # DELETED: Moved to gateway
  │   └── http/               # DELETED: Replaced by grpc/
  └── [rest unchanged]
```

#### gRPC Service Definition

```protobuf
// docs/proto/analysis/v1/analysis.proto
syntax = "proto3";

package analysis.v1;

option go_package = "github.com/architeacher/svc-web-analyzer/internal/adapters/grpc/pb;pb";

service AnalysisService {
  // Unary RPCs
  rpc AnalyzeURL(AnalyzeRequest) returns (AnalyzeResponse);
  rpc GetAnalysis(GetAnalysisRequest) returns (Analysis);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
  rpc ReadinessCheck(ReadinessCheckRequest) returns (ReadinessCheckResponse);
  rpc LivenessCheck(LivenessCheckRequest) returns (LivenessCheckResponse);

  // Server streaming for SSE conversion
  rpc StreamAnalysisEvents(StreamEventsRequest) returns (stream AnalysisEvent);
}

message AnalyzeRequest {
  string url = 1;
  AnalysisOptions options = 2;
}

message AnalysisOptions {
  bool include_headings = 1;
  bool check_links = 2;
  bool detect_forms = 3;
  int32 timeout = 4;
}

message AnalyzeResponse {
  string analysis_id = 1;
  string status = 2;
  google.protobuf.Timestamp created_at = 3;
}

message StreamEventsRequest {
  string analysis_id = 1;
}

message AnalysisEvent {
  string type = 1;
  google.protobuf.Struct payload = 2;
  google.protobuf.Timestamp timestamp = 3;
}
```

#### Load Shedding Implementation

Load shedding prevents service overload by adaptively dropping requests based on system health metrics.

**Algorithm: Adaptive Probabilistic Shedding**

```go
package middleware

import (
    "math"
    "math/rand"
    "net/http"
    "runtime"
    "sync/atomic"
    "time"
)

type LoadShedder struct {
    config        LoadSheddingConfig
    queueDepth    int64
    metrics       *MetricsCollector
}

type LoadSheddingConfig struct {
    Enabled             bool
    MaxQueueDepth       int
    MaxLatencyP99       time.Duration
    MaxCPUPercent       float64
    ShedThreshold       float64  // Start shedding at this load factor (0.7 = 70%)
    MetricsWindow       time.Duration
}

func (ls *LoadShedder) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !ls.config.Enabled {
            next.ServeHTTP(w, r)
            return
        }

        // Calculate load factor from multiple signals
        loadFactor := ls.calculateLoadFactor()

        // Probabilistic shedding above threshold
        if loadFactor > ls.config.ShedThreshold {
            shedProbability := (loadFactor - ls.config.ShedThreshold) / (1.0 - ls.config.ShedThreshold)

            if rand.Float64() < shedProbability {
                ls.metrics.IncrementLoadSheddingDrops()

                // Return 503 with Retry-After header
                w.Header().Set("Retry-After", "5")
                w.Header().Set("X-Load-Shedding", "true")
                http.Error(w, "Service temporarily overloaded", http.StatusServiceUnavailable)
                return
            }
        }

        // Track request in queue
        atomic.AddInt64(&ls.queueDepth, 1)
        defer atomic.AddInt64(&ls.queueDepth, -1)

        next.ServeHTTP(w, r)
    })
}

func (ls *LoadShedder) calculateLoadFactor() float64 {
    // Signal 1: Queue depth
    queueLoad := float64(atomic.LoadInt64(&ls.queueDepth)) / float64(ls.config.MaxQueueDepth)

    // Signal 2: Response latency (p99)
    p99Latency := ls.metrics.GetLatencyP99()
    latencyLoad := float64(p99Latency) / float64(ls.config.MaxLatencyP99)

    // Signal 3: CPU usage
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)
    cpuLoad := float64(runtime.NumGoroutine()) / 1000.0 // Simplified CPU proxy

    // Return maximum load factor (most constrained resource)
    return math.Max(math.Max(queueLoad, latencyLoad), cpuLoad)
}
```

**Configuration:**

```yaml
# config/gateway.yaml
load_shedding:
  enabled: true
  max_queue_depth: 100
  max_latency_p99: 5s
  max_cpu_percent: 0.8
  shed_threshold: 0.7        # Start shedding at 70% load
  metrics_window: 30s
```

**Benefits:**
- Prevents cascading failures under overload
- Maintains service availability for accepted requests
- Graceful degradation with retry hints
- Multiple signal inputs (queue, latency, CPU)
- Configurable thresholds per environment

#### Authorization Implementation (RBAC)

Role-Based Access Control for fine-grained authorization beyond authentication.

**Algorithm: Policy-Based Authorization**

```go
package middleware

import (
    "context"
    "net/http"
    "strings"
)

type Authorizer struct {
    policies PolicyStore
    cache    *Cache
}

type PolicyStore interface {
    GetUserRoles(ctx context.Context, userID string) ([]string, error)
    CheckPermission(ctx context.Context, role, resource, action string) (bool, error)
}

type AuthzConfig struct {
    Enabled         bool
    CacheTTL        time.Duration
    DefaultDeny     bool
}

func (a *Authorizer) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract user from context (set by auth middleware)
        userID := r.Context().Value("user_id").(string)

        // Get user roles (cached)
        roles, err := a.getUserRoles(r.Context(), userID)
        if err != nil {
            http.Error(w, "Authorization failed", http.StatusForbidden)
            return
        }

        // Extract resource and action from request
        resource := extractResource(r.URL.Path)  // e.g., "analysis"
        action := mapHTTPMethod(r.Method)        // e.g., "read", "write"

        // Check permissions
        hasPermission := false
        for _, role := range roles {
            allowed, _ := a.policies.CheckPermission(r.Context(), role, resource, action)
            if allowed {
                hasPermission = true
                break
            }
        }

        if !hasPermission {
            http.Error(w, "Insufficient permissions", http.StatusForbidden)
            return
        }

        // Add roles to context for downstream use
        ctx := context.WithValue(r.Context(), "user_roles", roles)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func mapHTTPMethod(method string) string {
    switch method {
    case http.MethodGet:
        return "read"
    case http.MethodPost:
        return "create"
    case http.MethodPut, http.MethodPatch:
        return "update"
    case http.MethodDelete:
        return "delete"
    default:
        return "unknown"
    }
}
```

**Configuration:**

```yaml
# config/gateway.yaml
authorization:
  enabled: true
  cache_ttl: 5m
  default_deny: true
  policies:
    - role: "admin"
      permissions:
        - resource: "*"
          actions: ["read", "write", "delete"]
    - role: "user"
      permissions:
        - resource: "analysis"
          actions: ["read", "create"]
    - role: "readonly"
      permissions:
        - resource: "analysis"
          actions: ["read"]
```

#### TLS/mTLS Configuration

Secure communication for client connections and backend services.

**Implementation: TLS 1.3 + mTLS**

```go
package middleware

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "net/http"
    "os"
)

type TLSConfig struct {
    // Client-facing TLS
    ClientTLSEnabled    bool
    CertFile            string
    KeyFile             string
    MinTLSVersion       uint16
    CipherSuites        []uint16

    // Backend mTLS
    BackendMTLSEnabled  bool
    ClientCAFile        string
    ClientCertFile      string
    ClientKeyFile       string
    VerifyBackend       bool
}

func (tc *TLSConfig) CreateClientTLSConfig() (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
    }

    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_AES_128_GCM_SHA256,
            tls.TLS_AES_256_GCM_SHA384,
            tls.TLS_CHACHA20_POLY1305_SHA256,
        },
        PreferServerCipherSuites: true,
        CurvePreferences: []tls.CurveID{
            tls.X25519,
            tls.CurveP256,
        },
    }, nil
}

func (tc *TLSConfig) CreateBackendMTLSConfig() (*tls.Config, error) {
    // Load client certificate for mTLS
    cert, err := tls.LoadX509KeyPair(tc.ClientCertFile, tc.ClientKeyFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load client certificate: %w", err)
    }

    // Load CA certificate for backend verification
    caCert, err := os.ReadFile(tc.ClientCAFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load CA certificate: %w", err)
    }

    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)

    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      caCertPool,
        MinVersion:   tls.VersionTLS13,
        ClientAuth:   tls.RequireAndVerifyClientCert,
    }, nil
}
```

**Configuration:**

```yaml
# config/gateway.yaml
tls:
  # Client-facing TLS
  client:
    enabled: true
    cert_file: "/certs/gateway.crt"
    key_file: "/certs/gateway.key"
    min_version: "1.3"

  # Backend mTLS
  backend:
    mtls_enabled: true
    client_cert: "/certs/gateway-client.crt"
    client_key: "/certs/gateway-client.key"
    ca_file: "/certs/backend-ca.crt"
    verify_backend: true
```

#### Service Discovery Implementation

Dynamic backend service registration and health-aware routing.

**Algorithm: Health-Aware Service Registry**

```go
package router

import (
    "context"
    "sync"
    "time"
)

type ServiceRegistry struct {
    mu       sync.RWMutex
    services map[string][]*ServiceInstance
    health   *HealthChecker
}

type ServiceInstance struct {
    ID       string
    Address  string
    Port     int
    Metadata map[string]string
    Health   HealthStatus
    LastSeen time.Time
}

type HealthStatus string

const (
    HealthStatusHealthy   HealthStatus = "healthy"
    HealthStatusUnhealthy HealthStatus = "unhealthy"
    HealthStatusDraining  HealthStatus = "draining"
)

func (sr *ServiceRegistry) Register(serviceName string, instance *ServiceInstance) error {
    sr.mu.Lock()
    defer sr.mu.Unlock()

    if sr.services[serviceName] == nil {
        sr.services[serviceName] = make([]*ServiceInstance, 0)
    }

    // Update or add instance
    for i, existing := range sr.services[serviceName] {
        if existing.ID == instance.ID {
            sr.services[serviceName][i] = instance
            return nil
        }
    }

    sr.services[serviceName] = append(sr.services[serviceName], instance)

    // Start health checking
    go sr.health.Monitor(instance)

    return nil
}

func (sr *ServiceRegistry) GetHealthyInstances(serviceName string) []*ServiceInstance {
    sr.mu.RLock()
    defer sr.mu.RUnlock()

    instances := sr.services[serviceName]
    healthy := make([]*ServiceInstance, 0)

    for _, instance := range instances {
        if instance.Health == HealthStatusHealthy {
            healthy = append(healthy, instance)
        }
    }

    return healthy
}

func (sr *ServiceRegistry) Deregister(serviceName, instanceID string) error {
    sr.mu.Lock()
    defer sr.mu.Unlock()

    instances := sr.services[serviceName]
    for i, instance := range instances {
        if instance.ID == instanceID {
            sr.services[serviceName] = append(instances[:i], instances[i+1:]...)
            return nil
        }
    }

    return fmt.Errorf("instance not found: %s", instanceID)
}

type HealthChecker struct {
    interval time.Duration
    timeout  time.Duration
}

func (hc *HealthChecker) Monitor(instance *ServiceInstance) {
    ticker := time.NewTicker(hc.interval)
    defer ticker.Stop()

    for range ticker.C {
        ctx, cancel := context.WithTimeout(context.Background(), hc.timeout)

        // gRPC health check
        healthy := hc.checkHealth(ctx, instance.Address, instance.Port)

        if healthy {
            instance.Health = HealthStatusHealthy
            instance.LastSeen = time.Now()
        } else {
            instance.Health = HealthStatusUnhealthy
        }

        cancel()
    }
}
```

**Configuration:**

```yaml
# config/gateway.yaml
service_discovery:
  enabled: true
  health_check:
    interval: 10s
    timeout: 5s
    unhealthy_threshold: 3
  services:
    - name: "analysis-service"
      instances:
        - id: "analysis-1"
          address: "api"
          port: 9090
          metadata:
            zone: "us-east-1a"
        - id: "analysis-2"
          address: "api-replica"
          port: 9090
          metadata:
            zone: "us-east-1b"
```

#### Dynamic Request Routing

Path-based routing with pattern matching and service resolution.

**Implementation: Pattern-Based Router**

```go
package router

import (
    "fmt"
    "net/http"
    "regexp"
    "strings"
)

type Router struct {
    routes   []*Route
    registry *ServiceRegistry
}

type Route struct {
    Pattern     *regexp.Regexp
    Service     string
    Method      string
    Rewrite     string
    Metadata    map[string]string
}

func (r *Router) AddRoute(pattern, service, method string) error {
    regex, err := regexp.Compile(pattern)
    if err != nil {
        return fmt.Errorf("invalid pattern: %w", err)
    }

    r.routes = append(r.routes, &Route{
        Pattern: regex,
        Service: service,
        Method:  method,
    })

    return nil
}

func (r *Router) Match(req *http.Request) (*Route, *ServiceInstance, error) {
    path := req.URL.Path
    method := req.Method

    // Find matching route
    var matchedRoute *Route
    for _, route := range r.routes {
        if route.Pattern.MatchString(path) {
            if route.Method == "" || route.Method == method {
                matchedRoute = route
                break
            }
        }
    }

    if matchedRoute == nil {
        return nil, nil, fmt.Errorf("no route found for %s %s", method, path)
    }

    // Get healthy backend instance
    instances := r.registry.GetHealthyInstances(matchedRoute.Service)
    if len(instances) == 0 {
        return nil, nil, fmt.Errorf("no healthy instances for service: %s", matchedRoute.Service)
    }

    // Load balancer selects instance (handled by load balancer)
    instance := instances[0]

    return matchedRoute, instance, nil
}

func (r *Router) Rewrite(route *Route, path string) string {
    if route.Rewrite == "" {
        return path
    }

    return route.Pattern.ReplaceAllString(path, route.Rewrite)
}
```

**Configuration:**

```yaml
# config/gateway.yaml
routing:
  rules:
    - pattern: "^/v1/analyze$"
      service: "analysis-service"
      method: "POST"
      timeout: 30s

    - pattern: "^/v1/analysis/([^/]+)$"
      service: "analysis-service"
      method: "GET"
      rewrite: "/api/analysis/$1"

    - pattern: "^/v1/analysis/([^/]+)/events$"
      service: "analysis-service"
      method: "GET"
      streaming: true

    - pattern: "^/v1/health$"
      service: "health-service"
      method: "GET"
      cache_ttl: 10s
```

#### Backend Service Conversion

**Remove from HTTP API Service:**
- ❌ All HTTP middleware (lines 103-174 in `internal/runtime/deps.go`)
- ❌ `internal/adapters/http/` (request_handler.go, handlers/)
- ❌ `internal/adapters/middleware/` (all middleware files)
- ❌ Chi router setup

**Add to Backend Service:**
- ✅ gRPC server initialization
- ✅ gRPC service implementation (handlers that call existing CQRS commands/queries)
- ✅ gRPC streaming for SSE events
- ✅ Proto definitions in `docs/proto/`

**Keep Unchanged:**
- ✅ CQRS handlers (commands/queries)
- ✅ Business logic (usecases, service layer)
- ✅ Domain models
- ✅ Database operations
- ✅ Outbox pattern
- ✅ Publisher/Subscriber services

#### Gateway Middleware Chain

**Request Flow (Order Matters):**

Based on industry best practices, the middleware execution order is critical for optimal performance and security:

```
┌─────────────────────────────────────────────────────────────────┐
│ INCOMING REQUEST                                                 │
└─────────────────────────────────────────────────────────────────┘
                               ↓
1. TLS Termination              → Decrypt client TLS connections
2. Logging (start)              → Record request timestamp, ID, headers
3. Panic Recovery               → Catch and handle runtime panics
4. CORS/Security Headers        → Set CSP, HSTS, X-Frame-Options, etc.
5. ACL (Allow/Deny List)        → IP/CIDR/user-agent filtering
6. Load Shedding ⭐             → Drop requests under overload (adaptive)
7. Rate Limiting                → Token bucket per client/endpoint
8. Authentication (PASETO)      → Validate token, extract claims
9. Request Signing (RFC 9421)   → Verify HTTP message signatures
10. Authorization (RBAC)        → Check permissions and policies
11. Parameter Validation        → OpenAPI schema validation (deep)
12. Response Cache Check        → Check KeyDB for cached response
13. Circuit Breaker             → Check backend health status
14. Service Discovery           → Resolve backend service instances
15. Load Balancer Selection     → Choose backend instance (strategy)
16. Dynamic Request Routing     → Route to correct service/endpoint
17. HTTP→gRPC Conversion        → Translate HTTP to gRPC messages
18. API Composition             → Combine multiple gRPC calls if needed
19. mTLS to Backend             → Establish secure backend connection
20. Proxy to Backend            → Forward gRPC request
                               ↓
                          [Backend Service]
                               ↓
21. gRPC→HTTP Conversion        → Translate gRPC response to HTTP
22. Response Cache Store        → Cache response in KeyDB
23. Error Handling              → Retry logic, fallback responses
24. Metrics Collection          → Record latency, status, throughput
25. Logging (end)               → Record response, duration, errors
                               ↓
┌─────────────────────────────────────────────────────────────────┐
│ OUTGOING RESPONSE                                                │
└─────────────────────────────────────────────────────────────────┘
```

**Key Considerations:**
- **Early rejection**: ACL, load shedding, and rate limiting happen before expensive operations
- **Cache optimization**: Cache check occurs after auth/authz to ensure user-specific caching
- **Security layers**: TLS → ACL → Auth → Authz forms defense in depth
- **Circuit breaker placement**: After validation but before proxying prevents wasted backend calls
- **mTLS for backends**: Ensures secure service-to-service communication

#### Docker Compose Changes

```yaml
services:
  gateway:                        # NEW: API Gateway
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile.gateway
    ports:
      - "8090:8090"
    networks:
      - web-analyzer-network
    depends_on:
      - api
      - keydb
      - vault
    environment:
      - GATEWAY_ENV=development
      - GATEWAY_BACKEND_GRPC_ADDR=api:9090
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.gateway.rule=Host(`api.web-analyzer.dev`)"
      - "traefik.http.services.gateway.loadbalancer.server.port=8090"

  api:                            # MODIFIED: Now gRPC service
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile
    ports:
      - "9090:9090"               # gRPC port (not exposed via Traefik)
    # Gateway talks to this service directly
```

### Technology Stack

```yaml
Gateway Dependencies:
  - github.com/go-chi/chi/v5 or github.com/gin-gonic/gin  # HTTP routing
  - google.golang.org/grpc                                 # gRPC client
  - golang.org/x/time/rate                                 # Rate limiting
  - github.com/sony/gobreaker                              # Circuit breaker
  - github.com/redis/go-redis/v9                           # KeyDB for caching/rate limiting
  - go.opentelemetry.io/otel                               # Observability

Backend Dependencies:
  - google.golang.org/grpc                                 # gRPC server
  - google.golang.org/protobuf                             # Protobuf support
  - [Keep existing dependencies]

Code Generation:
  - protoc                                                 # Protocol buffer compiler
  - protoc-gen-go                                          # Go protobuf plugin
  - protoc-gen-go-grpc                                     # Go gRPC plugin
```

### Implementation Phases (6 Weeks)

**Week 1: Foundation**
- Create proto definitions in `docs/proto/`
- Generate Go code with protoc
- Create `cmd/gateway/` structure
- Basic HTTP→gRPC routing (no features yet)

**Week 2: Backend Conversion**
- Implement gRPC service handlers
- Replace HTTP with gRPC in `internal/runtime/deps.go`
- Implement gRPC streaming for events
- Remove all HTTP middleware from backend

**Week 3: Gateway Security & Access Control**
- PASETO authentication middleware
- Request signing verification (RFC 9421) for request integrity and non-repudiation
- Authorization (RBAC) implementation with fine-grained permissions
- Parameter validation (OpenAPI schema)
- ACL (allow/deny lists - IP/CIDR/user-agent)
- Security headers (CORS, CSP, HSTS, X-Frame-Options, etc.)
- TLS 1.3 configuration for client connections

**Week 4: Gateway Resilience & Traffic Management**
- Rate limiting with KeyDB backend (per client/endpoint/global)
- Load shedding implementation ⭐ (adaptive probabilistic)
- Circuit breaker pattern (per-service with recovery)
- Error handling & retry logic (exponential backoff)
- mTLS for backend service communication

**Week 5: Gateway Performance & Routing**
- Service discovery implementation (health-aware registry)
- Dynamic request routing (pattern-based with service resolution)
- Load balancing strategies (round-robin, weighted, least-connections, consistent hashing)
- Response caching with KeyDB (Cache-Control/ETag support)
- API composition (scatter-gather pattern)
- Protocol conversion optimization (HTTP→gRPC, SSE→gRPC streaming)
- Logging, metrics, tracing integration (OpenTelemetry)

**Week 6: Integration & Testing**
- Docker Compose updates
- End-to-end integration tests
- Load testing (verify load shedding behavior)
- Performance benchmarking
- Documentation updates

### Migration Strategy

**Phase 1: Parallel Deployment (Week 1-2)**
- Deploy gateway alongside existing HTTP API
- Route 10% of traffic through gateway for testing
- Monitor metrics and error rates

**Phase 2: Backend Conversion (Week 3-4)**
- Convert HTTP API to gRPC
- Route 50% traffic through gateway
- Validate SSE→gRPC streaming works correctly

**Phase 3: Full Migration (Week 5)**
- Route 100% traffic through gateway
- Decommission direct HTTP API access
- Update Traefik to point to gateway only

**Phase 4: Cleanup (Week 6)**
- Remove HTTP middleware code from backend
- Update documentation
- Archive old HTTP implementation

### Key Metrics

```yaml
Gateway Metrics:
  - gateway_requests_total{method, path, status}
  - gateway_request_duration_seconds{method, path}
  - gateway_load_shedding_drops_total
  - gateway_circuit_breaker_state{service}
  - gateway_cache_hits_total / gateway_cache_misses_total
  - gateway_grpc_requests_total{service, method, status}
  - gateway_grpc_request_duration_seconds{service, method}

Load Shedding Metrics:
  - load_shedding_load_factor (gauge)
  - load_shedding_queue_depth (gauge)
  - load_shedding_requests_dropped_total (counter)
  - load_shedding_latency_p99 (histogram)
```

### Testing Strategy

```go
// Gateway tests
func TestHTTPToGRPCConversion(t *testing.T) {
    // Test HTTP request → gRPC call → HTTP response
}

func TestSSEStreaming(t *testing.T) {
    // Test gRPC stream → SSE event conversion
}

func TestLoadShedding(t *testing.T) {
    // Simulate overload, verify requests dropped with 503
}

func TestCircuitBreaker(t *testing.T) {
    // Simulate backend failures, verify circuit opens
}

func TestRateLimiting(t *testing.T) {
    // Test per-client and per-endpoint rate limits
}

// Load tests
func BenchmarkGatewayThroughput(b *testing.B) {
    // Measure requests/second under load
}

func TestGatewayUnderLoad(t *testing.T) {
    // Verify load shedding triggers at correct thresholds
}
```

### SSE to gRPC Streaming Conversion

The gateway handles SSE→gRPC streaming conversion transparently:

```go
// Gateway receives HTTP SSE request
// GET /v1/analysis/{id}/events
func (g *Gateway) handleSSE(w http.ResponseWriter, r *http.Request) {
    analysisID := chi.URLParam(r, "analysisId")

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.WriteHeader(http.StatusOK)

    flusher := w.(http.Flusher)

    // Call gRPC streaming endpoint
    stream, err := g.grpcClient.StreamAnalysisEvents(r.Context(), &pb.StreamEventsRequest{
        AnalysisId: analysisID,
    })
    if err != nil {
        fmt.Fprintf(w, "event: error\ndata: {\"error\": \"failed to start stream\"}\n\n")
        flusher.Flush()
        return
    }

    // Stream gRPC responses as SSE events
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
            flusher.Flush()
            break
        }

        // Convert protobuf to JSON
        jsonData, _ := protojson.Marshal(event.Payload)

        // Write as SSE format
        fmt.Fprintf(w, "event: %s\n", event.Type)
        fmt.Fprintf(w, "data: %s\n\n", jsonData)
        flusher.Flush()
    }
}
```

### Benefits

**Architectural:**
- Centralized cross-cutting concerns
- Clean separation: gateway (infra) vs backend (business logic)
- Protocol flexibility (HTTP for clients, gRPC for services)
- Microservices-ready architecture

**Operational:**
- Independent scaling (gateway vs backend)
- Canary deployments via traffic routing
- A/B testing support
- Simplified backend services

**Performance:**
- Binary gRPC protocol (faster than JSON)
- HTTP/2 multiplexing
- Response caching reduces backend load
- Load shedding prevents overload

**Security:**
- Single point for security policies
- Consistent auth/authz enforcement
- Request validation before reaching backend
- DDoS protection via rate limiting

**Resilience:**
- Circuit breaker prevents cascading failures
- Load shedding maintains availability under stress
- Retry logic for transient errors
- Graceful degradation

### Implementation Checklist

- [ ] Create protobuf definitions in `docs/proto/analysis/v1/`
- [ ] Generate Go code from proto files
- [ ] Create `cmd/gateway/main.go` entry point
- [ ] Implement gateway router and service registry
- [ ] Implement HTTP→gRPC protocol converter
- [ ] Implement SSE→gRPC streaming converter
- [ ] Migrate PASETO auth middleware to gateway
- [ ] Implement request signing verification (RFC 9421) middleware
- [ ] Implement RBAC authorization with fine-grained permissions
- [ ] Migrate OpenAPI validation middleware to gateway
- [ ] Implement rate limiting with KeyDB backend
- [ ] Implement load shedding middleware ⭐
- [ ] Implement circuit breaker per-service
- [ ] Implement response caching with KeyDB
- [ ] Implement load balancing strategies
- [ ] Implement ACL (IP/CIDR filtering)
- [ ] Implement security headers middleware
- [ ] Implement error handling and retry logic
- [ ] Implement logging and monitoring
- [ ] Integrate OpenTelemetry distributed tracing
- [ ] Implement API composition for complex queries
- [ ] Convert HTTP API service to gRPC
- [ ] Implement gRPC service handlers
- [ ] Implement gRPC server streaming for events
- [ ] Remove HTTP middleware from backend
- [ ] Update Docker Compose with gateway service
- [ ] Create gateway configuration management
- [ ] Write comprehensive unit tests for gateway
- [ ] Write integration tests for HTTP→gRPC conversion
- [ ] Write integration tests for SSE streaming
- [ ] Perform load testing and verify load shedding
- [ ] Update architecture documentation
- [ ] Update deployment documentation
- [ ] Create gateway operational runbook

---

## 17. C4 Architecture Documentation 🔵 **LOW**

### Current State
- Good architectural documentation in `docs/architecture.md`
- No visual architecture diagrams
- No standardized architecture modeling
- Difficult to onboard new developers quickly
- No automated diagram generation from code

### C4 Model Overview

The [C4 model](https://c4model.com/) provides a hierarchical way to visualize software architecture at different levels of abstraction:

1. **Level 1 - System Context**: Shows how the system fits into the wider world
2. **Level 2 - Container**: Shows the high-level technical building blocks
3. **Level 3 - Component**: Shows how containers are made up of components
4. **Level 4 - Code**: Shows how components are implemented (optional)

### Required Improvements

#### Project Structure

```
docs/architecture/c4/
├── main.go                    # Go program to generate C4 diagrams
├── go.mod                     # Go module for go-structurizr
├── go.sum                     # Go module checksums
└── diagrams/                  # Generated PlantUML and rendered images
    ├── 01-system-context.puml
    ├── 01-system-context.png
    ├── 02-containers.puml
    ├── 02-containers.png
    ├── 03-component-api.puml
    ├── 03-component-api.png
    ├── 04-component-subscriber.puml
    └── 04-component-subscriber.png
```

#### Go-Structurizr Implementation

Using [go-structurizr](https://github.com/krzysztofreczek/go-structurizr) for programmatic C4 diagram generation:

**Benefits:**
- Type-safe diagram definitions in Go
- Version controlled alongside code
- Automated generation in CI/CD pipeline
- Exports to PlantUML format
- Easy to update and maintain

**Example Structure:**

```go
package main

import (
    "github.com/krzysztofreczek/go-structurizr/pkg/scraper"
    "github.com/krzysztofreczek/go-structurizr/pkg/view"
    . "github.com/krzysztofreczek/go-structurizr/pkg/model"
)

func defineWorkspace() *Workspace {
    w := NewWorkspace("Web Analyzer Service")

    // Define people
    webUser := w.Model.AddPerson("Web User", "Analyzes web pages")

    // Define systems
    webAnalyzer := w.Model.AddSoftwareSystem("Web Analyzer",
        "Event-driven web page analysis service")

    // Define containers
    httpAPI := webAnalyzer.AddContainer("gRPC API Service",
        "RESTful API with CQRS", "Go")
    publisher := webAnalyzer.AddContainer("Publisher Service",
        "Outbox pattern implementation", "Go")
    subscriber := webAnalyzer.AddContainer("Subscriber Service",
        "Async analysis processing", "Go")

    // Define relationships
    webUser.Uses(httpAPI, "Submits analysis requests", "HTTPS/REST")
    httpAPI.Uses(publisher, "Publishes events", "Database/Outbox")
    publisher.Uses(subscriber, "Delivers events", "RabbitMQ")

    return w
}
```

#### Level 1: System Context Diagram

Shows the Web Analyzer system and its interactions with external actors:

**Elements:**
- **People:**
  - Web User (submits URLs for analysis)
  - API Client (programmatic access)
- **Systems:**
  - Web Analyzer System (main system)
  - Target Website (external system being analyzed)

**Relationships:**
- Web User → Web Analyzer (HTTPS/REST)
- API Client → Web Analyzer (HTTPS/REST)
- Web Analyzer → Target Website (HTTPS fetch)

#### Level 2: Container Diagram

Shows the high-level containers within the Web Analyzer system:

**Frontend Containers:**
- Web Frontend (Vanilla JavaScript SPA)
- Traefik Proxy (reverse proxy with SSL)

**Backend Containers:**
- API Gateway (HTTP→gRPC conversion, 19 features)
- gRPC API Service (business logic, CQRS handlers)
- Publisher Service (outbox pattern)
- Subscriber Service (async processing)
- Janitor Service (data lifecycle management)

**Infrastructure Containers:**
- PostgreSQL (transactional data, outbox)
- RabbitMQ (message queue)
- KeyDB (cache, rate limiting)
- HashiCorp Vault (secrets management)

**Key Flows:**
```
User → Traefik → API Gateway → gRPC API → PostgreSQL
                                    ↓
                               (Outbox Table)
                                    ↓
                    Publisher → RabbitMQ → Subscriber → PostgreSQL
```

#### Level 3: Component Diagrams

**API Service Components:**
- HTTP Handlers (Chi router, OpenAPI-generated)
- CQRS Commands (CreateAnalysis, UpdateStatus)
- CQRS Queries (GetAnalysis, ListAnalyses)
- Middleware Chain (auth, validation, tracing)
- Repository Layer (PostgreSQL persistence)
- Outbox Publisher (transactional event publishing)

**Subscriber Service Components:**
- Queue Consumer (RabbitMQ AMQP client)
- Event Handler (workflow orchestration)
- HTML Analyzer (goquery-based parsing)
- Link Checker (link validation)
- Web Fetcher (HTTP client)
- Analysis Repository (result persistence)

**Gateway Components (from Improvement #13):**
- Protocol Converter (HTTP→gRPC)
- Authentication Middleware (PASETO)
- Rate Limiter (token bucket + KeyDB)
- Load Shedder (adaptive probabilistic)
- Circuit Breaker (per-service)
- Response Cache (KeyDB-backed)
- Security Headers (CORS, CSP, etc.)

#### PlantUML Rendering

The generated PlantUML files can be rendered using:

**Option 1: PlantUML CLI**
```bash
# Install PlantUML
brew install plantuml

# Render diagrams
plantuml -tpng docs/architecture/c4/diagrams/*.puml
plantuml -tsvg docs/architecture/c4/diagrams/*.puml
```

**Option 2: Docker**
```bash
docker run -v $(pwd)/docs/architecture/c4/diagrams:/data \
    plantuml/plantuml:latest \
    -tpng /data/*.puml
```

**Option 3: Online Renderer**
- [PlantUML Online Server](http://www.plantuml.com/plantuml)
- GitHub automatically renders `.puml` files in README

#### Makefile Integration

Add targets for automated diagram generation:

```makefile
# Generate C4 architecture diagrams
.PHONY: c4-generate
c4-generate:
	@echo "Generating C4 diagrams..."
	cd docs/architecture/c4 && go run main.go

# Render PlantUML to PNG
.PHONY: c4-render
c4-render: c4-generate
	@echo "Rendering diagrams to PNG..."
	docker run --rm -v $(PWD)/docs/architecture/c4/diagrams:/data \
		plantuml/plantuml:latest -tpng /data/*.puml

# Render PlantUML to SVG (better for web/docs)
.PHONY: c4-svg
c4-svg: c4-generate
	@echo "Rendering diagrams to SVG..."
	docker run --rm -v $(PWD)/docs/architecture/c4/diagrams:/data \
		plantuml/plantuml:latest -tsvg /data/*.puml

# Full C4 pipeline
.PHONY: c4-docs
c4-docs: c4-generate c4-render c4-svg
	@echo "✅ C4 documentation generated!"
```

#### CI/CD Integration

Add to `.github/workflows/docs.yml`:

```yaml
name: Documentation

on:
  push:
    paths:
      - 'docs/architecture/c4/**'
      - 'internal/**'
      - 'cmd/**'

jobs:
  c4-diagrams:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'

      - name: Generate C4 Diagrams
        run: make c4-docs

      - name: Commit Rendered Diagrams
        run: |
          git config user.name "GitHub Actions"
          git config user.email "actions@github.com"
          git add docs/architecture/c4/diagrams/*.png
          git add docs/architecture/c4/diagrams/*.svg
          git diff --quiet && git diff --staged --quiet || \
            git commit -m "docs: :art: update C4 architecture diagrams"
          git push
```

#### Documentation Updates

**Update `docs/architecture.md`:**

```markdown
# System Architecture

## Architecture Diagrams

Visual architecture documentation using the C4 model:

### System Context
![System Context](c4/diagrams/01-system-context.png)

Shows how the Web Analyzer fits into the wider ecosystem.

### Containers
![Containers](c4/diagrams/02-containers.png)

High-level technical building blocks and their interactions.

### Components - API Service
![API Components](c4/diagrams/03-component-api.png)

Internal structure of the gRPC API Service.

### Components - Subscriber Service
![Subscriber Components](c4/diagrams/04-component-subscriber.png)

Internal structure of the Subscriber Service.

## Regenerating Diagrams

To regenerate architecture diagrams:

\`\`\`bash
make c4-docs
\`\`\`

This will:
1. Generate PlantUML files from Go definitions
2. Render PNG and SVG images
3. Update diagrams in this directory
```

**Create `docs/architecture/c4/README.md`:**

```markdown
# C4 Architecture Diagrams

This directory contains C4 model diagrams for the Web Analyzer service.

## Quick Start

Generate all diagrams:

\`\`\`bash
make c4-docs
\`\`\`

## Structure

- `main.go` - Go program defining C4 workspace
- `diagrams/` - Generated PlantUML files and rendered images

## Levels

1. **System Context** - External view of the system
2. **Containers** - High-level technical building blocks
3. **Components** - Internal structure of containers
4. **Code** - Class/code level (not implemented)

## Updating Diagrams

1. Edit `main.go` to modify architecture
2. Run `make c4-generate` to generate PlantUML
3. Run `make c4-render` to create PNG/SVG images
4. Commit changes to version control

## Resources

- [C4 Model](https://c4model.com/)
- [go-structurizr](https://github.com/krzysztofreczek/go-structurizr)
- [PlantUML](https://plantuml.com/)
```

### Key Elements in C4 Model

**System Context Diagram includes:**
- Web User (person)
- API Client (person)
- Web Analyzer System (software system)
- Target Website (external system)

**Container Diagram includes:**
- All containers from improvement plan sections
- Infrastructure components (PostgreSQL, RabbitMQ, KeyDB, Vault)
- Relationships showing data flow and protocols

**Component Diagrams include:**
- Detailed breakdown of gRPC API Service
- Detailed breakdown of Subscriber Service
- Optional: API Gateway components (from #13)
- Optional: Publisher Service components

### Tagging Strategy

Use tags for visual styling in PlantUML:

```go
// People tags
webUser.AddTags("User")
apiClient.AddTags("Developer")

// Container tags
frontend.AddTags("UI")
gateway.AddTags("Gateway")
httpAPI.AddTags("API")
postgres.AddTags("Database")
rabbitMQ.AddTags("MessageQueue")

// Component tags
handlers.AddTags("Handler")
repository.AddTags("Repository")
```

This enables custom styling in PlantUML output:

```plantuml
!define USER_COLOR #08427B
!define API_COLOR #1168BD
!define DATABASE_COLOR #999999
!define QUEUE_COLOR #F39C12

skinparam {
    UserColor USER_COLOR
    APIColor API_COLOR
    DatabaseColor DATABASE_COLOR
}
```

### Benefits

**For Development:**
- Clear visual representation of architecture
- Easy onboarding for new team members
- Better communication with stakeholders
- Version-controlled diagrams (text-based)

**For Operations:**
- Understanding service dependencies
- Troubleshooting communication flows
- Planning deployments and scaling
- Disaster recovery planning

**For Documentation:**
- Living documentation (stays in sync)
- Automated generation reduces manual work
- Consistent with industry standards
- Multiple export formats (PNG, SVG, PlantUML)

### Implementation Checklist

- [ ] Create `docs/architecture/c4/` directory structure
- [ ] Implement C4 workspace definition in `main.go`
- [ ] Define System Context diagram (Level 1)
- [ ] Define Container diagram (Level 2)
- [ ] Define Component diagrams for API Service (Level 3)
- [ ] Define Component diagrams for Subscriber Service (Level 3)
- [ ] Add gateway components from improvement #13
- [ ] Add janitor service from improvement #12
- [ ] Configure PlantUML export with go-structurizr
- [ ] Add Makefile targets for generation and rendering
- [ ] Set up Docker-based PlantUML rendering
- [ ] Test diagram generation locally
- [ ] Create `docs/architecture/c4/README.md`
- [ ] Update `docs/architecture.md` with diagram links
- [ ] Add CI/CD workflow for automated diagram updates
- [ ] Generate initial set of diagrams
- [ ] Commit rendered images to repository
- [ ] Document diagram regeneration process

### Dependencies

```yaml
Go Packages:
  - github.com/krzysztofreczek/go-structurizr v1.1.0  # C4 model in Go
  - github.com/google/uuid                            # UUID support (indirect)

Docker Images:
  - plantuml/plantuml:latest                          # PlantUML renderer

Optional:
  - PlantUML CLI (via homebrew: brew install plantuml)
  - Java Runtime (for local PlantUML rendering)
```

### Example Output

**Generated PlantUML (excerpt):**

```plantuml
@startuml
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Container.puml

Person(web_user, "Web User", "End user who wants to analyze web pages")
Person(api_client, "API Client", "Developer/system using the REST API")

System_Boundary(web_analyzer, "Web Analyzer") {
    Container(gateway, "API Gateway", "Go", "HTTP→gRPC conversion, auth, rate limiting")
    Container(api, "gRPC API Service", "Go", "Business logic, CQRS handlers")
    Container(publisher, "Publisher Service", "Go", "Outbox pattern implementation")
    Container(subscriber, "Subscriber Service", "Go", "Async analysis processing")
    ContainerDb(postgres, "PostgreSQL", "PostgreSQL", "Transactional data")
    ContainerQueue(rabbitmq, "RabbitMQ", "RabbitMQ", "Message queue")
}

System_Ext(target_site, "Target Website", "External website being analyzed")

Rel(web_user, gateway, "Submits URLs", "HTTPS/REST")
Rel(gateway, api, "Forwards requests", "gRPC")
Rel(api, postgres, "Stores data", "SQL")
Rel(publisher, rabbitmq, "Publishes events", "AMQP")
Rel(subscriber, target_site, "Fetches pages", "HTTPS")

@enduml
```

### Maintenance

**When to Update:**
- New services added (e.g., API Gateway, Janitor)
- Container relationships change
- New components added to services
- Infrastructure changes (new databases, queues)
- Major refactoring

**Update Process:**
1. Modify `docs/architecture/c4/main.go`
2. Run `make c4-docs`
3. Review generated diagrams
4. Commit changes to Git
5. CI/CD automatically renders and updates diagrams

---

## Implementation Timeline

### Week 1: Foundation (Critical)
- [ ] Complete metrics implementation
- [ ] Set up basic CI/CD pipeline
- [ ] Configure golangci-lint

### Week 2: Quality & Security (High Priority)
- [ ] Complete integration tests
- [ ] Add pre-commit hooks
- [ ] Implement RFC 9421 HTTP message signatures
- [ ] Set up content digest (RFC 9530)
- [ ] Implement CSRF protection middleware
- [ ] Add CSRF token endpoint and frontend integration

### Week 3: Security & Testing (High Priority)
- [ ] Complete HTTP signature middleware
- [ ] Implement key rotation mechanism
- [ ] Add signature verification tests
- [ ] Implement code coverage targets
- [ ] Add domain model fields for archival/expiration
- [ ] Implement repository archival methods

### Week 4: Data Lifecycle & Janitor (High Priority)
- [ ] Create janitor CLI tool with Ory Hydra-inspired design
- [ ] Implement batch deletion with safety controls
- [ ] Add API endpoints for manual archival
- [ ] Create deployment manifests for janitor
- [ ] Test janitor with dry-run mode

### Week 5: Architecture (Medium Priority)
- [ ] Integrate pipeline pattern
- [ ] Create Kubernetes manifests
- [ ] Implement circuit breaker
- [ ] Enhance rate limiting

### Week 6: Deployment & Documentation (Medium/Low Priority)
- [ ] Complete Helm charts
- [ ] Add retry mechanisms
- [ ] Complete documentation
- [ ] Create operational runbooks
- [ ] Document janitor usage and safety procedures
- [ ] Create C4 architecture diagrams with go-structurizr
- [ ] Generate and render PlantUML diagrams
- [ ] Add Makefile targets for C4 diagram generation
- [ ] Update architecture.md with C4 diagram links

### Weeks 7-12: API Gateway Implementation (High Priority)
- **Week 7: Gateway Foundation**
  - [ ] Create protobuf definitions in `docs/proto/`
  - [ ] Generate Go code from proto files
  - [ ] Create `cmd/gateway/` structure and basic routing
  - [ ] Set up gateway configuration management
- **Week 8: Backend gRPC Conversion**
  - [ ] Implement gRPC service handlers
  - [ ] Replace HTTP with gRPC in backend
  - [ ] Implement gRPC streaming for events
  - [ ] Remove HTTP middleware from backend
- **Week 9: Gateway Security**
  - [ ] Migrate PASETO authentication to gateway
  - [ ] Migrate OpenAPI validation to gateway
  - [ ] Implement ACL (IP/CIDR filtering)
  - [ ] Implement security headers middleware
- **Week 10: Gateway Resilience**
  - [ ] Implement rate limiting with KeyDB
  - [ ] Implement load shedding ⭐
  - [ ] Implement circuit breaker
  - [ ] Implement error handling and retry logic
- **Week 11: Gateway Performance**
  - [ ] Implement response caching with KeyDB
  - [ ] Implement load balancing strategies
  - [ ] Implement API composition
  - [ ] Integrate logging, metrics, tracing
- **Week 12: Integration & Testing**
  - [ ] Update Docker Compose configuration
  - [ ] Write comprehensive gateway tests
  - [ ] Perform load testing and verify load shedding
  - [ ] Update architecture documentation
  - [ ] Create gateway operational runbook

---

## Success Metrics

| Metric | Target | Measure |
|--------|--------|---------|
| Code Coverage | >80% | Via CI/CD |
| Build Success Rate | >95% | GitHub Actions |
| Mean Time to Recovery | <15min | Incident tracking |
| API Response Time | <200ms p99 | Prometheus metrics |
| Error Rate | <0.1% | Application metrics |
| Deployment Frequency | Daily | CI/CD pipeline |

---

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking changes | High | Feature flags, gradual rollout |
| Performance degradation | Medium | Load testing, monitoring |
| Security vulnerabilities | High | Automated scanning, audits |
| Integration failures | Medium | Comprehensive testing |

---

## Next Steps

1. **Review and Prioritize**: Team review of this plan
2. **Resource Allocation**: Assign team members to tasks
3. **Sprint Planning**: Break down into sprint-sized work items
4. **Implementation Kickoff**: Start with critical items
5. **Progress Tracking**: Weekly status updates

---

## Notes

- All improvements align with the documented architecture decisions
- Priority based on production readiness requirements
- Timeline assumes 2-3 developers working in parallel
- Consider creating feature branches for major improvements
- CSRF protection added as essential security measure for form submissions
- Security improvements include both RFC 9421 signatures and CSRF protection
- Janitor service design inspired by Ory Hydra's proven pattern for safe data cleanup
- Data lifecycle management leverages existing database schema (no migrations needed)
- API Gateway implementation includes full HTTP→gRPC protocol conversion
- Proto definitions stored in `docs/proto/` alongside OpenAPI specs for consistency
- Load shedding feature uses adaptive probabilistic algorithm for graceful degradation
- Gateway enables independent scaling and simplified backend services
- 6-week gateway implementation timeline (Weeks 7-12) can run in parallel with other improvements
- C4 architecture diagrams use go-structurizr for programmatic, version-controlled diagram generation
- PlantUML format enables automated rendering in CI/CD and GitHub integration
- C4 documentation provides visual onboarding aid for new team members

---

## 18. Priority-Based Worker Allocation 🟡 **HIGH**

### Current State
- Priority field exists in `outbox_events` table (`low`, `normal`, `high`, `urgent`)
- Database queries ordered by `priority DESC, created_at ASC`
- Priority-based max retries configuration implemented
- Single RabbitMQ consumer with `PrefetchCount: 10`
- No priority-based routing in RabbitMQ
- No dynamic worker allocation based on priority

### Problem Statement
Currently, priority is only used in database queries to process higher-priority items first. However, the subscriber service has only one consumer with limited concurrency (prefetch count = 10), which doesn't scale processing capacity based on priority levels. High-priority requests must wait in the same queue as low-priority requests.

### Recommended Approach: RabbitMQ Priority Queues + Goroutine Worker Pools

**Why this approach?**
- ✅ Minimal code changes (no schema migrations)
- ✅ Uses RabbitMQ's native priority queue feature (available since v3.5)
- ✅ Application-level worker pools provide fine-grained control
- ✅ Can adjust worker ratios without redeployment via environment variables
- ✅ Backward compatible with existing code
- ✅ Better resource utilization

### Architecture Design

```
┌─────────────────────────────────────────────────────────────┐
│                    Publisher Service                        │
│  Maps domain priority → AMQP priority                       │
│  urgent=10, high=7, normal=5, low=3                         │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│              RabbitMQ Priority Queue (x-max-priority=10)    │
│  Native priority ordering: Higher priority msgs first        │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                   Subscriber Service                        │
│  ┌─────────────────────────────────────────────┐           │
│  │         Priority Worker Pool Manager         │           │
│  ├─────────────────────────────────────────────┤           │
│  │ Urgent Worker Pool   → 10 goroutines (50%)  │           │
│  │ High Worker Pool     → 6 goroutines  (30%)  │           │
│  │ Normal Worker Pool   → 3 goroutines  (15%)  │           │
│  │ Low Worker Pool      → 1 goroutine   (5%)   │           │
│  └─────────────────────────────────────────────┘           │
│                 Total: 20 concurrent workers                │
└─────────────────────────────────────────────────────────────┘
```

### Required Improvements

#### Phase 1: Enable RabbitMQ Priority Queue (Week 1)

**1. Queue Declaration with Priority Support**

```go
// pkg/queue/queue.go - Update DeclareQueue method
func (q *RabbitMQQueue) DeclareQueue(name string, durable, autoDelete bool, args amqp.Table) (amqp.Queue, error) {
    if !q.IsConnected() {
        return amqp.Queue{}, fmt.Errorf("not connected to RabbitMQ")
    }

    // Add priority support if args provided
    if args == nil {
        args = amqp.Table{}
    }

    return q.channel.QueueDeclare(name, durable, autoDelete, false, false, args)
}
```

**2. Initialize Queue with Priority**

```go
// internal/infrastructure/queue.go - Update queue initialization
func InitializeQueue(queue Queue, cfg config.QueueConfig) error {
    // Declare exchange
    if err := queue.DeclareExchange(
        cfg.ExchangeName,
        "topic",
        cfg.Durable,
        cfg.AutoDelete,
    ); err != nil {
        return fmt.Errorf("failed to declare exchange: %w", err)
    }

    // Declare queue with priority support
    args := amqp.Table{
        "x-max-priority": cfg.MaxPriority, // NEW: Enable priority queue
    }

    _, err := queue.DeclareQueue(
        cfg.QueueName,
        cfg.Durable,
        cfg.AutoDelete,
        args, // Pass priority configuration
    )
    if err != nil {
        return fmt.Errorf("failed to declare queue: %w", err)
    }

    // Bind queue
    return queue.BindQueue(cfg.QueueName, cfg.RoutingKey, cfg.ExchangeName)
}
```

**3. Message Publishing with Priority**

```go
// pkg/queue/queue.go - Update PublishWithOptions
func (q *RabbitMQQueue) PublishWithOptions(ctx context.Context, exchange, routingKey string, payload any, opts ...publisherOption) error {
    if !q.IsConnected() {
        return fmt.Errorf("not connected to RabbitMQ")
    }

    options := defaultPublisherOptions()
    for _, opt := range opts {
        opt(&options)
    }

    msg := &Message{Body: payload}
    body, err := msg.marshal()
    if err != nil {
        return fmt.Errorf("failed to marshal message: %w", err)
    }

    ctx, cancel := context.WithTimeout(ctx, options.timeout)
    defer cancel()

    publishing := amqp.Publishing{
        ContentType:  "application/json",
        Body:         body,
        DeliveryMode: amqp.Persistent,
        Timestamp:    time.Now(),
        Priority:     options.priority, // NEW: Set message priority
    }

    return q.channel.Publish(exchange, routingKey, false, false, publishing)
}
```

**4. Priority Mapping**

```go
// pkg/queue/options.go - Add priority option
type publisherOptions struct {
    timeout  time.Duration
    priority uint8  // NEW: 0-10 priority
}

func WithPriority(priority uint8) publisherOption {
    return func(o *publisherOptions) {
        if priority > 10 {
            priority = 10
        }
        o.priority = priority
    }
}

// internal/adapters/repos/outbox_publisher.go - Map domain priority to AMQP priority
func (p *OutboxPublisher) mapPriorityToAMQP(domainPriority domain.Priority) uint8 {
    switch domainPriority {
    case domain.PriorityUrgent:
        return 10
    case domain.PriorityHigh:
        return 7
    case domain.PriorityNormal:
        return 5
    case domain.PriorityLow:
        return 3
    default:
        return 5
    }
}

// Use in publishing
func (p *OutboxPublisher) PublishEvent(ctx context.Context, event *domain.OutboxEvent) error {
    priority := p.mapPriorityToAMQP(event.Priority)

    return p.queue.PublishWithOptions(
        ctx,
        p.config.ExchangeName,
        p.config.RoutingKey,
        event.Payload,
        queue.WithPriority(priority),
    )
}
```

**5. Configuration Updates**

```go
// internal/config/settings.go - Add MaxPriority to QueueConfig
type QueueConfig struct {
    Host           string        `envconfig:"RABBITMQ_HOST" default:"rabbitmq"`
    Port           int           `envconfig:"RABBITMQ_PORT" default:"5672"`
    Username       string        `envconfig:"RABBITMQ_USERNAME" default:"admin"`
    Password       string        `envconfig:"RABBITMQ_PASSWORD" default:"bottom.Secret"`
    VirtualHost    string        `envconfig:"RABBITMQ_VIRTUAL_HOST" default:"/"`
    ExchangeName   string        `envconfig:"RABBITMQ_EXCHANGE_NAME" default:"web-analyzer"`
    RoutingKey     string        `envconfig:"RABBITMQ_ROUTING_KEY" default:"analysis.*"`
    QueueName      string        `envconfig:"RABBITMQ_NAME" default:"analysis_queue"`
    ConnectTimeout time.Duration `envconfig:"RABBITMQ_CONNECT_TIMEOUT" default:"10s"`
    Heartbeat      time.Duration `envconfig:"RABBITMQ_HEARTBEAT" default:"10s"`
    PrefetchCount  int           `envconfig:"RABBITMQ_PREFETCH_COUNT" default:"10"`
    MaxPriority    int           `envconfig:"RABBITMQ_MAX_PRIORITY" default:"10"` // NEW
    Durable        bool          `envconfig:"RABBITMQ_DURABLE" default:"true"`
    AutoDelete     bool          `envconfig:"RABBITMQ_AUTO_DELETE" default:"false"`
}
```

#### Phase 2: Implement Priority-Based Worker Pools (Week 2)

**1. Worker Pool Structure**

```go
// internal/adapters/queue/worker_pool.go - NEW FILE
package queue

import (
    "context"
    "sync"

    "github.com/architeacher/svc-web-analyzer/internal/infrastructure"
    "github.com/architeacher/svc-web-analyzer/pkg/queue"
)

type PriorityWorkerPool struct {
    urgentPool  *WorkerPool
    highPool    *WorkerPool
    normalPool  *WorkerPool
    lowPool     *WorkerPool
    logger      infrastructure.Logger
    wg          sync.WaitGroup
}

type WorkerPool struct {
    workerCount int
    msgChan     chan queue.Message
    handler     queue.MessageHandler
    ctx         context.Context
    ctrlChan    chan *queue.MsgController
}

func NewPriorityWorkerPool(
    urgentWorkers, highWorkers, normalWorkers, lowWorkers int,
    handler queue.MessageHandler,
    logger infrastructure.Logger,
) *PriorityWorkerPool {
    return &PriorityWorkerPool{
        urgentPool:  newWorkerPool(urgentWorkers, handler),
        highPool:    newWorkerPool(highWorkers, handler),
        normalPool:  newWorkerPool(normalWorkers, handler),
        lowPool:     newWorkerPool(lowWorkers, handler),
        logger:      logger,
    }
}

func newWorkerPool(count int, handler queue.MessageHandler) *WorkerPool {
    return &WorkerPool{
        workerCount: count,
        msgChan:     make(chan queue.Message, count*2), // Buffer 2x workers
        handler:     handler,
    }
}

func (p *PriorityWorkerPool) Start(ctx context.Context) {
    p.urgentPool.start(ctx, &p.wg, "urgent", p.logger)
    p.highPool.start(ctx, &p.wg, "high", p.logger)
    p.normalPool.start(ctx, &p.wg, "normal", p.logger)
    p.lowPool.start(ctx, &p.wg, "low", p.logger)
}

func (wp *WorkerPool) start(ctx context.Context, wg *sync.WaitGroup, priority string, logger infrastructure.Logger) {
    wp.ctx = ctx

    for i := 0; i < wp.workerCount; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()

            logger.Info().
                Str("priority", priority).
                Int("worker_id", workerID).
                Msg("worker started")

            for {
                select {
                case <-ctx.Done():
                    logger.Info().
                        Str("priority", priority).
                        Int("worker_id", workerID).
                        Msg("worker shutting down")

                    return
                case msg := <-wp.msgChan:
                    ctrl := &queue.MsgController{} // Create controller

                    if err := wp.handler(ctx, msg, ctrl); err != nil {
                        logger.Error().Err(err).
                            Str("priority", priority).
                            Int("worker_id", workerID).
                            Msg("message processing failed")
                    }
                }
            }
        }(i)
    }
}

func (p *PriorityWorkerPool) RouteMessage(msg queue.Message, priority string) error {
    var pool *WorkerPool

    switch priority {
    case "urgent":
        pool = p.urgentPool
    case "high":
        pool = p.highPool
    case "normal":
        pool = p.normalPool
    case "low":
        pool = p.lowPool
    default:
        pool = p.normalPool
    }

    select {
    case pool.msgChan <- msg:
        return nil
    case <-pool.ctx.Done():
        return pool.ctx.Err()
    }
}

func (p *PriorityWorkerPool) Shutdown() {
    close(p.urgentPool.msgChan)
    close(p.highPool.msgChan)
    close(p.normalPool.msgChan)
    close(p.lowPool.msgChan)

    p.wg.Wait()
}
```

**2. Configuration for Worker Pools**

```go
// internal/config/settings.go - Add WorkerPoolConfig
type WorkerPoolConfig struct {
    UrgentWorkers  int `envconfig:"WORKER_POOL_URGENT" default:"10"`
    HighWorkers    int `envconfig:"WORKER_POOL_HIGH" default:"6"`
    NormalWorkers  int `envconfig:"WORKER_POOL_NORMAL" default:"3"`
    LowWorkers     int `envconfig:"WORKER_POOL_LOW" default:"1"`
}

// Add to ServiceConfig
type ServiceConfig struct {
    // ... existing fields
    WorkerPool WorkerPoolConfig `json:"worker_pool"`
}
```

**3. Update Subscriber Runtime**

```go
// internal/runtime/subscriber.go - Update to use worker pools
func (c *SubscriberCtx) start() {
    // Initialize priority worker pool
    workerPool := queue.NewPriorityWorkerPool(
        c.deps.cfg.WorkerPool.UrgentWorkers,
        c.deps.cfg.WorkerPool.HighWorkers,
        c.deps.cfg.WorkerPool.NormalWorkers,
        c.deps.cfg.WorkerPool.LowWorkers,
        c.deps.Workers.AnalysisWorker.ProcessMessage,
        c.deps.logger,
    )

    workerPool.Start(c.backgroundActorCtx)

    go func() {
        c.deps.logger.Info().
            Str("queue", c.deps.cfg.Queue.QueueName).
            Msg("starting outbox subscriber service with priority worker pools")

        // Custom consumer that routes to worker pools
        deliveries := c.deps.Infra.QueueClient.StartConsumer(
            c.backgroundActorCtx,
            c.deps.cfg.Queue.QueueName,
            "analysis-worker",
            func(ctx context.Context, msg queue.Message, ctrl *queue.MsgController) error {
                // Extract priority from message
                priority := extractPriority(msg)

                // Route to appropriate worker pool
                return workerPool.RouteMessage(msg, priority)
            },
            queue.WithConsumingLogger(queue.NewLoggerAdapter(c.deps.logger)),
            queue.WithErrorHandler(func(err error) {
                c.deps.logger.Error().Err(err).Msg("consumer error")
            }),
        )

        if err != nil && !errors.Is(err, context.Canceled) {
            c.deps.logger.Fatal().Err(err).Msg("analysis worker failed")
        }
    }()
}

func extractPriority(msg queue.Message) string {
    // Extract priority from message metadata or payload
    // This depends on how you structure your messages
    var payload domain.AnalysisRequestPayload
    if err := msg.Unmarshal(&payload); err != nil {
        return "normal" // Default
    }

    return string(payload.Priority)
}
```

#### Phase 3: Metrics and Monitoring (Week 2)

```go
// internal/infrastructure/metrics.go - Add worker pool metrics
var (
    workerPoolActiveWorkers = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "worker_pool_active_workers",
            Help: "Number of active workers per priority",
        },
        []string{"priority"},
    )

    workerPoolQueueDepth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "worker_pool_queue_depth",
            Help: "Number of messages waiting in worker pool queue",
        },
        []string{"priority"},
    )

    workerPoolProcessedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "worker_pool_processed_total",
            Help: "Total messages processed by worker pool",
        },
        []string{"priority", "status"},
    )
)
```

### Configuration Example

```yaml
# Environment Variables
RABBITMQ_MAX_PRIORITY=10           # Enable priority queue
RABBITMQ_PREFETCH_COUNT=50         # Increase prefetch for worker pools

# Worker pool configuration (total 20 workers)
WORKER_POOL_URGENT=10              # 50% of capacity for urgent
WORKER_POOL_HIGH=6                 # 30% of capacity for high
WORKER_POOL_NORMAL=3               # 15% of capacity for normal
WORKER_POOL_LOW=1                  # 5% of capacity for low
```

### Expected Improvements

**Before:**
- All priorities share 10 concurrent workers (prefetch count)
- High-priority requests wait in same queue as low-priority
- No prioritization at RabbitMQ level
- Fixed resource allocation regardless of workload

**After:**
- RabbitMQ delivers high-priority messages first
- Urgent requests get 10 dedicated goroutine workers (50% capacity)
- High requests get 6 workers (30% capacity)
- Total concurrency: 20 workers (configurable via environment)
- Better resource utilization based on priority
- Graceful degradation under load (low-priority gets minimal resources)

### Alternative Options Considered

#### ❌ Option 1: Multiple Priority Queues
- Create separate queues per priority level
- **Why not**: Requires complex routing logic, harder to rebalance workers

#### ❌ Option 3: Separate Subscriber Instances
- Deploy 4 separate subscriber services (one per priority)
- **Why not**: Infrastructure overhead, resource inefficiency, complex orchestration

#### ✅ Option 4: Enhanced Prefetch + Channel-Based Pools
- Increase prefetch count significantly (e.g., 100)
- Use Go channels with buffering per priority
- **Fallback**: Simpler RabbitMQ setup but more memory usage

### Testing Strategy

```go
// Test priority ordering
func TestPriorityOrdering(t *testing.T) {
    t.Parallel()

    // Publish messages with different priorities
    publishMessage(t, "low", domain.PriorityLow)
    publishMessage(t, "normal", domain.PriorityNormal)
    publishMessage(t, "high", domain.PriorityHigh)
    publishMessage(t, "urgent", domain.PriorityUrgent)

    // Verify processing order: urgent → high → normal → low
    results := collectProcessedMessages(t, 4)
    assert.Equal(t, "urgent", results[0].Priority)
    assert.Equal(t, "high", results[1].Priority)
    assert.Equal(t, "normal", results[2].Priority)
    assert.Equal(t, "low", results[3].Priority)
}

// Test worker pool distribution
func TestWorkerPoolDistribution(t *testing.T) {
    t.Parallel()

    // Publish 100 messages (25 per priority)
    for i := 0; i < 25; i++ {
        publishMessage(t, fmt.Sprintf("urgent-%d", i), domain.PriorityUrgent)
        publishMessage(t, fmt.Sprintf("high-%d", i), domain.PriorityHigh)
        publishMessage(t, fmt.Sprintf("normal-%d", i), domain.PriorityNormal)
        publishMessage(t, fmt.Sprintf("low-%d", i), domain.PriorityLow)
    }

    // Verify urgent messages processed fastest
    metrics := collectMetrics(t)
    assert.True(t, metrics.UrgentAvgDuration < metrics.HighAvgDuration)
    assert.True(t, metrics.HighAvgDuration < metrics.NormalAvgDuration)
}

// Load test with priority distribution
func BenchmarkPriorityWorkerPool(b *testing.B) {
    // Simulate realistic workload: 10% urgent, 20% high, 50% normal, 20% low
    for i := 0; i < b.N; i++ {
        priority := selectPriorityWeighted()
        publishMessage(b, fmt.Sprintf("msg-%d", i), priority)
    }
}
```

### Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Increased memory usage (50 prefetch + buffered channels) | Medium | Monitor with Prometheus metrics, tune prefetch and buffer sizes |
| Worker pool starvation for low priority | Medium | Guarantee minimum 1 worker for low priority, alert on queue depth |
| RabbitMQ compatibility | Low | Priority queues available since RabbitMQ 3.5 (2014), widely supported |
| Message routing errors | High | Comprehensive error handling, default to normal priority on errors |
| Difficult to tune worker ratios | Medium | Externalize configuration, provide tuning guide in documentation |

### Monitoring Dashboard

**Grafana Panels:**
1. Worker Pool Active Workers (gauge by priority)
2. Worker Pool Queue Depth (gauge by priority)
3. Messages Processed per Second (counter by priority)
4. Average Processing Duration (histogram by priority)
5. Priority Distribution (pie chart)
6. RabbitMQ Queue Priority Distribution

### Implementation Checklist

**Week 1: RabbitMQ Priority Queue**
- [ ] Add `MaxPriority` to `QueueConfig` in settings.go
- [ ] Update queue declaration to include `x-max-priority` argument
- [ ] Add `priority` field to `publisherOptions`
- [ ] Implement `WithPriority()` option function
- [ ] Add priority mapping function (domain → AMQP)
- [ ] Update publisher to use `WithPriority()` when publishing
- [ ] Test priority queue in RabbitMQ management UI
- [ ] Verify messages delivered in priority order

**Week 2: Worker Pools**
- [ ] Create `internal/adapters/queue/worker_pool.go`
- [ ] Implement `PriorityWorkerPool` struct
- [ ] Implement `WorkerPool` struct with goroutine management
- [ ] Add `WorkerPoolConfig` to settings
- [ ] Update `subscriber.go` to initialize worker pool
- [ ] Implement message routing to appropriate worker pool
- [ ] Add graceful shutdown for all worker pools
- [ ] Add Prometheus metrics for worker pools

**Week 3: Testing & Documentation**
- [ ] Write unit tests for priority mapping
- [ ] Write unit tests for worker pool routing
- [ ] Write integration test for priority ordering
- [ ] Load test with mixed priority workload
- [ ] Verify high-priority processed faster under load
- [ ] Create Grafana dashboard for monitoring
- [ ] Document configuration options
- [ ] Write tuning guide for worker ratios
- [ ] Update architecture documentation

### Timeline

- **Week 1**: RabbitMQ priority queue implementation (5 days)
- **Week 2**: Worker pool implementation and metrics (5 days)
- **Week 3**: Testing, monitoring, documentation (5 days)

**Total Effort**: 3 weeks with comprehensive testing

### Success Metrics

| Metric | Target | Measure |
|--------|--------|---------|
| Urgent message processing time | <1s p99 | Prometheus histogram |
| High-priority throughput | 2x normal | Messages/second ratio |
| Worker utilization | >80% | Active workers / total workers |
| Low-priority starvation | 0 occurrences | Alert on queue depth >100 for >5min |

---

## 19. TODO Comments Resolution 🟡 **HIGH**

### Current State
- Multiple TODO/Todo comments throughout the codebase indicate incomplete implementations
- Production-critical features have placeholder implementations
- Testing gaps identified but not yet addressed

### Identified TODOs

#### 1. Authentication: Production Key Management 🔴 **CRITICAL**
**Location**: `internal/adapters/middleware/auth.go:39`

```go
// Todo: In production, this should be loaded from config or a key management service
```

**Issue**: PASETO public key is currently hardcoded in the middleware.

**Required Action**:
```yaml
Priority: CRITICAL
Impact: Security vulnerability in production
Tasks:
  - Move PASETO public key to configuration management
  - Integrate with Vault secrets manager (already configured)
  - Add key rotation capability
  - Implement key versioning for zero-downtime updates
  - Add monitoring for key expiration
```

**Implementation**:
```go
// internal/adapters/middleware/auth.go
type AuthMiddleware struct {
    secretsRepo ports.SecretsRepository
    keyVersion  string
}

func NewAuthMiddleware(secretsRepo ports.SecretsRepository) *AuthMiddleware {
    return &AuthMiddleware{
        secretsRepo: secretsRepo,
        keyVersion:  "v1", // Can be configured
    }
}

func (m *AuthMiddleware) loadPublicKey(ctx context.Context) (ed25519.PublicKey, error) {
    keyPath := fmt.Sprintf("auth/paseto/public-key/%s", m.keyVersion)
    keyData, err := m.secretsRepo.Get(ctx, keyPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load PASETO public key: %w", err)
    }

    publicKey, err := hex.DecodeString(keyData)
    if err != nil {
        return nil, fmt.Errorf("failed to decode public key: %w", err)
    }

    return ed25519.PublicKey(publicKey), nil
}
```

**Vault Setup**:
```bash
# Store PASETO public key in Vault
vault kv put secret/auth/paseto/public-key/v1 \
  key="your_public_key_hex_encoded"

# Enable key rotation with versioning
vault kv put secret/auth/paseto/public-key/v2 \
  key="new_public_key_hex_encoded"
```

---

#### 2. Health Checker: Real Storage Health Check 🟡 **HIGH**
**Location**: `internal/adapters/health_checker.go:117`

```go
case <-time.After(10 * time.Millisecond): // Todo: Apply do the actual check instead of the simulation of the storage check
```

**Issue**: Storage health check is simulated with a timeout instead of actual verification.

**Required Action**:
```yaml
Priority: HIGH
Impact: Unreliable health checks, false positives in production
Tasks:
  - Implement actual PostgreSQL connection test (ping with timeout)
  - Add connection pool health verification
  - Test read capability with lightweight query
  - Verify write capability with test transaction
  - Add degraded state detection (slow but working)
```

**Implementation**:
```go
// internal/adapters/health_checker.go
func (h *HealthChecker) checkStorage(ctx context.Context) *domain.ComponentHealth {
    status := domain.HealthStatusHealthy
    message := "Storage is healthy"

    // Create timeout context for health check
    checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    // Test 1: Basic connectivity with ping
    if err := h.db.PingContext(checkCtx); err != nil {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusUnhealthy,
            Message: fmt.Sprintf("Database ping failed: %v", err),
        }
    }

    // Test 2: Verify read capability with lightweight query
    var result int
    if err := h.db.QueryRowContext(checkCtx, "SELECT 1").Scan(&result); err != nil {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusDegraded,
            Message: fmt.Sprintf("Database read check failed: %v", err),
        }
    }

    // Test 3: Check connection pool stats
    stats := h.db.Stats()
    if stats.OpenConnections >= stats.MaxOpenConnections {
        status = domain.HealthStatusDegraded
        message = fmt.Sprintf(
            "Connection pool exhausted: %d/%d connections in use",
            stats.OpenConnections,
            stats.MaxOpenConnections,
        )
    }

    // Test 4: Verify write capability (optional, can be expensive)
    // Only run full write test if explicitly requested
    if h.config.FullHealthCheck {
        tx, err := h.db.BeginTx(checkCtx, nil)
        if err != nil {
            status = domain.HealthStatusDegraded
            message = fmt.Sprintf("Transaction start failed: %v", err)
        } else {
            _ = tx.Rollback() // Always rollback test transaction
        }
    }

    return &domain.ComponentHealth{
        Status:  status,
        Message: message,
        Metadata: map[string]interface{}{
            "open_connections": stats.OpenConnections,
            "max_connections":  stats.MaxOpenConnections,
            "idle_connections": stats.Idle,
            "in_use":           stats.InUse,
        },
    }
}
```

---

#### 3. Health Checker: Real Cache Health Check 🟡 **HIGH**
**Location**: `internal/adapters/health_checker.go:146`

```go
case <-time.After(5 * time.Millisecond): // Todo: Simulate cache check
```

**Issue**: Cache (KeyDB/Redis) health check is simulated instead of actual verification.

**Required Action**:
```yaml
Priority: HIGH
Impact: Unreliable cache health status, potential service degradation undetected
Tasks:
  - Implement actual KeyDB PING command
  - Test SET/GET operations with test key
  - Verify expiration functionality
  - Check memory usage and eviction policy status
  - Add latency measurement (slow cache detection)
```

**Implementation**:
```go
// internal/adapters/health_checker.go
func (h *HealthChecker) checkCache(ctx context.Context) *domain.ComponentHealth {
    status := domain.HealthStatusHealthy
    message := "Cache is healthy"

    checkCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
    defer cancel()

    // Test 1: Basic connectivity with PING
    start := time.Now()
    if err := h.cache.Ping(checkCtx); err != nil {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusUnhealthy,
            Message: fmt.Sprintf("Cache ping failed: %v", err),
        }
    }
    pingLatency := time.Since(start)

    // Test 2: Verify write capability
    testKey := "health:check:test"
    testValue := time.Now().String()
    if err := h.cache.Set(checkCtx, testKey, testValue, 10*time.Second); err != nil {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusDegraded,
            Message: fmt.Sprintf("Cache write failed: %v", err),
        }
    }

    // Test 3: Verify read capability
    retrievedValue, err := h.cache.Get(checkCtx, testKey)
    if err != nil {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusDegraded,
            Message: fmt.Sprintf("Cache read failed: %v", err),
        }
    }
    if retrievedValue != testValue {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusDegraded,
            Message: "Cache data integrity check failed",
        }
    }

    // Test 4: Clean up test key
    _ = h.cache.Delete(checkCtx, testKey)

    // Test 5: Check performance (high latency indicates problems)
    if pingLatency > 100*time.Millisecond {
        status = domain.HealthStatusDegraded
        message = fmt.Sprintf("Cache responding slowly: %v", pingLatency)
    }

    // Test 6: Check memory usage (if supported by cache implementation)
    if info, err := h.cache.Info(checkCtx); err == nil {
        usedMemory := info["used_memory"].(int64)
        maxMemory := info["maxmemory"].(int64)
        if maxMemory > 0 && float64(usedMemory)/float64(maxMemory) > 0.9 {
            status = domain.HealthStatusDegraded
            message = fmt.Sprintf("Cache memory usage high: %.1f%%",
                float64(usedMemory)/float64(maxMemory)*100)
        }
    }

    return &domain.ComponentHealth{
        Status:  status,
        Message: message,
        Metadata: map[string]interface{}{
            "ping_latency_ms": pingLatency.Milliseconds(),
        },
    }
}
```

---

#### 4. Health Checker: Complete Queue Health Check 🟢 **MEDIUM**
**Location**: `internal/adapters/health_checker.go:176`

```go
// Todo: Continue with health check
```

**Issue**: Queue health check implementation is incomplete.

**Required Action**:
```yaml
Priority: MEDIUM
Impact: Cannot detect RabbitMQ issues affecting event processing
Tasks:
  - Implement RabbitMQ connection health check
  - Verify channel status and availability
  - Check queue depth and consumer status
  - Monitor message rates and acknowledgments
  - Detect message accumulation (backlog)
  - Add cluster node status for HA setups
```

**Implementation**:
```go
// internal/adapters/health_checker.go
func (h *HealthChecker) checkQueue(ctx context.Context) *domain.ComponentHealth {
    status := domain.HealthStatusHealthy
    message := "Queue is healthy"

    // Test 1: Connection status
    if h.queue.IsClosed() {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusUnhealthy,
            Message: "RabbitMQ connection is closed",
        }
    }

    // Test 2: Channel availability
    ch, err := h.queue.Channel()
    if err != nil {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusUnhealthy,
            Message: fmt.Sprintf("Cannot create channel: %v", err),
        }
    }
    defer ch.Close()

    // Test 3: Check queue status
    queueInfo, err := ch.QueueInspect(h.config.QueueName)
    if err != nil {
        return &domain.ComponentHealth{
            Status:  domain.HealthStatusDegraded,
            Message: fmt.Sprintf("Cannot inspect queue: %v", err),
        }
    }

    // Test 4: Detect message backlog
    if queueInfo.Messages > h.config.BacklogThreshold {
        status = domain.HealthStatusDegraded
        message = fmt.Sprintf(
            "Message backlog detected: %d messages (threshold: %d)",
            queueInfo.Messages,
            h.config.BacklogThreshold,
        )
    }

    // Test 5: Check consumer count
    if queueInfo.Consumers == 0 {
        status = domain.HealthStatusDegraded
        message = "No active consumers on queue"
    }

    return &domain.ComponentHealth{
        Status:  status,
        Message: message,
        Metadata: map[string]interface{}{
            "messages":  queueInfo.Messages,
            "consumers": queueInfo.Consumers,
            "queue":     h.config.QueueName,
        },
    }
}
```

---

#### 5. Testing: Integration Test for StartAnalysis 🟡 **HIGH**
**Location**: `internal/service/application_service_test.go:173`

```go
// Todo: Test StartAnalysis success - This test requires database transactions so should be an integration test
```

**Issue**: Critical test case missing for the main analysis workflow.

**Required Action**:
```yaml
Priority: HIGH
Impact: Insufficient test coverage for core functionality
Tasks:
  - Create integration test with testcontainers
  - Test complete StartAnalysis workflow with real database
  - Verify outbox event creation
  - Test transaction rollback on errors
  - Validate analysis state transitions
  - Test concurrent analysis requests
```

**Implementation**:
```go
// itest/application_service_integration_test.go
package itest

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"

    "github.com/architeacher/svc-web-analyzer/internal/domain"
    "github.com/architeacher/svc-web-analyzer/internal/service"
)

func TestApplicationService_StartAnalysis_Success(t *testing.T) {
    t.Parallel()

    // Setup testcontainer for PostgreSQL
    ctx := context.Background()
    pgContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
        postgres.WithDatabase("test_db"),
        postgres.WithUsername("test_user"),
        postgres.WithPassword("test_pass"),
    )
    require.NoError(t, err)
    defer pgContainer.Terminate(ctx)

    // Get connection string and setup database
    connStr, err := pgContainer.ConnectionString(ctx)
    require.NoError(t, err)

    db, err := setupDatabase(connStr)
    require.NoError(t, err)
    defer db.Close()

    // Run migrations
    err = runMigrations(db)
    require.NoError(t, err)

    // Setup service with real repositories
    analysisRepo := repos.NewAnalysisRepository(db)
    outboxRepo := repos.NewOutboxRepository(db)
    appService := service.NewApplicationService(analysisRepo, outboxRepo)

    // Test StartAnalysis with real transaction
    t.Run("should create analysis and outbox event in transaction", func(t *testing.T) {
        cmd := &commands.StartAnalysisCommand{
            URL: "https://example.com",
            Options: &domain.AnalysisOptions{
                Timeout: 30 * time.Second,
                Headers: map[string]string{
                    "User-Agent": "WebAnalyzer/1.0",
                },
            },
        }

        analysisID, err := appService.StartAnalysis(ctx, cmd)
        require.NoError(t, err)
        assert.NotEqual(t, uuid.Nil, analysisID)

        // Verify analysis record created
        analysis, err := analysisRepo.GetByID(ctx, analysisID.String())
        require.NoError(t, err)
        assert.Equal(t, cmd.URL, analysis.URL)
        assert.Equal(t, domain.AnalysisStatusPending, analysis.Status)

        // Verify outbox event created
        events, err := outboxRepo.FindUnpublished(ctx, 10)
        require.NoError(t, err)
        require.Len(t, events, 1)
        assert.Equal(t, domain.EventTypeAnalysisRequested, events[0].EventType)
        assert.Equal(t, analysisID.String(), events[0].AggregateID)
    })

    t.Run("should rollback on error", func(t *testing.T) {
        // Test with invalid URL to trigger error
        cmd := &commands.StartAnalysisCommand{
            URL: "invalid-url",
        }

        analysisID, err := appService.StartAnalysis(ctx, cmd)
        require.Error(t, err)
        assert.Equal(t, uuid.Nil, analysisID)

        // Verify nothing was persisted
        _, err = analysisRepo.GetByID(ctx, analysisID.String())
        assert.Error(t, err)

        events, err := outboxRepo.FindUnpublished(ctx, 10)
        require.NoError(t, err)
        assert.Len(t, events, 0)
    })

    t.Run("should handle concurrent requests", func(t *testing.T) {
        const concurrency = 10
        results := make(chan uuid.UUID, concurrency)
        errors := make(chan error, concurrency)

        for i := 0; i < concurrency; i++ {
            go func(idx int) {
                cmd := &commands.StartAnalysisCommand{
                    URL: fmt.Sprintf("https://example-%d.com", idx),
                }
                id, err := appService.StartAnalysis(ctx, cmd)
                if err != nil {
                    errors <- err

                    return
                }
                results <- id
            }(i)
        }

        // Collect results
        successCount := 0
        for i := 0; i < concurrency; i++ {
            select {
            case <-results:
                successCount++
            case err := <-errors:
                t.Logf("Concurrent request failed: %v", err)
            }
        }

        assert.Equal(t, concurrency, successCount, "All concurrent requests should succeed")
    })
}
```

---

### Implementation Priority

| TODO | Priority | Effort | Risk | Order |
|------|----------|--------|------|-------|
| 1. Auth Key Management | 🔴 CRITICAL | 3 days | High - Security | 1 |
| 5. StartAnalysis Integration Test | 🟡 HIGH | 2 days | Medium - Quality | 2 |
| 2. Storage Health Check | 🟡 HIGH | 1 day | Low - Operations | 3 |
| 3. Cache Health Check | 🟡 HIGH | 1 day | Low - Operations | 4 |
| 4. Queue Health Check | 🟢 MEDIUM | 1 day | Low - Operations | 5 |

**Total Effort**: 8 days (1.6 weeks)

### Implementation Checklist

**Phase 1: Authentication Security (Priority 1)**
- [ ] Move PASETO public key to Vault secrets manager
- [ ] Update `AuthMiddleware` to load key from Vault
- [ ] Implement key version management
- [ ] Add key rotation capability
- [ ] Update configuration for key path and version
- [ ] Add monitoring for key loading failures
- [ ] Test with both key versions to ensure zero-downtime rotation
- [ ] Update deployment documentation

**Phase 2: Integration Testing (Priority 2)**
- [ ] Create `itest/application_service_integration_test.go`
- [ ] Add testcontainers PostgreSQL setup
- [ ] Implement `TestApplicationService_StartAnalysis_Success`
- [ ] Test transaction commit scenario
- [ ] Test transaction rollback scenario
- [ ] Test concurrent request handling
- [ ] Update CI/CD pipeline to run integration tests
- [ ] Add test documentation

**Phase 3: Health Check Implementations (Priorities 3-5)**
- [ ] Implement `checkStorage()` with real PostgreSQL health verification
- [ ] Implement `checkCache()` with real KeyDB/Redis health verification
- [ ] Complete `checkQueue()` implementation for RabbitMQ
- [ ] Add health check configuration options (thresholds, timeouts)
- [ ] Add health check metrics to Prometheus
- [ ] Update health check endpoint response format
- [ ] Add Grafana dashboard for health status visualization
- [ ] Test health checks under failure scenarios

**Phase 4: Documentation & Monitoring**
- [ ] Document PASETO key management procedure
- [ ] Document health check behavior and thresholds
- [ ] Add runbook for handling degraded health states
- [ ] Create alerts for health check failures
- [ ] Update API documentation for enhanced health endpoint

### Benefits

**Security Improvements:**
- Production-ready authentication with proper key management
- Key rotation capability for zero-downtime security updates
- Audit trail for key access via Vault

**Reliability Improvements:**
- Accurate health checks prevent false positives/negatives
- Early detection of infrastructure issues
- Better observability for production operations

**Testing Improvements:**
- Higher confidence in core functionality
- Integration tests validate real-world scenarios
- Better coverage for transaction handling

### Success Metrics

| Metric | Target | Validation |
|--------|--------|------------|
| Auth key load from Vault | 100% | No hardcoded keys in code |
| Health check accuracy | 99%+ | Compare with actual service status |
| Integration test coverage | >80% | Cover critical paths |
| Health check response time | <100ms | Monitor P95 latency |

---

## 20. Analysis Results Export 🟡 **HIGH**

### Current State
- Analysis results only available via JSON API
- No export functionality for reports or data analysis
- Users cannot integrate results with external tools
- No support for business reporting formats

### Required Improvements

Add comprehensive export functionality supporting multiple formats for different use cases.

#### Supported Export Formats

1. **CSV (Comma-Separated Values)**
   - Tabular data for spreadsheet applications
   - Multiple CSV files per analysis (overview.csv, links.csv, forms.csv, resources.csv)
   - Compatible with Excel, Google Sheets, data analysis tools

2. **PDF (Portable Document Format)**
   - Professional branded reports with charts and visualizations
   - Summary and detailed views
   - Client-ready deliverables

3. **JSON (JavaScript Object Notation)**
   - Machine-readable structured data
   - Enhanced export endpoint with formatting options
   - API integration support

4. **Excel (XLSX)**
   - Rich spreadsheets with multiple sheets
   - Formatted tables and charts
   - Analysis Overview, Link Analysis, Form Detection, Media Resources sheets
   - Formulas and conditional formatting

### API Endpoints

```yaml
GET /v1/analysis/{analysisId}/export?format={csv|pdf|json|xlsx}
  Description: Export analysis results in specified format
  Query Parameters:
    - format: Export format (csv, pdf, json, xlsx)
    - template: PDF template name (default, professional, detailed)
    - include: Comma-separated sections (overview,links,forms,media)
  Response: File download with appropriate Content-Type

GET /v1/analysis/{analysisId}/export/{format}
  Description: Alternative endpoint with format in path
  Path Parameters:
    - format: Export format
  Response: File download

GET /v1/exports
  Description: List export history
  Response: Array of export records

GET /v1/exports/{exportId}
  Description: Download previously generated export
  Response: File download
```

### Domain Models

```go
package domain

type ExportFormat string

const (
    ExportFormatCSV  ExportFormat = "csv"
    ExportFormatPDF  ExportFormat = "pdf"
    ExportFormatJSON ExportFormat = "json"
    ExportFormatXLSX ExportFormat = "xlsx"
)

type ExportConfig struct {
    Format   ExportFormat
    Template string   // For PDF: default, professional, detailed
    Include  []string // Sections to include: overview, links, forms, media
    Branding *BrandingConfig
}

type BrandingConfig struct {
    LogoURL     string
    CompanyName string
    Colors      ColorScheme
}

type ColorScheme struct {
    Primary   string
    Secondary string
    Accent    string
}

type ExportHistory struct {
    ID         uuid.UUID
    AnalysisID uuid.UUID
    Format     ExportFormat
    FileSize   int64
    FilePath   string
    GeneratedAt time.Time
    DownloadCount int
    ExpiresAt  *time.Time
}

type Exporter interface {
    Export(ctx context.Context, analysis *Analysis, config ExportConfig) ([]byte, error)
    ContentType() string
    FileExtension() string
}
```

### Database Schema

```sql
-- Migration: 20251025000001_create_export_history_table.up.sql
CREATE TABLE export_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id UUID NOT NULL REFERENCES analysis(id) ON DELETE CASCADE,
    format VARCHAR(10) NOT NULL CHECK (format IN ('csv', 'pdf', 'json', 'xlsx')),
    file_size BIGINT NOT NULL,
    file_path TEXT NOT NULL,
    generated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    download_count INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMP,
    INDEX idx_export_history_analysis_id (analysis_id),
    INDEX idx_export_history_expires_at (expires_at) WHERE expires_at IS NOT NULL
);

COMMENT ON TABLE export_history IS 'Tracks generated export files for analysis results';
COMMENT ON COLUMN export_history.file_path IS 'Storage path or S3 key for the generated file';
COMMENT ON COLUMN export_history.expires_at IS 'When the export file should be deleted (default: 7 days)';
```

### Dependencies

```go
// go.mod additions
require (
    github.com/johnfercher/maroto/v2 v2.0.0-beta.12  // PDF generation
    github.com/xuri/excelize/v2 v2.8.1                // Excel generation
)
```

### Implementation Checklist

- [ ] Add export domain models to `internal/domain/export.go`
- [ ] Create export service interface in `internal/ports/export.go`
- [ ] Implement CSV exporter
- [ ] Implement PDF exporter with Maroto library
- [ ] Implement Excel exporter with Excelize library
- [ ] Enhance JSON export with formatting options
- [ ] Create export history repository
- [ ] Create export history table migration
- [ ] Implement export service with file storage
- [ ] Add export API endpoints to OpenAPI specification
- [ ] Implement export HTTP handlers
- [ ] Add rate limiting for export operations
- [ ] Add export file cleanup job (delete expired exports)
- [ ] Write unit tests for each exporter
- [ ] Write integration tests for export endpoints
- [ ] Add export examples to API documentation
- [ ] Create PDF templates (default, professional, detailed)
- [ ] Add export metrics to monitoring

### Benefits

- **Multi-format Support**: Users choose their preferred format for different workflows
- **Professional Reports**: Branded PDF reports for client delivery and stakeholders
- **Data Analysis**: CSV/Excel for business analysts and data scientists
- **Automation**: JSON exports for programmatic integration with external systems
- **Archival**: Export and store analysis results for compliance and historical tracking
- **Offline Access**: Download results for offline viewing and sharing
- **Business Intelligence**: Excel exports with charts for executive dashboards

---

## 21. Webhook Integration 🟡 **HIGH**

### Current State
- No mechanism to notify external systems of analysis completion
- Users must poll API for status updates
- No integration with workflow automation tools
- Limited event notification capabilities

### Overview

Implement comprehensive webhook functionality to push analysis completion events to external systems with:

- **Asynchronous Delivery**: Non-blocking webhook calls with dedicated worker pool
- **Retry Mechanism**: Exponential backoff with configurable max retries (default: 5)
- **HMAC-SHA256 Signatures**: Secure payload signing for authenticity verification
- **Delivery Tracking**: Complete audit trail of all webhook attempts
- **Multiple Endpoints**: Support multiple webhook configurations per user/organization
- **Event Filtering**: Subscribe to specific events (currently `analysis.completed`)

### API Endpoints

```yaml
# Webhook Configuration Management
POST /v1/webhooks
  Description: Create a new webhook configuration
  Request Body:
    {
      "name": "Production Webhook",
      "url": "https://example.com/webhooks/analysis",
      "events": ["analysis.completed"],
      "active": true,
      "retry_enabled": true,
      "max_retries": 5,
      "timeout_seconds": 30
    }
  Response: 201 Created with webhook configuration

GET /v1/webhooks
  Description: List all webhook configurations
  Query Parameters:
    - active: Filter by active status (true/false)
    - page: Page number (default: 1)
    - limit: Items per page (default: 20)
  Response: Array of webhook configurations

GET /v1/webhooks/{id}
  Description: Get webhook configuration details
  Response: Webhook configuration object

PATCH /v1/webhooks/{id}
  Description: Update webhook configuration
  Request Body: Partial webhook object
  Response: Updated webhook configuration

DELETE /v1/webhooks/{id}
  Description: Delete webhook configuration
  Response: 204 No Content

# Webhook Testing
POST /v1/webhooks/{id}/test
  Description: Send a test payload to webhook
  Response: Test delivery result

# Webhook Deliveries
GET /v1/webhooks/{id}/deliveries
  Description: List delivery attempts for webhook
  Query Parameters:
    - status: Filter by status (pending, success, failed)
    - limit: Max results (default: 50)
  Response: Array of delivery records

GET /v1/webhooks/{id}/deliveries/{deliveryId}
  Description: Get delivery attempt details
  Response: Delivery record with full details

POST /v1/webhooks/{id}/deliveries/{deliveryId}/redeliver
  Description: Manually retry a failed delivery
  Response: New delivery attempt result
```

### Database Schema

```sql
-- Migration: 20251025000002_create_webhook_tables.up.sql

-- Webhook configurations
CREATE TABLE webhook_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,  -- Optional: link to users table if multi-tenant
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL,  -- For HMAC-SHA256 signing
    events TEXT[] NOT NULL DEFAULT ARRAY['analysis.completed'],
    active BOOLEAN NOT NULL DEFAULT true,
    retry_enabled BOOLEAN NOT NULL DEFAULT true,
    max_retries INT NOT NULL DEFAULT 5 CHECK (max_retries >= 0 AND max_retries <= 10),
    timeout_seconds INT NOT NULL DEFAULT 30 CHECK (timeout_seconds > 0 AND timeout_seconds <= 300),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_url CHECK (url ~ '^https?://'),
    INDEX idx_webhook_configs_user_id (user_id) WHERE user_id IS NOT NULL,
    INDEX idx_webhook_configs_active (active) WHERE active = true
);

COMMENT ON TABLE webhook_configs IS 'Webhook endpoint configurations for event notifications';
COMMENT ON COLUMN webhook_configs.secret IS 'Secret key for HMAC-SHA256 signature generation';
COMMENT ON COLUMN webhook_configs.events IS 'Array of event types this webhook subscribes to';

-- Webhook deliveries
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_config_id UUID NOT NULL REFERENCES webhook_configs(id) ON DELETE CASCADE,
    analysis_id UUID NOT NULL REFERENCES analysis(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    signature VARCHAR(255) NOT NULL,  -- HMAC-SHA256 signature
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'success', 'failed')),
    http_status_code INT,
    response_body TEXT,
    response_headers JSONB,
    error_message TEXT,
    attempt_count INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMP,
    INDEX idx_webhook_deliveries_webhook_id (webhook_config_id),
    INDEX idx_webhook_deliveries_analysis_id (analysis_id),
    INDEX idx_webhook_deliveries_status (status),
    INDEX idx_webhook_deliveries_next_retry (next_retry_at) WHERE status = 'pending' AND next_retry_at IS NOT NULL
);

COMMENT ON TABLE webhook_deliveries IS 'Audit log of all webhook delivery attempts';
COMMENT ON COLUMN webhook_deliveries.signature IS 'HMAC-SHA256 signature of the payload';
COMMENT ON COLUMN webhook_deliveries.next_retry_at IS 'Timestamp for next retry attempt (NULL if no retry scheduled)';
```

### Domain Models

```go
// internal/domain/webhook.go
package domain

import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
)

type DeliveryStatus string

const (
    DeliveryStatusPending DeliveryStatus = "pending"
    DeliveryStatusSuccess DeliveryStatus = "success"
    DeliveryStatusFailed  DeliveryStatus = "failed"
)

type WebhookConfig struct {
    ID             uuid.UUID `json:"id"`
    UserID         *uuid.UUID `json:"user_id,omitempty"`
    Name           string    `json:"name"`
    URL            string    `json:"url"`
    Secret         string    `json:"-"` // Never expose in API responses
    Events         []string  `json:"events"`
    Active         bool      `json:"active"`
    RetryEnabled   bool      `json:"retry_enabled"`
    MaxRetries     int       `json:"max_retries"`
    TimeoutSeconds int       `json:"timeout_seconds"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
    ID              uuid.UUID       `json:"id"`
    WebhookConfigID uuid.UUID       `json:"webhook_config_id"`
    AnalysisID      uuid.UUID       `json:"analysis_id"`
    EventType       string          `json:"event_type"`
    Payload         json.RawMessage `json:"payload"`
    Signature       string          `json:"signature"`
    Status          DeliveryStatus  `json:"status"`
    HTTPStatusCode  *int            `json:"http_status_code,omitempty"`
    ResponseBody    string          `json:"response_body,omitempty"`
    ResponseHeaders json.RawMessage `json:"response_headers,omitempty"`
    ErrorMessage    string          `json:"error_message,omitempty"`
    AttemptCount    int             `json:"attempt_count"`
    NextRetryAt     *time.Time      `json:"next_retry_at,omitempty"`
    CreatedAt       time.Time       `json:"created_at"`
    DeliveredAt     *time.Time      `json:"delivered_at,omitempty"`
}

type WebhookPayload struct {
    Event       string    `json:"event"`
    AnalysisID  uuid.UUID `json:"analysis_id"`
    Timestamp   time.Time `json:"timestamp"`
    Data        any       `json:"data"`
}
```

### Webhook Payload Structure

When `analysis.completed` event fires:

```json
{
  "event": "analysis.completed",
  "analysis_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-10-25T12:00:00Z",
  "data": {
    "url": "https://example.com",
    "status": "completed",
    "html_version": "HTML5",
    "title": "Example Domain",
    "heading_counts": {"h1": 1, "h2": 3, "h3": 5, "h4": 0, "h5": 0, "h6": 0},
    "links": {
      "total_count": 15,
      "internal_count": 10,
      "external_count": 5,
      "inaccessible_links": []
    },
    "forms": {
      "total_count": 1,
      "login_forms_detected": 1,
      "login_form_details": [{"method": "POST", "action": "/login", "fields": ["username", "password"]}]
    },
    "duration_ms": 1234,
    "completed_at": "2025-10-25T12:00:00Z"
  }
}
```

### HTTP Headers

```http
POST /webhooks/analysis HTTP/1.1
Host: example.com
Content-Type: application/json
User-Agent: WebAnalyzer-Webhook/1.0
X-Webhook-ID: 123e4567-e89b-12d3-a456-426614174000
X-Webhook-Delivery: 987fcdeb-51a2-43f7-8d9e-1234567890ab
X-Webhook-Event: analysis.completed
X-Webhook-Signature: sha256=5d41402abc4b2a76b9719d911017c592
X-Webhook-Timestamp: 1698249600
```

### HMAC-SHA256 Signature

```go
// Signature generation
func GenerateSignature(payload []byte, secret string, timestamp int64) string {
    message := fmt.Sprintf("%d.%s", timestamp, string(payload))
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(message))

    return hex.EncodeToString(mac.Sum(nil))
}

// Client-side verification
func VerifyWebhook(payload []byte, secret string, timestamp int64, receivedSignature string) bool {
    expectedSignature := GenerateSignature(payload, secret, timestamp)

    return hmac.Equal([]byte(expectedSignature), []byte(receivedSignature))
}
```

### Retry Strategy

Exponential backoff:
- Attempt 0: Immediate
- Attempt 1: 1 second
- Attempt 2: 2 seconds
- Attempt 3: 4 seconds
- Attempt 4: 8 seconds
- Attempt 5: 16 seconds

After 5 failed attempts, mark as permanently failed.

### Implementation Checklist

- [ ] Add webhook domain models to `internal/domain/webhook.go`
- [ ] Create webhook repository interfaces in `internal/ports/webhook.go`
- [ ] Create webhook configuration table migration
- [ ] Create webhook deliveries table migration
- [ ] Implement webhook configuration repository
- [ ] Implement webhook delivery repository
- [ ] Implement HMAC-SHA256 signature generator
- [ ] Implement retry strategy with exponential backoff
- [ ] Implement webhook delivery service
- [ ] Create webhook worker service runtime
- [ ] Add webhook trigger to subscriber service
- [ ] Add webhook API endpoints to OpenAPI specification
- [ ] Implement webhook HTTP handlers
- [ ] Add webhook configuration CRUD operations
- [ ] Add webhook test endpoint
- [ ] Add webhook delivery history endpoints
- [ ] Implement manual redeliver functionality
- [ ] Write unit tests for signature generation/verification
- [ ] Write unit tests for retry strategy
- [ ] Write integration tests for webhook delivery
- [ ] Add webhook metrics to monitoring
- [ ] Add webhook delivery alerts
- [ ] Create webhook configuration documentation
- [ ] Create example webhook receiver implementations
- [ ] Add webhook security best practices guide
- [ ] Deploy webhook worker service

### Deployment

```yaml
# compose.yaml
services:
  webhook-worker:
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile
      target: webhook-worker
    command: /app/webhook-worker
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - LOG_LEVEL=info
    depends_on:
      - db
      - rabbitmq
    restart: unless-stopped
```

### Monitoring

```go
// Prometheus metrics
webhook_deliveries_total{status="success|failed"}
webhook_delivery_duration_seconds
webhook_retry_count
webhook_active_configs_total
```

### Benefits

- **Real-time Integration**: Immediate notification when analysis completes
- **Workflow Automation**: Trigger downstream processes (Slack, Jira, CI/CD)
- **Reliability**: Exponential backoff retry ensures delivery
- **Security**: HMAC-SHA256 signatures prevent tampering
- **Auditability**: Complete delivery history with timestamps and responses
- **Scalability**: Dedicated worker pool handles high volumes
- **Flexibility**: Multiple webhook endpoints support complex integrations
- **Developer Experience**: Simple receiver implementation with verification examples

---

## 22. Frontend Migration to Vue.js + Tailwind CSS 🟡 **HIGH**

### Current State
- Frontend uses vanilla JavaScript without framework structure
- Plain CSS styling without utility-first approach
- No component architecture or state management
- Manual DOM manipulation and event handling
- No build tooling or hot module replacement
- No TypeScript for type safety
- Limited code organization and scalability
- D3.js loaded from CDN for visualizations

### Overview

Migrate the frontend from vanilla JavaScript to a modern Vue.js 3 application with TypeScript, Tailwind CSS, and Vite for improved developer experience, maintainability, and scalability.

**Technology Stack:**
- **Vue.js 3**: Composition API for reactive UI components
- **TypeScript**: Type-safe development with excellent IDE support
- **Tailwind CSS**: Utility-first CSS framework for rapid UI development
- **Vite**: Fast build tool with HMR (Hot Module Replacement)
- **Pinia**: State management for Vue 3 (replaces Vuex)
- **Vue Router**: Client-side routing for SPA navigation
- **VueUse**: Collection of essential Vue composition utilities
- **Headless UI**: Unstyled, accessible UI components for Vue

### Required Directory Structure

```yaml
web/
├── public/                           # Static assets
│   ├── favicon.ico
│   └── robots.txt
├── src/
│   ├── assets/                       # Images, fonts, global styles
│   │   ├── images/
│   │   └── styles/
│   │       └── main.css             # Tailwind directives
│   ├── components/                   # Reusable Vue components
│   │   ├── common/                   # Common UI components
│   │   │   ├── Button.vue
│   │   │   ├── Card.vue
│   │   │   ├── Input.vue
│   │   │   ├── Spinner.vue
│   │   │   └── ErrorAlert.vue
│   │   ├── analysis/                 # Analysis-specific components
│   │   │   ├── AnalysisForm.vue
│   │   │   ├── AnalysisResults.vue
│   │   │   ├── AnalysisProgress.vue
│   │   │   ├── AnalysisCard.vue
│   │   │   └── AnalysisVisualization.vue
│   │   └── layout/                   # Layout components
│   │       ├── Header.vue
│   │       ├── Footer.vue
│   │       └── Container.vue
│   ├── composables/                  # Vue composition functions
│   │   ├── useAnalysis.ts           # Analysis API integration
│   │   ├── useSSE.ts                # Server-Sent Events handling
│   │   ├── useToast.ts              # Toast notifications
│   │   └── useKonamiCode.ts         # Easter egg functionality
│   ├── stores/                       # Pinia state stores
│   │   ├── analysis.ts              # Analysis state management
│   │   └── ui.ts                    # UI state (modals, toasts)
│   ├── services/                     # API service layer
│   │   └── api.ts                   # API client (from OpenAPI generation)
│   ├── types/                        # TypeScript type definitions
│   │   ├── analysis.ts              # Analysis domain types
│   │   └── api.ts                   # API response types
│   ├── utils/                        # Utility functions
│   │   ├── formatters.ts            # Date, number formatting
│   │   └── validators.ts            # Form validation
│   ├── router/                       # Vue Router configuration
│   │   └── index.ts
│   ├── App.vue                       # Root component
│   ├── main.ts                       # Application entry point
│   └── vite-env.d.ts                # Vite type declarations
├── .env.development                  # Development environment variables
├── .env.production                   # Production environment variables
├── index.html                        # HTML entry point
├── package.json                      # Dependencies and scripts
├── tsconfig.json                     # TypeScript configuration
├── tsconfig.node.json               # TypeScript config for Vite
├── vite.config.ts                   # Vite configuration
├── tailwind.config.js               # Tailwind CSS configuration
├── postcss.config.js                # PostCSS configuration
└── README.md                         # Frontend documentation
```

### Package Dependencies

```json
{
  "name": "@web-analyzer/frontend",
  "version": "1.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc && vite build",
    "preview": "vite preview",
    "type-check": "vue-tsc --noEmit",
    "lint": "eslint . --ext .vue,.js,.jsx,.cjs,.mjs,.ts,.tsx,.cts,.mts --fix --ignore-path .gitignore",
    "format": "prettier --write src/"
  },
  "dependencies": {
    "vue": "^3.4.21",
    "vue-router": "^4.3.0",
    "pinia": "^2.1.7",
    "@vueuse/core": "^10.9.0",
    "@headlessui/vue": "^1.7.18",
    "@heroicons/vue": "^2.1.1",
    "axios": "^1.6.7",
    "d3": "^7.9.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.4",
    "typescript": "^5.4.2",
    "vue-tsc": "^2.0.6",
    "vite": "^5.1.5",
    "tailwindcss": "^3.4.1",
    "postcss": "^8.4.35",
    "autoprefixer": "^10.4.18",
    "@typescript-eslint/parser": "^7.2.0",
    "@typescript-eslint/eslint-plugin": "^7.2.0",
    "eslint": "^8.57.0",
    "eslint-plugin-vue": "^9.22.0",
    "prettier": "^3.2.5",
    "prettier-plugin-tailwindcss": "^0.5.11"
  }
}
```

### Configuration Files

#### Vite Configuration

```typescript
// vite.config.ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/v1': {
        target: 'https://api.web-analyzer.dev',
        changeOrigin: true,
        secure: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'vue-vendor': ['vue', 'vue-router', 'pinia'],
          'd3-vendor': ['d3'],
        },
      },
    },
  },
})
```

#### Tailwind CSS Configuration

```javascript
// tailwind.config.js
/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{vue,js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#f0f9ff',
          100: '#e0f2fe',
          200: '#bae6fd',
          300: '#7dd3fc',
          400: '#38bdf8',
          500: '#0ea5e9',
          600: '#0284c7',
          700: '#0369a1',
          800: '#075985',
          900: '#0c4a6e',
          950: '#082f49',
        },
      },
      animation: {
        'spin-slow': 'spin 3s linear infinite',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
    require('@tailwindcss/aspect-ratio'),
  ],
}
```

#### TypeScript Configuration

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "preserve",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src/**/*.ts", "src/**/*.d.ts", "src/**/*.tsx", "src/**/*.vue"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

### Example Component Implementation

#### Main App Component

```vue
<!-- src/App.vue -->
<script setup lang="ts">
import { RouterView } from 'vue-router'
import Header from '@/components/layout/Header.vue'
import Footer from '@/components/layout/Footer.vue'
import Container from '@/components/layout/Container.vue'
import { useKonamiCode } from '@/composables/useKonamiCode'

// Enable Konami code easter egg
useKonamiCode()
</script>

<template>
  <div class="min-h-screen flex flex-col bg-gradient-to-br from-blue-50 to-indigo-100">
    <Header />

    <main class="flex-1 py-8">
      <Container>
        <RouterView />
      </Container>
    </main>

    <Footer />
  </div>
</template>
```

#### Analysis Form Component

```vue
<!-- src/components/analysis/AnalysisForm.vue -->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAnalysisStore } from '@/stores/analysis'
import Button from '@/components/common/Button.vue'
import Input from '@/components/common/Input.vue'

const router = useRouter()
const analysisStore = useAnalysisStore()

const url = ref('')
const isSubmitting = ref(false)
const error = ref<string | null>(null)

const handleSubmit = async () => {
  if (!url.value) {
    return
  }

  isSubmitting.value = true
  error.value = null

  try {
    const analysis = await analysisStore.submitAnalysis(url.value)

    await router.push({
      name: 'analysis-results',
      params: { id: analysis.id },
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to submit analysis'
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="bg-white rounded-xl shadow-lg p-8">
    <div class="mb-6">
      <h1 class="text-3xl font-bold text-gray-900 flex items-center gap-3">
        <span class="text-4xl">🔍</span>
        Web Analyzer
      </h1>
      <p class="mt-2 text-gray-600">
        Analyze web pages and get detailed insights about HTML structure, links, and forms
      </p>
    </div>

    <form @submit.prevent="handleSubmit" class="space-y-4">
      <Input
        v-model="url"
        type="url"
        label="Enter URL to analyze"
        placeholder="https://example.com"
        required
        autocomplete="url"
        :error="error"
      />

      <Button
        type="submit"
        :loading="isSubmitting"
        :disabled="isSubmitting || !url"
        class="w-full"
      >
        <template v-if="isSubmitting">
          Analyzing...
        </template>
        <template v-else>
          Analyze
        </template>
      </Button>
    </form>
  </div>
</template>
```

#### Analysis Store (Pinia)

```typescript
// src/stores/analysis.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Analysis, AnalysisStatus } from '@/types/analysis'
import { analyzeUrl, getAnalysis } from '@/services/api'

export const useAnalysisStore = defineStore('analysis', () => {
  const analyses = ref<Map<string, Analysis>>(new Map())
  const currentAnalysisId = ref<string | null>(null)

  const currentAnalysis = computed(() => {
    if (!currentAnalysisId.value) {
      return null
    }

    return analyses.value.get(currentAnalysisId.value)
  })

  const submitAnalysis = async (url: string): Promise<Analysis> => {
    const response = await analyzeUrl({ url })
    const analysis: Analysis = {
      id: response.analysisId,
      url,
      status: 'pending' as AnalysisStatus,
      createdAt: new Date().toISOString(),
    }

    analyses.value.set(analysis.id, analysis)
    currentAnalysisId.value = analysis.id

    return analysis
  }

  const fetchAnalysis = async (id: string): Promise<Analysis> => {
    const response = await getAnalysis(id)
    const analysis: Analysis = {
      id: response.id,
      url: response.url,
      status: response.status as AnalysisStatus,
      htmlVersion: response.htmlVersion,
      title: response.title,
      headingCounts: response.headingCounts,
      links: response.links,
      forms: response.forms,
      duration: response.duration,
      createdAt: response.createdAt,
      completedAt: response.completedAt,
    }

    analyses.value.set(id, analysis)
    currentAnalysisId.value = id

    return analysis
  }

  const updateAnalysisStatus = (id: string, status: AnalysisStatus) => {
    const analysis = analyses.value.get(id)
    if (analysis) {
      analysis.status = status
    }
  }

  return {
    analyses,
    currentAnalysis,
    submitAnalysis,
    fetchAnalysis,
    updateAnalysisStatus,
  }
})
```

#### SSE Composable

```typescript
// src/composables/useSSE.ts
import { ref, onUnmounted } from 'vue'
import type { Ref } from 'vue'

interface SSEOptions {
  onMessage?: (event: MessageEvent) => void
  onError?: (event: Event) => void
  onOpen?: (event: Event) => void
}

export const useSSE = (url: Ref<string> | string, options: SSEOptions = {}) => {
  const isConnected = ref(false)
  const error = ref<Error | null>(null)
  let eventSource: EventSource | null = null

  const connect = () => {
    const sseUrl = typeof url === 'string' ? url : url.value

    eventSource = new EventSource(sseUrl)

    eventSource.onopen = (event) => {
      isConnected.value = true
      error.value = null
      options.onOpen?.(event)
    }

    eventSource.onmessage = (event) => {
      options.onMessage?.(event)
    }

    eventSource.onerror = (event) => {
      isConnected.value = false
      error.value = new Error('SSE connection error')
      options.onError?.(event)
    }
  }

  const disconnect = () => {
    if (eventSource) {
      eventSource.close()
      eventSource = null
      isConnected.value = false
    }
  }

  onUnmounted(() => {
    disconnect()
  })

  return {
    isConnected,
    error,
    connect,
    disconnect,
  }
}
```

### Migration Strategy

#### Phase 1: Project Setup
1. Create new Vite + Vue 3 + TypeScript project structure
2. Install dependencies (Vue, Tailwind, Pinia, etc.)
3. Configure Vite, Tailwind, TypeScript
4. Set up ESLint and Prettier
5. Create basic layout components

#### Phase 2: Component Migration
1. Convert HTML structure to Vue components
2. Break down monolithic UI into reusable components
3. Implement Tailwind CSS styling (replace custom CSS)
4. Add TypeScript types for all components
5. Create composables for shared logic

#### Phase 3: State Management
1. Set up Pinia stores for analysis state
2. Implement API service layer with typed interfaces
3. Integrate with generated OpenAPI client
4. Add error handling and loading states

#### Phase 4: Features Migration
1. Migrate analysis form submission
2. Migrate Server-Sent Events (SSE) integration
3. Migrate result visualization with D3.js
4. Migrate progress tracking
5. Migrate Konami code easter egg

#### Phase 5: Testing & Optimization
1. Add Vitest for unit testing
2. Add Playwright for E2E testing
3. Optimize bundle size with code splitting
4. Add PWA support (optional)
5. Performance audit and optimization

#### Phase 6: Deployment
1. Update Docker configuration for production build
2. Update Traefik configuration
3. Deploy to staging environment
4. User acceptance testing
5. Deploy to production

### Docker Integration

```dockerfile
# deployments/docker/Dockerfile.frontend
FROM node:20-alpine AS builder

WORKDIR /app

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Production stage
FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY deployments/docker/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
```

### Nginx Configuration

```nginx
# deployments/docker/nginx.conf
server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/javascript application/xml+rss application/json;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;

    # SPA routing - serve index.html for all routes
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # API proxy (if needed for development)
    location /v1/ {
        proxy_pass https://api.web-analyzer.dev;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

### Implementation Checklist

#### Setup & Configuration
- [ ] Initialize Vite + Vue 3 + TypeScript project
- [ ] Install all required dependencies
- [ ] Configure Tailwind CSS with custom theme
- [ ] Set up TypeScript with strict mode
- [ ] Configure ESLint and Prettier
- [ ] Set up path aliases (@/ for src/)
- [ ] Create environment variable files

#### Component Development
- [ ] Create common UI components (Button, Input, Card, etc.)
- [ ] Create layout components (Header, Footer, Container)
- [ ] Create analysis form component
- [ ] Create analysis results component
- [ ] Create analysis progress component
- [ ] Create error alert component
- [ ] Add loading states and skeletons

#### State & API Integration
- [ ] Set up Pinia store for analysis state
- [ ] Create API service layer
- [ ] Integrate OpenAPI generated client
- [ ] Implement SSE composable for real-time updates
- [ ] Add error handling middleware
- [ ] Implement toast notifications

#### Feature Migration
- [ ] Migrate form submission logic
- [ ] Migrate SSE event handling
- [ ] Migrate D3.js visualizations
- [ ] Migrate progress tracking
- [ ] Migrate Konami code easter egg
- [ ] Add confetti animation support

#### Routing
- [ ] Set up Vue Router
- [ ] Create home route
- [ ] Create analysis results route
- [ ] Add 404 page
- [ ] Add route guards if needed

#### Testing
- [ ] Set up Vitest for unit tests
- [ ] Write component tests
- [ ] Write composable tests
- [ ] Write store tests
- [ ] Set up Playwright for E2E tests
- [ ] Write E2E test scenarios

#### Build & Deployment
- [ ] Configure Vite production build
- [ ] Optimize bundle size
- [ ] Create Dockerfile for frontend
- [ ] Create Nginx configuration
- [ ] Update docker-compose.yaml
- [ ] Update Traefik routing
- [ ] Deploy to staging
- [ ] Deploy to production

#### Documentation
- [ ] Write frontend README
- [ ] Document component API
- [ ] Document composables usage
- [ ] Document state management
- [ ] Add code examples
- [ ] Update main project README

### Benefits

**Developer Experience:**
- Modern tooling with Vite HMR for instant feedback
- TypeScript type safety catches errors at compile time
- Component-based architecture improves code organization
- Composables enable logic reuse across components
- Auto-completion and IntelliSense in IDEs
- ESLint and Prettier enforce code consistency

**Maintainability:**
- Clear separation of concerns with components
- Centralized state management with Pinia
- Testable code with Vitest and Playwright
- Type-safe API integration
- Better error handling and debugging
- Easier onboarding for new developers

**Performance:**
- Optimized bundle size with tree-shaking
- Code splitting for faster initial load
- Lazy loading of routes and components
- Built-in reactivity system (no manual DOM updates)
- Efficient re-rendering with Virtual DOM
- Production builds with minification and compression

**User Experience:**
- Faster page transitions (SPA)
- Responsive design with Tailwind CSS
- Consistent UI with design system
- Better accessibility with Headless UI
- Smooth animations and transitions
- Progressive enhancement

**Scalability:**
- Easy to add new features and pages
- Component library grows with application
- Shared composables reduce code duplication
- Clear patterns for state management
- Easy integration with backend APIs
- Future-proof architecture

---

*This improvement plan provides a clear roadmap to elevate the Web Analyzer service to production-grade quality with enterprise features, including comprehensive data lifecycle management, a modern API Gateway architecture with HTTP→gRPC protocol conversion, priority-based worker allocation for optimal resource utilization, multi-format export functionality for business reporting and automation, webhook integration for real-time event notifications and workflow automation, detailed performance timing breakdown for enhanced observability, a modern Vue.js frontend with TypeScript and Tailwind CSS for improved developer experience and maintainability, and resolution of all identified TODO comments for production readiness.*
