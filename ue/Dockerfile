FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY proto ./proto
COPY ue ./ue
RUN cd ue && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ue .

FROM alpine:3.22
RUN adduser -D -u 10001 litecore
USER litecore
COPY --from=builder /out/ue /usr/local/bin/ue
ENTRYPOINT ["ue"]
