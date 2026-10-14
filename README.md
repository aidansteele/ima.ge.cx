# [ima.ge.cx](https://ima.ge.cx)

Browse the files, layers and configuration of public container images without
pulling them to your machine or running Docker.

This repository contains the web frontend, built with Next.js 13, React 18,
Mantine 5, SWR and CodeMirror 6. Image indexing and file extraction live in the
[backend repository](https://github.com/aidansteele/ima.ge.cx-backend).

## Using the site

Enter an image reference at [ima.ge.cx](https://ima.ge.cx), or put it directly in
the URL, such as [ima.ge.cx/ubuntu:24.04](https://ima.ge.cx/ubuntu:24.04).
Choose a platform when an image has multiple variants. Images are indexed on
first access; the page shows progress while indexing runs.

- **Files:** expand directories, inspect file contents and permissions, follow
  symbolic links, or download files. Text files support syntax highlighting,
  folding and search with ⌘F / Ctrl+F inside the viewer.
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
digest and selected file.

On desktop, the file tree and viewer sit side by side. On phones, switch between
**Browse files** and **View file**. When reading a file, scroll the page to move
the metadata out of the way; long code lines can scroll horizontally.

The service supports public container images with gzip-compressed layers, not
arbitrary OCI artifacts or every layer compression format.

## Local development

Use Node.js 20 or later and npm. The Amplify build currently uses Node.js 20.

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
CORS. See the backend repository for its own setup and deployment instructions.

### Checks and production build

```shell
npm run lint
npx tsc --noEmit
npm run build
npm start
```

The production build also runs lint and type checks. There is no separate
automated test suite configured. For UI changes, check both desktop and phone
widths, including file deep links, switching between the tree and viewer, and
scrolling long files past the metadata.

## Code layout

- `pages/index.tsx`: image search, tag suggestions and examples.
- `pages/[...slug].tsx`: image routes, indexing states and workspace tabs.
- `components/`: file tree, file viewer, CodeMirror editor, layer history and
  other UI components.
- `src/imageHooks.ts`: API requests, response types and SWR caching.
- `src/useHashPath.ts`: file selection and shareable URL fragments.
- `styles/globals.css`: shared styling and responsive layouts.

## Deployment

The frontend is hosted on AWS Amplify. Pushing to `main` triggers a production
build and deployment to [ima.ge.cx](https://ima.ge.cx).
[amplify.yml](amplify.yml) installs dependencies with `npm ci`, runs
`npm run build` and publishes `.next`.

Set `NEXT_PUBLIC_API_BASE_URL` in the Amplify branch's environment when targeting
a non-default backend. Backend changes are deployed separately.
