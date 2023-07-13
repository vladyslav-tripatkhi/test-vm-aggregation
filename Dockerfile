FROM golang:1.20 AS builder

WORKDIR /opt/app

COPY src ./

RUN CGO_ENABLED=0 GOOS=linux go build -v -a -installsuffix cgo -o metrics .

FROM alpine:3.18

WORKDIR /opt/app
COPY --from=builder /opt/app/metrics /opt/app/default_config.yml ./

ENTRYPOINT ["./metrics"]