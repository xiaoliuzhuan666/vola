ARG NODE_BASE_IMAGE=node:20-alpine
ARG GO_BASE_IMAGE=golang:1.25-alpine
ARG RUNTIME_BASE_IMAGE=alpine:3.19

# ---- Node stage: build the React frontend ----
FROM ${NODE_BASE_IMAGE} AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ .
RUN npm run build

# ---- Go stage: build the backend with embedded frontend ----
FROM ${GO_BASE_IMAGE} AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy the built frontend into the embed directory
COPY --from=frontend /app/web/dist/ ./internal/web/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -o /vola ./cmd/vola

# ---- Final image: just the binary + migrations ----
FROM ${RUNTIME_BASE_IMAGE}
RUN apk add --no-cache ca-certificates git tzdata
WORKDIR /app
COPY --from=builder /vola .
COPY --from=builder /vola ./vola
COPY migrations/ ./migrations/
EXPOSE 8080
CMD ["./vola", "server", "--listen", ":8080"]
