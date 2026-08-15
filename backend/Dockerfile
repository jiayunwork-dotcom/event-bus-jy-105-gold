FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . .
RUN GOPROXY=https://goproxy.cn,direct go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /eventbus-server ./main.go

FROM alpine:3.18

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /eventbus-server /app/eventbus-server

EXPOSE 8080

CMD ["/app/eventbus-server"]
