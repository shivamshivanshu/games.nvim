FROM golang:1.26 AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /games-server ./cmd/games-server \
    && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /games-server /games-server
COPY --from=build --chown=65532:65532 /data /data
VOLUME /data
EXPOSE 7777
ENTRYPOINT ["/games-server"]
CMD ["serve", "--addr", "0.0.0.0:7777", "--db", "/data/games.db"]
