ENV_FILE := .envrc
DOT_ENV := .env
CERTS_DIR := ".certs"

MOCKS_DIR := internal/mocks

.PHONY: $(ENV_FILE) $(DOT_ENV)
$(ENV_FILE) $(DOT_ENV):
	cat .envrc.dist | tee "$(ENV_FILE)" "$(DOT_ENV)" > /dev/null

$(CERTS_DIR):
	mkdir -p "${CERTS_DIR}"

.PHONY: set-hosts
set-hosts: ## Update local hosts.
	$(call printMessage,"🤖  Updating local hosts",$(INFO_CLR))
	echo "\n# Web Analyzer Hosts\n\
====================\n\
127.0.0.1 web-analyzer.dev api.web-analyzer.dev docs.web-analyzer.dev traefik.web-analyzer.dev vault.web-analyzer.dev rabbitmq.web-analyzer.dev" | sudo tee -a /etc/hosts

.PHONY: init
init: $(ENV_FILE) set-hosts certify
	go mod vendor
	$(MAKE) generate-api

.PHONY: start
start: ## 🐳 Start the Docker containers.
	$(call printMessage,"🏁  Starting containers",$(INFO_CLR))
	docker compose \
			--profile development \
			-f compose.yaml \
			-f compose-tools.yaml \
			up \
			-d \
    		--force-recreate

.PHONY: restart
restart: ## 🐳 Restart the Docker containers.
	$(call printMessage,"♻️  Restarting containers",$(INFO_CLR))
	docker compose \
			--profile development \
			-f compose.yaml \
			-f compose-tools.yaml \
			restart

.PHONY: destroy
destroy: ## 🐳 Destroy Docker containers.
	$(call printMessage,"💣  Destroying containers",$(INFO_CLR))
	docker compose \
			down --remove-orphans

.PHONY: study
study: $(CERTS_DIR) ## 👨‍🎓 Studying hard and preparing for certification.
	$(call printMessage,"📖  Studying for the certification",$(INFO_CLR))
ifeq (, $(shell which "mkcert"))
 $(error "Command mkcert not found in $$PATH, please install https://github.com/FiloSottile/mkcert#installation")
endif
	mkcert -install

.PHONY: certify
certify: study ## 📜 Certify .localhost and .dev TLDs.
	$(call printMessage,"📚  Preparing for the certification",$(INFO_CLR))
	mkcert -cert-file "${CERTS_DIR}/star-web-analyzer-dev.crt" \
		-key-file "${CERTS_DIR}/star-web-analyzer-dev.key" \
		"web-analyzer.dev" "*.web-analyzer.dev"
	cp "$$(mkcert -CAROOT)/rootCA.pem" "${CERTS_DIR}/"

.PHONY: generate-api
generate-api:
	$(call printMessage,"🤖  Generating API specs",$(INFO_CLR))
	docker run --rm -v $(PWD):/spec redocly/cli:2.7.0 bundle ./docs/openapi-spec/svc-web-analyzer-api.yaml -d --output ./docs/openapi-spec/public/svc-web-analyzer-swagger-v1.json --ext json --config .redocly.yaml && \
	cd internal/tools && go generate ./generate.go

$(MOCKS_DIR):
	$(call printMessage,"🎭  Generating mocks",$(INFO_CLR))
	GOFLAGS="-mod=mod" go generate ./internal/...

.PHONY: generate-mocks
generate-mocks: $(MOCKS_DIR) ## 🎭 Generate test mocks from interfaces (only if needed).

.PHONY: generate-mocks-force
generate-mocks-force: ## 🎭 Force regenerate test mocks from interfaces.
	$(call printMessage,"🎭  Force regenerating mocks",$(INFO_CLR))
	rm -rf "${MOCKS_DIR}"
	$(MAKE) generate-mocks

.PHONY: create-migration
create-migration: ## 🗂️ Creates migration files based on a passed argument "migration_name".
	$(call printMessage,"🗃️  Creating migration",$(INFO_CLR))
	docker compose run --name migrate-this --rm -it migrate create -ext sql -dir migrations "${migration_name}"

.PHONY: test
test: generate-mocks ## 🏃Run tests with race flag 🏁
	$(call printMessage,"🕸️  Running tests",$(INFO_CLR))
	GOFLAGS="-mod=mod" go test -v -race ./...
