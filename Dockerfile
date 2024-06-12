FROM golang:latest as build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server .

FROM scratch
COPY --from=build /app/server .
ENTRYPOINT ["./server"]