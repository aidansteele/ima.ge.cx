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

FROM alpine:3.23 AS rapidgzip
RUN apk add --no-cache build-base linux-headers git cmake ninja
# Pin the tested rapidgzip release and all recursive submodules. Static musl
# builds need an explicit worker stack size for parallel decompression.
RUN git init /src/rapidgzip \
    && git -C /src/rapidgzip remote add origin https://github.com/mxmlnkn/rapidgzip.git \
    && git -C /src/rapidgzip fetch --depth 1 origin d2350e9c9ba54398cd64e45bfc8c631beec017f0 \
    && git -C /src/rapidgzip checkout --detach FETCH_HEAD \
    && git -C /src/rapidgzip submodule update --init --recursive --depth 1 \
    && test "$(git -C /src/rapidgzip/librapidarchive rev-parse HEAD)" = 1221a30bb548b305a69e5715f2bc348ba37ac243
RUN cmake -S /src/rapidgzip/librapidarchive -B /build -G Ninja \
      -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_EXE_LINKER_FLAGS="-static-pie -Wl,-z,stack-size=8388608" \
      -DLIBRAPIDARCHIVE_BUILD_TESTS=OFF \
      -DLIBRAPIDARCHIVE_BUILD_BENCHMARKS=OFF \
      -DLIBRAPIDARCHIVE_WITH_ISAL=ON \
    && cmake --build /build --target rapidgzip -j2 \
    && mkdir -p /out/bin /out/licenses/rapidgzip \
    && strip -o /out/bin/rapidgzip /build/src/tools/rapidgzip \
    && cd /src/rapidgzip/librapidarchive \
    && find . -type f \( -name 'LICENSE*' -o -name 'COPYING*' -o -name UNLICENSE \) \
      -exec cp --parents '{}' /out/licenses/rapidgzip/ \;

FROM scratch
COPY --from=build /out/ /
COPY --from=rapidgzip /out/ /
