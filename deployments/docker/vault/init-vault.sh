#!/usr/bin/env bash

set -o pipefail

exit_status=0

log_info() {
    echo "[INFO] ${1}"
}

log_warning() {
    echo "[WARNING] ${1}"
}

log_error() {
    echo "[ERROR] ${1}" >&2
}

abort() {
    log_error "${1}"
    exit 1
}

load_configuration() {
    log_info "Loading configuration from .envrc..."

    if [ -f "/app/.envrc" ]; then
        # Save the original VAULT_ADDR from environment
        local original_vault_addr="${VAULT_ADDR}"

        set -a  # automatically export all variables
        . /app/.envrc
        set +a

        # Restore the original VAULT_ADDR if it was set
        if [ -n "${original_vault_addr}" ]; then
            export VAULT_ADDR="${original_vault_addr}"
        fi

        log_info "Loaded configuration from .envrc"
        return 0
    else
        log_warning ".envrc file not found, using environment defaults"
        return 1
    fi
}

wait_for_vault() {
    log_info "Waiting for Vault to be ready..."

    local max_attempts=30
    local attempt=0

    while [ ${attempt} -lt ${max_attempts} ]; do
        if wget --quiet --spider "${VAULT_ADDR}/v1/sys/health" 2>/dev/null; then
            log_info "Vault is ready!"
            return 0
        fi

        log_info "Vault is unavailable - sleeping (attempt ${attempt}/${max_attempts})"
        sleep 2
        attempt=$((attempt + 1))
    done

    abort "Vault failed to become ready after ${max_attempts} attempts"
}

is_vault_initialized() {
    if vault kv get apps/svc-web-analyzer >/dev/null 2>&1; then
        log_info "Vault is already configured"
        return 0
    fi
    return 1
}

setup_vault_authentication() {
    log_info "Setting up Vault authentication..."

    if [ -z "${VAULT_ROOT_TOKEN}" ]; then
        abort "VAULT_ROOT_TOKEN is not set"
    fi

    export VAULT_TOKEN="${VAULT_ROOT_TOKEN}"
}

enable_secret_engines() {
    log_info "Enabling secret engines..."

    log_info "Enabling KV secrets engine at apps path..."
    if ! vault secrets enable -path=apps -version=2 kv; then
        log_info "Engine already exists or failed to enable - continuing..."
    fi
}


store_application_secrets() {
    log_info "Storing application secrets from .envrc..."

    # Use grep to get only the variable assignments, then process them
    local temp_file="/tmp/vault_vars"
    grep -E "^[A-Z_]+=.+" /app/.envrc > "${temp_file}"

    log_info "Found $(wc -l < "${temp_file}") environment variables in .envrc"
    log_info "Variables found:"
    while IFS='=' read -r key value; do
        log_info "  ${key}=${value}"
    done < "${temp_file}"

    # Build vault command arguments and collect keys for logging
    local vault_args_file="/tmp/vault_args"
    local exported_keys=""
    local count=0

    # Create empty args file
    > "${vault_args_file}"

    while IFS='=' read -r key value; do
        # Skip empty lines
        if [ -z "${key}" ]; then
            continue
        fi

        # Remove surrounding quotes if present
        value=$(echo "${value}" | sed 's/^"\(.*\)"$/\1/' | sed "s/^'\(.*\)'$/\1/")
        printf '%s=%s\n' "${key}" "${value}" >> "${vault_args_file}"
        if [ -z "${exported_keys}" ]; then
            exported_keys="${key}"
        else
            exported_keys="${exported_keys}, ${key}"
        fi
        count=$((count + 1))
    done < "${temp_file}"

    rm -f "${temp_file}"

    if [ ${count} -eq 0 ]; then
        rm -f "${vault_args_file}"
        abort "No valid environment variables found in .envrc"
    fi

    log_info "Storing all environment variables under apps/svc-web-analyzer..."

    # Use xargs to pass each line as separate arguments
    if ! xargs vault kv put apps/svc-web-analyzer < "${vault_args_file}"; then
        rm -f "${vault_args_file}"
        abort "Failed to store application secrets"
    fi

    rm -f "${vault_args_file}"
}

create_vault_policy() {
    log_info "Creating application policy..."

    if ! vault policy write web-analyzer-policy - <<EOF
# Read application secrets
path "apps/data/svc-web-analyzer" {
  capabilities = ["read"]
}
EOF
    then
        abort "Failed to create application policy"
    fi
}

setup_approle_authentication() {
    log_info "Setting up AppRole authentication..."

    log_info "Enabling AppRole authentication..."
    if ! vault auth enable approle; then
        abort "Failed to enable AppRole authentication"
    fi

    log_info "Creating AppRole for web-analyzer app..."
    if ! vault write auth/approle/role/web-analyzer \
        token_policies="web-analyzer-policy" \
        token_ttl=1h \
        token_max_ttl=4h; then
        abort "Failed to create AppRole"
    fi
}

store_approle_credentials() {
    log_info "Storing AppRole credentials..."

    local role_id
    local secret_id

    role_id=$(vault read -field=role_id auth/approle/role/web-analyzer/role-id)
    if [ -z "${role_id}" ]; then
        abort "Failed to get role ID"
    fi

    secret_id=$(vault write -force -field=secret_id auth/approle/role/web-analyzer/secret-id)
    if [ -z "${secret_id}" ]; then
        abort "Failed to get secret ID"
    fi

    if ! vault kv put svc-web-analyzer/auth \
        role_id="${role_id}" \
        secret_id="${secret_id}"; then
        abort "Failed to store AppRole credentials"
    fi

    log_info "AppRole credentials stored successfully"
}

initialize_vault() {
    log_info "Configuring Vault from .envrc values..."

    enable_secret_engines
    store_application_secrets

    log_info "Vault initialization completed successfully!"
}

main() {
    trap 'exit ${exit_status}' EXIT

    load_configuration
    wait_for_vault
    setup_vault_authentication

    if is_vault_initialized; then
        exit 0
    fi

    initialize_vault
}

main "${@}"
