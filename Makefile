# Create the new confirm target.
# https://stackoverflow.com/a/47839479
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]
run/api:
	go run ./cmd/api
db/psql:
	psql ${GREENLIGHT_DB_DSN}
db/migrations/new:
	@echo "Creating migrations for ${name}"
	migrate create -seq -ext=.sql -dir=./migrations ${name}
# 注意到我们是如何在 up 规则中使用 @ 字符来防止 echo 命令在运行时自动执行的吗
db/migrations/up: confirm
	@echo "Running up migrations"
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up