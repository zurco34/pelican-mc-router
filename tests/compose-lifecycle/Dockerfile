FROM golang:1.26-alpine AS build
WORKDIR /src
COPY fake-pelican.go .
RUN CGO_ENABLED=0 go build -trimpath -o /fake-pelican fake-pelican.go

FROM scratch
COPY --from=build /fake-pelican /fake-pelican
USER 10001:10001
ENTRYPOINT ["/fake-pelican"]
