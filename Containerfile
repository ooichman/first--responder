# Stage 1: Build binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /first-responder main.go

# Stage 2: Runtime image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /first-responder .
COPY companydocs/ ./companydocs/
EXPOSE 8080
CMD ["./first-responder"]
