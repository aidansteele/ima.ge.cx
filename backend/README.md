I had grand plans for cleaning up this codebase and publishing it a year ago.
Life got in the way and I've forgotten entirely how it works. As an act of personal
growth, I am publishing it here in its very rough and incomplete state, on the
slim chance that someone else might find something useful in it.

### Development and deployment

This directory is the `browseimage` Go module. Run Go and SAM commands from
`backend/`. The web frontend lives in [../frontend](../frontend/).

Initialize the gztool submodule from the repository root:

```shell
git submodule update --init backend/targzi/gztool
cd backend
sam build --parallel
```

`sam build` builds locally; `./build.sh` also deploys the
`imagecx-backend-beta` stack to AWS. Frontend deployment remains separate.

#### Beta HTTP API on Lambda Web Functions

`beta-api.ima.ge.cx` uses the separate `imagecx-api-web-beta` stack defined in
`web-function.yml`. Its Go server uses the existing beta table, bucket and
Step Functions `live` alias. The original Event Function remains available for
comparison and rollback; indexing workers and production are unchanged.

From this directory, deploy the beta API with:

```shell
AWS_PROFILE=imagecx ./deploy-web-beta.sh
```

The script requires Go, Docker/buildx, AWS CLI, jq and zip. It checks the AWS
account, builds ARM64 Go and static gztool binaries, and uploads a versioned zip.
On the first deployment it creates the code bucket before publishing a revision.
Later deployments replace the revision and update its endpoint through
CloudFormation. The code bucket is retained: do not delete versions referenced
by an active revision. The script does not deploy workers or change CDN routing.

The Node 24 entry point calls `process.execve()` to replace Node with Go. Go
listens on `AWS_LAMBDA_HTTP_ENDPOINT`, posts to the internal
`/2025-09-01/ready` API and drains requests for up to 900 ms on SIGTERM. This
readiness contract is not an officially supported Go runtime. Validate a real
request after deployment; an Active revision alone does not prove startup works.
`Function-Version` identifies the deployed zip's SHA-256. Request logs include
the platform request ID. X-Ray is disabled for this server because Web Functions
do not provide the Event Function daemon/context; application and platform logs
are retained for 14 days in `/aws/lambda/web/imagecx-api-web-beta[.local]`.

Concurrency starts at a ceiling of eight requests per environment because file
extraction buffers responses in memory. The gzip-index cache remains bounded to
512 MiB and serializes cache misses. CORS is handled by Go instead of Function
URLs. The server propagates a 90-second request deadline, including to gztool.

To switch beta's CDN origin or roll it back, update **only** the `Origin`
parameter on `imagecx-cdn-beta` using its deployed template. The Web Function
domain is the `Domain` output of `imagecx-api-web-beta`; the rollback origin is
the hostname of `TheLambdaUrl` from `imagecx-backend-beta`. Preserve the other
parameters and wait for both CloudFormation and CloudFront to finish. Validate
with a unique query parameter to avoid a cached response from the previous origin.
Do not redeploy the repository's older CDN template over the deployed one.

Do not run the full Go test suite against live credentials: it includes legacy
registry/S3 integration tests and a table-wipe test gated by `WIPE_TABLE=1`.
Local-only checks include `go test ./... -run '^$'` (compile all test packages),
`go test ./thelambda ./concatenator`, and
`go test ./targzi -run '^TestReadIndexInfo$'`.

### Hidden files API

`GET /api/dir` and `GET /api/file` accept `hidden=true` alongside the existing
`image`, `digest`, and `path` parameters. The default remains the final merged
filesystem. Hidden mode uses a separate `hidden-index.json.gz`: only paths absent
from the final filesystem, with the latest stored entry per path (not version
history or whiteout markers). File extraction uses the original layer offsets.
Directory listings retain their existing shape; navigation directories are added
as needed and sizes/counts include only hidden regular files, not live files.
Empty directory-only branches are omitted.

Missing indexes return HTTP 404. Directory/file requests never trigger indexing;
`/api/info` starts indexing when the image's DynamoDB record is absent.

### Layer indexing

The lister returns compressed sizes from the manifest without downloading layer
blobs. Step Functions routes layers below 64 MiB to the 3,538 MB ARM64 reader
(gztool), and layers at or above 64 MiB to a 6,144 MB reader using four rapidgzip
workers. Both use one HTTP stream and the same cache and index format. The
concatenator retains 3,538 MB; the other functions use the 1,769 MB default.
Go parses buffered tar headers without counting newlines or spooling the layer
to disk. The original compressed size and digest are verified before publishing
an index. Cached large layers still use the large reader but skip decompression.

Completed layer indexes are reused by digest, after checking the source image's
manifest. `layers/index-v2/<digest>/complete.json` points to an immutable pair of
tar metadata and gzip checkpoints. Concurrent builds cannot overwrite another
image's checkpoints. `LAYER_CACHE=false` bypasses both cache reads and publication
for uncached measurements. Readers require explicit immutable index references;
the old `layers/<digest>/` layout is no longer supported. Retiring an old image
requires removing both its S3 listings and DynamoDB image/progress records so
`/api/info` can regenerate it on demand. Preserve immutable pairs referenced by
current images.

The HTTP Lambda caches gzip indexes in `/tmp/imagecx-gzi-cache` across warm
invocations, evicting least-recently-used files before a download would exceed
512 MiB. Files in use cannot be evicted; incomplete downloads are discarded.
Cache hits avoid S3 entirely. The cache is local to each execution environment
and is cleared when the process starts, including after a runtime reset that
preserves `/tmp`.

The concatenator reads up to four indexes concurrently, merges newest layers
first, and applies their whiteouts only to older layers. It still publishes the
same final and hidden views; no changes to the directory/file API are required.

`sam build --parallel` now requires Docker/buildx and an initialized gztool
submodule (`git submodule update --init targzi/gztool`). The layer Dockerfile
builds pinned gztool and rapidgzip sources for ARM64 Linux, including license
notices. rapidgzip is statically linked with an explicit 8 MiB worker stack.
Local decoder round-trip tests require these Linux binaries on `PATH`;
run them in an ARM64 Linux container, not directly on macOS.
