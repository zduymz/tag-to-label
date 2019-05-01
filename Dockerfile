FROM alpine:3.9
LABEL maintainer="duymai"

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
COPY ./build/linux/tag-to-label /bin/tag-to-label

USER nobody

ENTRYPOINT ["/bin/tag-to-label"]

