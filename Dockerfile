FROM alpine:3.24.1

RUN apk --no-cache --no-progress add git ca-certificates tzdata make \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

COPY ./dist/linux/amd64/plugin-service .

USER 65534

ENTRYPOINT ["/plugin-service"]
EXPOSE 80
