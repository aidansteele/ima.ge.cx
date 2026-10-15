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
