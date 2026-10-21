# [ima.ge.cx](https://ima.ge.cx)

Browse the files, layers and configuration of public container images without
pulling them to your machine or running Docker.

This directory contains the web frontend, built with Next.js 13, React 18,
Mantine 5, SWR and CodeMirror 6. Image indexing and file extraction live in the
[backend directory](../backend/).

## Using the site

Enter an image reference at [ima.ge.cx](https://ima.ge.cx), or put it directly in
the URL, such as [ima.ge.cx/ubuntu:24.04](https://ima.ge.cx/ubuntu:24.04).
Choose a platform when an image has multiple variants. Images are indexed on
first access; the page shows progress while indexing runs.

- **Files:** expand directories, inspect file contents and permissions, follow
  symbolic links, or download files. Text files support syntax highlighting,
  folding and search with ⌘F / Ctrl+F inside the viewer. Automatic previews are
  limited to 1 MiB using indexed sizes before fetching. Binary files show a
  download prompt instead of garbled text. Downloads preserve the original bytes
  and support files up to 4 MiB, independently of the preview limit. This
  conservative limit leaves room for base64 and JSON in the API's buffered
  Lambda response; larger files must be extracted from the image locally.
  Both limits are also enforced while reading responses, including when size
  metadata is incorrect. Oversized responses fail rather than being truncated.
  Download failures appear in the viewer; click **Download** to retry.
- **Hidden files:** switch on **Only show hidden files** above the tree to inspect
  files deleted from the final image but still stored in its layers. This is not
  a dotfile filter or version history: paths that still exist are excluded, and
  each absent path shows its latest stored version. Tree sizes count only hidden
  files in this view. The switch clears the selection; copied links preserve
  hidden mode with `?hidden=true`.
- **Directory sizes:** see uncompressed sizes and file counts, with bars showing
  each entry's share of its parent directory. Totals describe regular files in
  the final filesystem after layer deletions; symlinks and hard links do not
  contribute additional bytes or files. These are file sizes, not disk usage.
- **Layers:** inspect build instructions, layer digests and compressed sizes.
- **Environment and config:** filter environment variables or inspect the image
  configuration as JSON.
- **Tags and platforms:** switch using the selectors above the image.

File selections are shareable URL fragments:
`/<image>@<digest>#/path/to/file`. Use **Copy link** to share the exact image
digest and selected file. Click a line number to include it in the address bar
without moving the scroll position, then use **Copy link** to share
`/<image>@<digest>#/path/to/file:L42`.
Line numbers start at 1. Opening a line link scrolls to and highlights that line,
including inside a collapsed block; links beyond the end reveal the final line.
Invalid line numbers are ignored. Selecting files or lines replaces the current
history entry rather than adding one. Colons in file names are URL-encoded so
they cannot be confused with the line suffix.

Shared links include server-rendered Open Graph and Twitter metadata and a
1200×630 sharing image. Indexed digest links get a dynamic filesystem card with
the platform, regular-file count, uncompressed size and three largest root
directories, plus compressed size and layer count. Long references wrap and
truncate with an ellipsis. Tag-only links and unavailable indexes get an
identity card without filesystem statistics; no platform is chosen implicitly.
Canonical URLs omit query strings and file fragments (social crawlers cannot
see fragments). The homepage retains its static sharing image.

Pages also advertise `/api/oembed?url=…&format=json`, returning an oEmbed `photo`
with the same sharing image and no optional title, author or provider text.
Slack prefers this over OG/Twitter metadata for an image-only preview; other
clients can continue using the existing tags. Explicit `?theme=light` or
`?theme=dark` links retain their card palette. The endpoint accepts only
`https://ima.ge.cx` page URLs and never fetches them or starts indexing.
Slack caches unfurls, so test changes with a new URL (for example, add
`&preview=2` to a URL that already has a query string), not an existing message.

On desktop, the file tree and viewer sit side by side. On phones, switch between
**Browse files** and **View file**. When reading a file, scroll the page to move
the metadata out of the way; long code lines can scroll horizontally.

The service supports public container images with gzip-compressed layers, not
arbitrary OCI artifacts or every layer compression format.

## Local development

Use Node.js 20 or later and npm. The Amplify build currently uses Node.js 20.
Run the commands in this document from `frontend/`.

```shell
npm ci
npm run dev
```

Open `http://localhost:3000`. Docker and AWS credentials are not required to run
the frontend against the hosted API.

### API configuration

By default, the frontend calls `https://api.ima.ge.cx/api/`. Local browsing
therefore uses the production backend and can trigger indexing of images that
have not been indexed yet.

To use a different backend, set its API base URL in `.env.local`:

```dotenv
NEXT_PUBLIC_API_BASE_URL=https://your-backend.example/api/
```

Include the trailing slash. This is a browser-visible, build-time setting, not a
secret. Restart the development server after changing it, or rebuild for
production. The backend must allow requests from the frontend's origin through
CORS. See the [backend README](../backend/README.md) for its own setup and
deployment instructions.

### Checks and production build

```shell
npm run lint
npx tsc --noEmit
npm test
npm run build
npm start
```

The production build also runs lint and type checks. `npm test` checks social
card data, reference handling, PNG generation and cache/fallback behavior using
local fixtures, without contacting the production API. For UI changes, check
both desktop and phone widths, including file deep links, switching between the
tree and viewer, and scrolling long files past the metadata.

## Code layout

- `pages/index.tsx`: image search, tag suggestions and examples.
- `pages/[...slug].tsx`: image routes, indexing states and workspace tabs.
- `pages/api/og.tsx`: Node.js PNG endpoint, using `@vercel/og` and bundled Inter
  fonts (Google Fonts; SIL Open Font License in `assets/Inter-LICENSE.txt`).
- `components/imageSocialCard.tsx`: fixed-size social card layout.
- `src/imageMetadata.ts`: read-only metadata and root-directory summaries.
- `components/`: file tree, file viewer, CodeMirror editor, layer history and
  other UI components.
- `src/imageHooks.ts`: API requests, response types and SWR caching.
- `src/useHashPath.ts`: file selection and shareable URL fragments.
- `styles/globals.css`: shared styling and responsive layouts.

## Deployment

The frontend is hosted on AWS Amplify. Pushing to `main` triggers a production
build and deployment to [ima.ge.cx](https://ima.ge.cx).
[amplify.yml](../amplify.yml) installs dependencies with `npm ci`, runs
`npm run build` and publishes `.next`, all relative to `frontend/`.

Before deploying the monorepo layout, set `AMPLIFY_MONOREPO_APP_ROOT=frontend`
in the existing Amplify app's environment variables. This must match the
`appRoot` in `amplify.yml`.

Set `NEXT_PUBLIC_API_BASE_URL` in the Amplify branch's environment when targeting
a non-default backend. Backend changes are deployed separately.

Deploy backend hidden-index support before this frontend. Hidden browsing uses
`hidden=true` on both `/api/dir` and `/api/file`. Current indexes include both
views; missing indexes return HTTP 404. Retired images are indexed again through
`/api/info`, not by toggling the hidden view.

Rich image previews use the backend's read-only `GET /api/metadata?image=…&digest=…`
endpoint, which returns the existing `/api/info` response or 404 without starting
an indexing job. Deploy backend support for this endpoint to enable the extra
facts; older backends safely fall back to reference-only previews. The frontend
waits at most two seconds for metadata before using that fallback.

`GET /api/og?image=…&digest=…&v=1` renders the dynamic PNG. For completed indexes
it also reads `/api/dir` at the root, with a separate two-second timeout. Neither
request can start indexing. Complete cards can be cached for one hour by clients
and one day by CDNs; incomplete cards use `no-store` so they can improve after
indexing. Bump the `v` parameter in `socialImageUrl` when changing the design to
give social crawlers a new image URL (platforms may keep their own preview cache).
