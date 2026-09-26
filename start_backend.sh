cp .env.example .env   # skip if already present
pnpm install --frozen-lockfile
docker compose up -d db

set -a
. ./.env
set +a

go run ./cmd/btg-lms migrate
go run ./cmd/btg-lms doctor
go run ./cmd/btg-lms serve
