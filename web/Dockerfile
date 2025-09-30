ARG     NGINX_VERSION=1.25.5

# Base stage - common setup for both dev and prod
FROM    nginx:${NGINX_VERSION}-alpine AS base

ARG     ARTIFACTS_DIR="/usr/share/nginx/html"
ARG     CONFIG_DIR="/etc/nginx"
ARG     WORKDIR="/app"

ENV     APP_GROUP="nginx" \
        APP_USER="nginx"

WORKDIR "${WORKDIR}"

# Install necessary packages
RUN     apk --no-cache add -u \
            bash \
            ca-certificates \
            curl \
            jq \
            tzdata \
        && \
        update-ca-certificates

# Create nginx directories and set permissions
RUN     mkdir -p /var/cache/nginx /var/log/nginx /tmp \
        && \
        chown -R "${APP_USER}:${APP_GROUP}" \
            /var/cache/nginx \
            /var/log/nginx \
            /tmp \
            "${CONFIG_DIR}" \
            "${ARTIFACTS_DIR}" \
        && \
        touch /var/log/nginx/access.log /var/log/nginx/error.log \
        && \
        chown "${APP_USER}:${APP_GROUP}" /var/log/nginx/access.log /var/log/nginx/error.log

USER    "${APP_USER}"

EXPOSE  8080

# Development stage - no files copied, expects volume mount
FROM    base AS development

ENV     NODE_ENV="development"

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD     ["nginx", "-g", "daemon off;"]

# Production stage - files baked into image
FROM    base AS production

ENV     NODE_ENV="production"

ARG     ARTIFACTS_DIR="/usr/share/nginx/html"

# Copy static files
COPY    --chown="${APP_USER}:${APP_GROUP}" \
        src/ \
        "${ARTIFACTS_DIR}"

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD     ["nginx", "-g", "daemon off;"]