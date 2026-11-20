FROM golang:1.27.3-alpine3.24 AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./... && go test ./... && \
    CGO_ENABLED=0 go build -trimpath -o /out/api . && \
    CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -o /out/set-admin-pwd ./cmd/set-admin-pwd

FROM alpine:3.24
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/api ./api
COPY --from=build /out/migrate ./migrate
COPY --from=build /out/set-admin-pwd ./set-admin-pwd
USER app
EXPOSE 8080
CMD ["./api"]

