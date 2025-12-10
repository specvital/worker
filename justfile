set dotenv-load := true

root_dir := justfile_directory()

bootstrap: install-psql install-sqlc

deps: deps-root

deps-root:
    pnpm install

dump-schema:
    PGPASSWORD=postgres pg_dump -h specvital-postgres -U postgres -d specvital --schema-only --no-owner --no-privileges -n public | grep -v '^\\\|^SET \|^SELECT ' > src/internal/db/schema.sql

gen-sqlc:
    cd src && sqlc generate

install-psql:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! command -v psql &> /dev/null; then
      DEBIAN_FRONTEND=noninteractive apt-get update && \
        apt-get -y install lsb-release wget && \
        wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - && \
        echo "deb http://apt.postgresql.org/pub/repos/apt/ $(lsb_release -cs)-pgdg main" | tee /etc/apt/sources.list.d/pgdg.list && \
        apt-get update && \
        apt-get -y install postgresql-client-16
    fi

install-sqlc:
    go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0

lint target="all":
    #!/usr/bin/env bash
    set -euox pipefail
    case "{{ target }}" in
      all)
        just lint justfile
        just lint config
        just lint go
        ;;
      justfile)
        just --fmt --unstable
        ;;
      config)
        npx prettier --write "**/*.{json,yml,yaml,md}"
        ;;
      go)
        gofmt -w src
        ;;
      *)
        echo "Unknown target: {{ target }}"
        exit 1
        ;;
    esac

build:
    cd src && go build ./...

run:
    cd src && air

test target="all":
    #!/usr/bin/env bash
    set -euo pipefail
    cd src
    case "{{ target }}" in
      all)
        TESTCONTAINERS_RYUK_DISABLED=true go test -v ./...
        ;;
      unit)
        go test -v -short ./...
        ;;
      integration)
        TESTCONTAINERS_RYUK_DISABLED=true go test -v -run 'Integration|Repository' ./...
        ;;
      *)
        echo "Unknown target: {{ target }}. Use: unit, integration, all"
        exit 1
        ;;
    esac

tidy:
    cd src && go mod tidy

update-core:
    cd src && GOPROXY=direct go get -u github.com/specvital/core@main && go mod tidy
