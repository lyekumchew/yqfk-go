FROM golang:1.14-alpine AS builder

RUN apk --no-cache add build-base gcc

WORKDIR /src

COPY . .

RUN go mod download \
    && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o yqfk .


FROM alpine:latest

LABEL maintainer="AUTUMN"

WORKDIR /app

COPY --from=builder /src/yqfk /app/

RUN apk add --no-cache tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && rm -rf /var/cache/apk/*

ENV USERNAME=
ENV PASSWORD=
ENV SCKEY=

ENTRYPOINT ["/bin/sh", "-c", "exec ./yqfk -u ${USERNAME} -p ${PASSWORD} -k ${SCKEY}"]