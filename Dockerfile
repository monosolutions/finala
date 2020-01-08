FROM node:12-alpine as frontend_build
ENV NODE_ENV=production
COPY ./ui /app
WORKDIR /app
RUN npm install webpack webpack-cli -g
RUN npm install
RUN webpack --config config/webpack.config.production.js --color -p --progress --hide-modules --display-optimization-bailou

FROM golang:1.12-alpine AS build_base

RUN apk add --update alpine-sdk git make && \
	git config --global http.https://gopkg.in.followRedirects true \
	update-ca-certificates
RUN go get -u github.com/gobuffalo/packr/packr

WORKDIR /app

CMD CGO_ENABLED=0 go test ./...

COPY . .
COPY --from=frontend_build /app /app/ui
RUN packr
RUN go install -v ./...


FROM alpine:3.9 
RUN apk add ca-certificates xdg-utils
COPY config.yaml /root/config.yaml
COPY --from=build_base /go/bin/finala /bin/finala

CMD ["finala"]