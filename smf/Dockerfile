FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.work go.work.sum ./
COPY proto ./proto
COPY smf ./smf
RUN cd smf && GOWORK=off CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/smf .

FROM alpine:3.22
RUN adduser -D -u 10001 litecore
USER litecore
COPY --from=builder /out/smf /usr/local/bin/smf
EXPOSE 50052
ENTRYPOINT ["smf"]
