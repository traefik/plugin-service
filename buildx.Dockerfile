FROM alpine

RUN apk --no-cache --no-progress add git ca-certificates tzdata make \
    && update-ca-certificates \
    && rm -rf /var/cache/apk/*

ARG TARGETPLATFORM
COPY ./dist/$TARGETPLATFORM/plugin-service .

ENTRYPOINT ["/plugin-service"]
EXPOSE 80
