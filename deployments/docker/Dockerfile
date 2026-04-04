# Build frontend
FROM node:22-alpine AS frontend

WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build Go binary
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/web/static /app/web/static
RUN CGO_ENABLED=1 GOOS=linux go build -o /skillhub ./cmd/skillhub

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY --from=builder /skillhub /app/skillhub
COPY configs/ /app/configs/

RUN mkdir -p /app/data/repos

EXPOSE 10070

ENTRYPOINT ["/app/skillhub"]
CMD ["serve"]
