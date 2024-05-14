# syntax=docker/dockerfile:1
# golang image uses debian as base
FROM golang:latest AS builder

# Add timezone data and certs
RUN apt-get -y install tzdata ca-certificates

WORKDIR /src

COPY . .

RUN go mod download

# Ensure statically built
RUN CGO_ENABLED=0 go build  -ldflags '-s' -o /bin/meterman

FROM scratch

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Certs are required for https posts.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /bin/meterman /bin/meterman

CMD ["/bin/meterman", "--config=/config", "--port=8080"]
