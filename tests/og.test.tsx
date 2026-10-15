import React from "react";
import {afterEach, test} from "node:test";
import assert from "node:assert/strict";
import {renderToStaticMarkup} from "react-dom/server";
import type {NextApiRequest, NextApiResponse} from "next";
import handler from "../pages/api/og";
import {ImageSocialCard} from "../components/imageSocialCard";
import {imageDescription, readFilesystemSummary} from "../src/imageMetadata";

const digest = `sha256:${"a".repeat(64)}`;
const originalFetch = global.fetch;
afterEach(() => { global.fetch = originalFetch; });

const info = {Status: "SUCCEEDED", Config: {os: "linux", architecture: "arm64", variant: "v8"},
  TotalSize: 4194304, Manifest: {layers: [{size: 1048576}, {size: 3145728}]}};
const entries = [
  {Hdr: {Name: "/", Typeflag: 53}, DirSize: 1670, DirFiles: 17},
  {Hdr: {Name: "etc/", Typeflag: 53}, DirSize: 120, DirFiles: 2},
  {Hdr: {Name: "usr/", Typeflag: 53}, DirSize: 900, DirFiles: 8},
  {Hdr: {Name: "var/", Typeflag: 53}, DirSize: 600, DirFiles: 4},
  {Hdr: {Name: "tmp/", Typeflag: 53}}, // omitempty: an empty directory
  {Hdr: {Name: "init", Typeflag: 48, Size: 30}},
  {Hdr: {Name: "empty", Typeflag: 48, Size: 0}},
  {Hdr: {Name: "legacy", Typeflag: 0, Size: 20}},
  {Hdr: {Name: "symlink", Typeflag: 50, Size: 999}},
  {Hdr: {Name: "hardlink", Typeflag: 49, Size: 888}},
];

function api(body: unknown = entries, status = 200) {
  global.fetch = async (input) => {
    const url = new URL(String(input));
    assert.equal(url.origin, "https://api.ima.ge.cx");
    assert.ok(["/api/metadata", "/api/dir"].includes(url.pathname), "previews must only use read-only endpoints");
    assert.equal(url.searchParams.get("digest"), digest);
    return new Response(JSON.stringify(url.pathname.endsWith("metadata") ? info : body), {status});
  };
}

test("filesystem totals count regular files once, ignore links, and rank directories by bytes", async () => {
  api();
  assert.deepEqual(await readFilesystemSummary("example/app:v1", digest), {
    bytes: 1670, files: 17,
    directories: [{path: "/usr", bytes: 900}, {path: "/var", bytes: 600}, {path: "/etc", bytes: 120}],
  });
  api(null);
  assert.deepEqual(await readFilesystemSummary("empty", digest), {bytes: 0, files: 0, directories: []});
  api({}, 503);
  assert.equal(await readFilesystemSummary("unavailable", digest), undefined);
});

test("image descriptions retain platform, compressed size, layers and full digest", async () => {
  api();
  const description = await imageDescription("ghcr.io/org/app:v1", digest);
  assert.ok(description.startsWith("ghcr.io/org/app:v1 — linux/arm64/v8 · 2 layers · 4 MB compressed."));
  assert.ok(description.includes(digest));
});

test("card identity separates registry ports, namespaces and tags without inventing a tag for digest links", () => {
  for (const [image, context, name, tag] of [
    ["nginx:1.27", "docker.io/library", "nginx", ":1.27"],
    ["mcr.microsoft.com/dotnet/sdk:9.0-preview", "mcr.microsoft.com/dotnet", "sdk", ":9.0-preview"],
    ["registry.example.com:5000/team/controller:v2", "registry.example.com:5000/team", "controller", ":v2"],
    ["docker.io", "docker.io/library", "docker.io", ""],
  ]) {
    const html = renderToStaticMarkup(<ImageSocialCard image={image} digest={digest}/>);
    for (const value of [context, name, tag].filter(Boolean)) assert.ok(html.includes(`>${value}<`), value);
    assert.ok(!html.includes(":latest"));
    assert.ok(!html.includes("files to explore"), "unavailable statistics must not appear as zero");
  }
});

async function request(query: NextApiRequest["query"], method = "GET") {
  const result = {status: 0, headers: {} as Record<string, string>, body: undefined as Buffer | string | undefined};
  const res = {
    setHeader(key: string, value: string) { result.headers[key] = value; },
    status(code: number) { result.status = code; return res; },
    send(body: Buffer | string | undefined) { result.body = body; },
    end() {},
  };
  await handler({query, method} as NextApiRequest, res as unknown as NextApiResponse);
  return result;
}

test("OG endpoint produces a 1200×630 PNG and only caches complete filesystem cards", async () => {
  api();
  const indexed = await request({image: "nginx:1.27", digest});
  assert.equal(indexed.status, 200);
  assert.equal(indexed.headers["Content-Type"], "image/png");
  assert.match(indexed.headers["Cache-Control"], /s-maxage=86400/);
  const png = indexed.body as Buffer;
  assert.deepEqual(Array.from(png.subarray(0, 8)), [137, 80, 78, 71, 13, 10, 26, 10]);
  assert.equal(png.readUInt32BE(16), 1200);
  assert.equal(png.readUInt32BE(20), 630);

  const dark = await request({image: "nginx:1.27", digest, theme: "dark"});
  assert.deepEqual(dark.body, indexed.body, "existing URLs retain the dark card");
  const light = await request({image: "nginx:1.27", digest, theme: "light"});
  assert.equal(light.status, 200);
  assert.equal(light.headers["Cache-Control"], indexed.headers["Cache-Control"]);
  assert.equal((light.body as Buffer).readUInt32BE(16), 1200);
  assert.equal((light.body as Buffer).readUInt32BE(20), 630);
  assert.notDeepEqual(light.body, dark.body, "the theme query must change the rendered PNG");

  api({}, 404);
  const unavailable = await request({image: "nginx:1.27", digest});
  assert.equal(unavailable.status, 200);
  assert.equal(unavailable.headers["Cache-Control"], "no-store");
  assert.notDeepEqual(unavailable.body, indexed.body);
  const tagOnly = await request({image: "nginx"});
  assert.equal(tagOnly.status, 200);
  assert.equal(tagOnly.headers["Cache-Control"], "no-store");
  const lightFallback = await request({image: "nginx", theme: "light"});
  assert.equal(lightFallback.status, 200);
  assert.equal(lightFallback.headers["Cache-Control"], "no-store");
  assert.notDeepEqual(lightFallback.body, tagOnly.body);
});

test("OG endpoint rejects oversized or malformed requests and unsupported methods", async () => {
  global.fetch = async () => { throw new Error("invalid requests should never fetch data"); };
  for (const query of [{}, {image: ["one", "two"]}, {image: "x".repeat(513)}, {image: "<script>"},
    {image: "https://example.com/image"}, {image: "nginx/"}, {image: "nginx", digest: "sha256:short"},
    {image: "nginx", theme: "auto"}, {image: "nginx", theme: ["light", "dark"]}]) {
    assert.equal((await request(query)).status, 400);
  }
  const post = await request({image: "nginx"}, "POST");
  assert.equal(post.status, 405);
  assert.equal(post.headers.Allow, "GET, HEAD");
});
