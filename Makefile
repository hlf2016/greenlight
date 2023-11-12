run/api:
	go run ./cmd/api
db/psql:
	psql ${GREENLIGHT_DB_DSN}
db/migrations/new:
	@echo "Creating migrations for ${name}"
	migrate create -seq -ext=.sql -dir=./migrations ${name}
db/migrations/up:
	# 注意到我们是如何在 up 规则中使用 @ 字符来防止 echo 命令在运行时自动执行的吗
	@echo "Running up migrations"
	migrate -path ./migrations -database ${GREENLIGHT_DB_DSN} up