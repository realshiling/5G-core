FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY proto ./proto
COPY amf ./amf
RUN cd amf && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/amf .

FROM alpine:3.22
RUN adduser -D -u 10001 litecore
USER litecore
COPY --from=builder /out/amf /usr/local/bin/amf
EXPOSE 50051
ENTRYPOINT ["amf"]
