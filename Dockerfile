FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app ./cmd/app/main.go

FROM alpine:3.20

RUN apk --no-cache add tzdata

WORKDIR /app

COPY --from=builder /app/app .

EXPOSE 8080

CMD ["./app"]