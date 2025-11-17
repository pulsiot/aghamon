FROM golang:1.25.4-bookworm AS builder
ENV GITHUB_ORG=pulsiot \
    GITHUB_REPO=aghamon \
    AGHAMON_CONFIG_FILE=config.yml

WORKDIR $GOPATH/src/${GITHUB_ORG}/${GITHUB_REPO}/
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o ${GITHUB_REPO}
RUN config.yaml.sample config.yaml
FROM scratch
COPY --from=builder $GOPATH/src/${GITHUB_ORG}/${GITHUB_REPO}/ /app/
WORKDIR /app
ENTRYPOINT ["/app/aghamon/aghamon"]
