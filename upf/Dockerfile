FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY proto ./proto
COPY upf ./upf
RUN cd upf && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/upf .

FROM alpine:3.22
RUN adduser -D -u 10001 litecore
USER litecore
COPY --from=builder /out/upf /usr/local/bin/upf
EXPOSE 50053
ENTRYPOINT ["upf"]
