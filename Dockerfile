# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o main .

# Stage 2: Runtime
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata curl
ENV TZ=Asia/Jakarta
COPY --from=builder /app/main .

EXPOSE 8090
CMD ["./main"]
