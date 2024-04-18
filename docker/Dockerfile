# syntax=docker/dockerfile:1
FROM golang:1.20-alpine AS builder

# Add timezone data
RUN apk add --no-cache tzdata

WORKDIR /src

COPY . .

RUN go mod download

RUN go build  -ldflags '-s' -o /bin/meterman

FROM scratch

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Certs are required for https posts.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /bin/meterman /bin/meterman

CMD ["/bin/meterman", "--config=/config", "--checkpoint=/data/checkpoint", "--port=8080"]
