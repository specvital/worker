set dotenv-load := true

root_dir := justfile_directory()

bootstrap: install-docker install-psql install-sqlc

clean-containers:
    docker ps -a --filter "label=org.testcontainers=true" -q | xargs -r docker rm -f

deps: deps-root

deps-root:
    pnpm install

dump-schema:
    PGPASSWORD=postgres pg_dump -h specvital-postgres -U postgres -d specvital --schema-only --no-owner --no-privileges -n public | grep -v '^\\\|^SET \|^SELECT ' > src/internal/infra/db/schema.sql
    cp src/internal/infra/db/schema.sql src/internal/testutil/postgres/schema.sql

enqueue mode="local" *args:
    #!/usr/bin/env bash
    set -euo pipefail
    cd src
    case "{{ mode }}" in
      local)
        DATABASE_URL="$LOCAL_DATABASE_URL" go run ./cmd/enqueue {{ args }}
        ;;
      integration)
        go run ./cmd/enqueue {{ args }}
        ;;
      *)
        echo "Unknown mode: {{ mode }}. Use: local, integration"
        exit 1
        ;;
    esac

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

install-docker:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! command -v docker &> /dev/null; then
      curl -fsSL https://get.docker.com | sh
    fi

install-railway:
    npm install -g @railway/cli

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
        npx prettier --write --cache "**/*.{json,yml,yaml,md}"
        ;;
      go)
        gofmt -w src
        ;;
      *)
        echo "Unknown target: {{ target }}"
        exit 1
        ;;
    esac

lint-file file:
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{ file }}" in
      */justfile|*Justfile)
        just --fmt --unstable
        ;;
      *.json|*.yml|*.yaml|*.md)
        npx prettier --write --cache "{{ file }}"
        ;;
      *.go)
        gofmt -w "{{ file }}"
        go vet "{{ file }}" 2>/dev/null || true
        ;;
      *)
        echo "No lint rule for: {{ file }}"
        ;;
    esac

# Local db migration always initializes the database
migrate-local:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Resetting local database..."
    PGPASSWORD=postgres psql -h local-postgres -U postgres -c "DROP DATABASE IF EXISTS specvital;"
    PGPASSWORD=postgres psql -h local-postgres -U postgres -c "CREATE DATABASE specvital;"
    echo "Applying schema..."
    PGPASSWORD=postgres psql -h local-postgres -U postgres -d specvital -f src/internal/infra/db/schema.sql
    echo "✅ Migration complete!"

build target="all":
    #!/usr/bin/env bash
    set -euo pipefail
    cd src
    case "{{ target }}" in
      all)
        go build -o ../bin/analyzer ./cmd/analyzer
        go build -o ../bin/spec-generator ./cmd/spec-generator
        go build -o ../bin/retention-cleanup ./cmd/retention-cleanup
        go build -o ../bin/enqueue ./cmd/enqueue
        echo "Built: bin/analyzer, bin/spec-generator, bin/retention-cleanup, bin/enqueue"
        ;;
      analyzer)
        go build -o ../bin/analyzer ./cmd/analyzer
        ;;
      spec-generator)
        go build -o ../bin/spec-generator ./cmd/spec-generator
        ;;
      retention-cleanup)
        go build -o ../bin/retention-cleanup ./cmd/retention-cleanup
        ;;
      enqueue)
        go build -o ../bin/enqueue ./cmd/enqueue
        ;;
      check)
        go build ./...
        ;;
      *)
        echo "Unknown target: {{ target }}. Use: all, analyzer, spec-generator, retention-cleanup, enqueue, check"
        exit 1
        ;;
    esac

release:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "⚠️  WARNING: This will trigger a production release!"
    echo ""
    echo "GitHub Actions will automatically:"
    echo "  - Analyze commits to determine version bump"
    echo "  - Generate release notes"
    echo "  - Create tag and GitHub release"
    echo "  - Update CHANGELOG.md"
    echo ""
    echo "Progress: https://github.com/specvital/worker/actions"
    echo ""
    read -p "Type 'yes' to continue: " confirm
    if [ "$confirm" != "yes" ]; then
        echo "Aborted."
        exit 1
    fi
    git checkout release
    git merge main
    git push origin release
    git checkout main
    echo "✅ Release triggered! Check GitHub Actions for progress."

run-analyzer mode="local":
    #!/usr/bin/env bash
    set -euo pipefail
    cd src
    case "{{ mode }}" in
      local)
        DATABASE_URL="$LOCAL_DATABASE_URL" air
        ;;
      integration)
        air
        ;;
      *)
        echo "Unknown mode: {{ mode }}. Use: local, integration"
        exit 1
        ;;
    esac

run-spec-generator mode="local":
    #!/usr/bin/env bash
    set -euo pipefail
    cd src
    case "{{ mode }}" in
      local)
        DATABASE_URL="$LOCAL_DATABASE_URL" air -c .air.spec-generator.toml
        ;;
      integration)
        air -c .air.spec-generator.toml
        ;;
      *)
        echo "Unknown mode: {{ mode }}. Use: local, integration"
        exit 1
        ;;
    esac

run-retention-cleanup mode="local":
    #!/usr/bin/env bash
    set -euo pipefail
    cd src
    case "{{ mode }}" in
      local)
        DATABASE_URL="$LOCAL_DATABASE_URL" go run ./cmd/retention-cleanup
        ;;
      integration)
        go run ./cmd/retention-cleanup
        ;;
      *)
        echo "Unknown mode: {{ mode }}. Use: local, integration"
        exit 1
        ;;
    esac

sync-docs:
    baedal specvital/specvital.github.io/docs docs --exclude ".vitepress/**"

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

docker-build service="analyzer":
    docker build -f infra/{{ service }}/Dockerfile -t specvital-{{ service }}:local .
