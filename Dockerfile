FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /resource-service ./cmd/resource

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /resource-service /usr/local/bin/resource-service
COPY config/local.yml /app/config/local.yml

EXPOSE 60007 60008 60009

ENTRYPOINT ["resource-service"]
CMD ["-config", "/app/config/local.yml"]
