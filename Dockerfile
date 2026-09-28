# The client: TypeScript compiled into client/static
FROM node:24-slim AS frontend
WORKDIR /nytrpg
COPY package.json package-lock.json ./
RUN npm ci
COPY tsconfig.json ./
COPY client ./client
RUN npx tsc -p .

# The server. go-sqlite3 needs cgo, so the binary links against glibc
FROM golang:1.27-trixie AS backend
WORKDIR /nytrpg
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app ./cmd/server

# Same Debian as the build stage so glibc matches. ca-certificates is for
# HTTPS calls to the Claude API (Cleric prayers)
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /nytrpg
COPY --from=backend /app ./app
COPY --from=frontend /nytrpg/client/static ./client/static
RUN mkdir -p db
EXPOSE 8080
CMD ["./app"]
