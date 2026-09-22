# ---------- Stage 1: Build ----------
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w" -o /app/bin/api ./cmd/api

# ---------- Stage 2: Runtime ----------
FROM alpine:3.20

WORKDIR /app

COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:1.0.1 /lambda-adapter /opt/extensions/lambda-adapter

COPY --from=builder /app/bin/api /app/api

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/api"]