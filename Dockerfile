FROM node:24-alpine AS frontend
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend ./frontend
COPY api ./api
COPY tsconfig.json vite.config.ts ./
RUN pnpm build

FROM golang:1.25-alpine AS backend
WORKDIR /src
ARG VERSION=dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=${VERSION}" -o /btg-lms ./cmd/btg-lms

FROM alpine:3.22
RUN addgroup -S btg && adduser -S -G btg btg
COPY --from=backend /btg-lms /usr/local/bin/btg-lms
USER btg
EXPOSE 8080
ENTRYPOINT ["btg-lms"]
CMD ["serve"]
