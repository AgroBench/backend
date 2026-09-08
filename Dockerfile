# Imagem de produção: binário estático em scratch. Migrations vão embutidas no binário
# (`agrobench migrate up`), então não precisa de CLI externa.
FROM alpine:3.21 AS certs
RUN apk --no-cache add tzdata ca-certificates

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agrobench ./cmd

FROM scratch
COPY --from=certs /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/agrobench /api/agrobench
COPY config/env/config.dev.json /api/config/env/config.dev.json
WORKDIR /api
EXPOSE 8080
ENTRYPOINT ["./agrobench"]
CMD ["serve"]
