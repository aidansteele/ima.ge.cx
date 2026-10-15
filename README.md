I had grand plans for cleaning up this codebase and publishing it a year ago.
Life got in the way and I've forgotten entirely how it works. As an act of personal
growth, I am publishing it here in its very rough and incomplete state, on the
slim chance that someone else might find something useful in it.

### Hidden files API

`GET /api/dir` and `GET /api/file` accept `hidden=true` alongside the existing
`image`, `digest`, and `path` parameters. The default remains the final merged
filesystem. Hidden mode uses a separate `hidden-index.json.gz`: only paths absent
from the final filesystem, with the latest stored entry per path (not version
history or whiteout markers). File extraction uses the original layer offsets.
Directory listings retain their existing shape; navigation directories are added
as needed and sizes/counts include only hidden regular files, not live files.
Empty directory-only branches are omitted.

Older indexes without this object return HTTP 409 with an explanation that the
image must be reindexed. Requests never trigger reindexing or migration.

### Layer indexing

The reader and concatenator use 3,538 MB ARM64 Lambdas (approximately two vCPUs);
the other functions retain the 1,769 MB default. gztool decodes the gzip stream
while Go parses buffered tar headers, without counting newlines or spooling the
layer to disk. The original compressed size and digest are verified before
publishing an index.

Completed layer indexes are reused by digest, after checking the source image's
manifest. `layers/index-v2/<digest>/complete.json` points to an immutable pair of
tar metadata and gzip checkpoints. Concurrent builds cannot overwrite another
image's checkpoints. `LAYER_CACHE=false` bypasses both cache reads and publication
for uncached measurements. Older indexes remain readable but are not reused by
this cache; do not remove their objects or immutable pairs referenced by images.

The concatenator reads up to four indexes concurrently, merges newest layers
first, and applies their whiteouts only to older layers. It still publishes the
same final and hidden views; no changes to the directory/file API are required.

`sam build --parallel` now requires Docker/buildx and an initialized gztool
submodule (`git submodule update --init targzi/gztool`). The layer Dockerfile
builds the pinned gztool source for ARM64 Linux, including its license notice.
Local decoder round-trip tests require this Linux binary on `PATH`;
run them in an ARM64 Linux container, not directly on macOS.
