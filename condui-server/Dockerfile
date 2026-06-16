FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o condui-server .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/condui-server .
RUN mkdir -p /data && chmod 700 /data
EXPOSE 8080
VOLUME ["/data"]
ENV DB_PATH=/data/condui.db
CMD ["./condui-server"]
