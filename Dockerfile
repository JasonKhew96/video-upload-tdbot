FROM golang:1.16.4-alpine AS golang

COPY --from=jasonkhew96/tdlib:1.7.4-alpine /usr/local/lib/libtd* /usr/local/lib/
COPY --from=jasonkhew96/tdlib:1.7.4-alpine /usr/local/include/td /usr/local/include/td
COPY --from=jasonkhew96/tdlib:1.7.4-alpine /usr/lib/libssl.so /usr/lib/libssl.so
COPY --from=jasonkhew96/tdlib:1.7.4-alpine /usr/lib/libcrypto.so /usr/lib/libcrypto.so
COPY --from=jasonkhew96/tdlib:1.7.4-alpine /lib/libz.so /usr/local/lib/libz.so
RUN apk add build-base

WORKDIR /runner

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -o /tmp/output-binary main.go

FROM alpine:3.13.2
WORKDIR /runner/
RUN apk add ffmpeg
COPY --from=golang /tmp/output-binary /runner/tdbot
CMD [ "/runner/tdbot" ]