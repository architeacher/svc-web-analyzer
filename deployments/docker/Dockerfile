ARG     GO_VERSION

# Build Stage
FROM    golang:${GO_VERSION:-1.25}-alpine AS base

LABEL   stage="base"

RUN     go install github.com/air-verse/air@v1.63.0 \
        && \
        go install github.com/go-delve/delve/cmd/dlv@v1.25.2


FROM    golang:${GO_VERSION:-1.25}-alpine

ARG     ARTIFACTS_DIR="/usr/local/bin"
ARG     SRC_ARTIFACTS_DIR="/go/bin"
ARG     PATH_PREFIX="./deployments/docker/svc-web-analyzer"
ARG     WORKDIR="/app"

ENV     CGO_ENABLED=0 \
        APP_ENV="development" \
        APP_GROUP="app" \
        APP_USER="app"

WORKDIR "${WORKDIR}"

RUN     apk --no-cache add -u \
            bash \
            build-base \
            ca-certificates \
            curl \
            git \
            jq \
            tzdata \
        && \
        update-ca-certificates \
        && \
        addgroup -S "${APP_GROUP}" \
        && \
        adduser -S -G "${APP_GROUP}" "${APP_USER}"

COPY    --from="base" \
        --chown="${APP_USER}:${APP_GROUP}" \
        "${SRC_ARTIFACTS_DIR}/air" \
        "${SRC_ARTIFACTS_DIR}/dlv" \
        "${ARTIFACTS_DIR}"

COPY    --chown=${APP_USER}:${APP_GROUP} "${PATH_PREFIX}/config/air/*" /etc/air/

USER    "${APP_USER}"

EXPOSE  8088

