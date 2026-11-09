# Build the isolated Coraza/CRS demo. This is not the final product image.
FROM golang:1.26.0 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/waf-demo ./cmd/waf-demo

# Distroless nonroot runtime minimizes the image and has no shell/package manager.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/waf-demo /waf-demo
USER nonroot:nonroot
EXPOSE 8080 8443
ENTRYPOINT ["/waf-demo"]
CMD ["-mode", "detectionOnly", "-http-listen", "0.0.0.0:8080", "-https-listen", "0.0.0.0:8443", "-allow-remote-listen"]
