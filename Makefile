include .envrc

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
# MAKEFILE_LIST 是一个特殊变量，它包含 make 正在解析的 makefile 的名称。
# 把 help target 放在 Makefile 的第一条是有意为之。如果运行 make 而不指定目标，它将默认执行文件中的第一条规则
.PHONY: help
help:
	@echo "Usage"
	@sed -n "s/^##//p" ${MAKEFILE_LIST} | column -t -s ":" | sed -e "s/^/ /"
# Create the new confirm target.
# https://stackoverflow.com/a/47839479
.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api -db-dsn=${GREENLIGHT_DB_DSN}
## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	psql ${GREENLIGHT_DB_DSN}
## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo "Creating migrations for ${name}"
	migrate create -seq -ext=.sql -dir=./migrations ${name}
# 注意到我们是如何在 up 规则中使用 @ 字符来防止 echo 命令在运行时自动执行的吗
## db/migrations/up: apply all up database migrations
.PHONY: db/migrations/up
db/migrations/up: confirm
	@echo "Running up migrations"
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## audit: tidy dependencies and format, vet and test all code
.PHONY: audit
audit: vendor
	@echo 'Formatting code...'
	go fmt ./...
	@echo 'Vetting code...'
	go vet ./...
	staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

## vendor: tidy and vendor dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify
	@echo 'Vendoring dependencies...'
	go mod vendor

# ==================================================================================== #
# BUILD
# ==================================================================================== #

## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s' -o ./bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=./bin/linux_amd64/api ./cmd/api