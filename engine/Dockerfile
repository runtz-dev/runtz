FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG LDFLAGS=""

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-} \
    go build -ldflags "${LDFLAGS}" -o /out/runtz ./cmd/server

FROM alpine:3.22

RUN adduser -D -g "" runtz
USER runtz

COPY --from=build /out/runtz /usr/local/bin/runtz

EXPOSE 8080
CMD ["runtz"]
