# Stage 1: compile the Go program.
FROM golang:1.27 AS build
WORKDIR /src
# Download dependencies first so Docker can cache them between code changes.
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
# Static binary (no C libraries), so it runs on the minimal image below.
RUN CGO_ENABLED=0 go build -o /demo .

# Stage 2: a tiny image with only the binary (no shell or package manager).
FROM gcr.io/distroless/static-debian12
COPY --from=build /demo /demo
ENTRYPOINT ["/demo"]
