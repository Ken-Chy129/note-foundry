FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/notefoundry-api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/notefoundry-migrate ./cmd/migrate \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/notefoundry-worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/notefoundry-restore ./cmd/restore \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/notefoundry-publish-public ./cmd/publishpublic

FROM postgres:17-alpine

RUN addgroup -S notefoundry && adduser -S -G notefoundry notefoundry
COPY --from=build /out/notefoundry-* /usr/local/bin/
RUN mkdir -p /var/lib/notefoundry/attachments && chown -R notefoundry:notefoundry /var/lib/notefoundry
ENTRYPOINT []
USER notefoundry
WORKDIR /var/lib/notefoundry
EXPOSE 8080
CMD ["/usr/local/bin/notefoundry-api"]
