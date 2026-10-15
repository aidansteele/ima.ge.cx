FROM alpine:3.23 AS build
RUN apk add --no-cache build-base zlib-dev zlib-static
# Build the pinned submodule as a static binary, independent of the host OS
# and of newer glibc versions than the Lambda provided.al2 runtime supports.
COPY gztool/gztool.c /src/gztool.c
RUN mkdir -p /out/bin /out/licenses \
    && cc -O3 -static -no-pie -o /out/bin/gztool /src/gztool.c -lz -lm \
    && cp /src/gztool.c /out/licenses/gztool.c \
    && cp /usr/include/zlib.h /out/licenses/zlib.h \
    && strip /out/bin/*

FROM scratch
COPY --from=build /out/ /
