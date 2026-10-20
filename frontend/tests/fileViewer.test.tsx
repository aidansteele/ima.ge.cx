import React from "react";
import {test} from "node:test";
import assert from "node:assert/strict";
import {act, create, ReactTestRenderer} from "react-test-renderer";
import {SWRConfig} from "swr";
import {Button} from "@mantine/core";
import {FileViewer} from "../components/fileViewer";
import {fetchFile} from "../src/fileContents";

test("file responses preserve binary bytes and reject oversized bodies rather than truncating them", async () => {
  const originalFetch = global.fetch;
  try {
    const bytes = new Uint8Array([0, 255, 128, 13, 10, 42]);
    global.fetch = async () => new Response(bytes);
    assert.deepEqual(new Uint8Array(await (await fetchFile("fixture", 6)).arrayBuffer()), bytes);
    await assert.rejects(fetchFile("fixture", 5), /exceeds/);

    // An advertised oversized body must be rejected without consuming it.
    let cancelled = false;
    global.fetch = async () => new Response(new ReadableStream({
      pull() { throw new Error("body must not be read"); },
      cancel() { cancelled = true; },
    }, {highWaterMark: 0}), {headers: {"Content-Length": "7"}});
    await assert.rejects(fetchFile("fixture", 6), /exceeds/);
    assert.equal(cancelled, true);

    // Missing or dishonest Content-Length must not bypass the running byte limit.
    for (const length of [undefined, "1"]) {
      cancelled = false;
      let chunk = 0;
      global.fetch = async () => new Response(new ReadableStream({
        pull(controller) { controller.enqueue(new Uint8Array(++chunk === 1 ? 4 : 3)); },
        cancel() { cancelled = true; },
      }, {highWaterMark: 0}), {headers: length ? {"Content-Length": length} : {}});
      await assert.rejects(fetchFile("fixture", 6), /exceeds/);
      assert.equal(cancelled, true);
    }
    global.fetch = async () => new Response("unavailable", {status: 503});
    await assert.rejects(fetchFile("fixture", 6), /HTTP 503/);
    global.fetch = async () => { throw new TypeError("Failed to fetch"); };
    await assert.rejects(fetchFile("fixture", 6), /Failed to fetch/);
  } finally {
    global.fetch = originalFetch;
  }
});

test("viewer gates requests by indexed size, keeps binary downloads, and scopes retryable errors to the selection", async () => {
  const originalFetch = global.fetch;
  const originalReact = Object.getOwnPropertyDescriptor(globalThis, "React");
  Object.defineProperty(globalThis, "React", {configurable: true, value: React});
  const sizes: Record<string, number> = {
    "binary": 4, "preview-limit": 1_048_576, "over-preview": 1_048_577,
    "download-limit": 4_194_304, "over-download": 4_194_305, "empty": 0,
  };
  const requests: URL[] = [];
  let failDownload = false;
  let tree: ReactTestRenderer | undefined;
  global.fetch = async input => {
    const url = new URL(String(input));
    if (url.pathname.endsWith("/dir")) {
      return new Response(JSON.stringify(Object.entries(sizes).map(([Name, Size]) => ({
        Hdr: {Name, Size, Typeflag: 48, Mode: 420, Linkname: ""}, Offset: 0, Layer: "",
      }))));
    }
    requests.push(url);
    if (failDownload) return new Response("unavailable", {status: 503});
    return new Response(new Uint8Array(sizes[url.searchParams.get("path")!]));
  };
  const cache = new Map();
  const render = (path: string, hidden = false) => <SWRConfig value={{provider: () => cache, dedupingInterval: 0}}>
    <FileViewer image="fixture" digest="sha256:test" path={path} hidden={hidden} onSelect={() => {}}/>
  </SWRConfig>;
  const select = async (path: string, hidden = false) => {
    await act(async () => { tree!.update(render(path, hidden)); });
  };
  const button = () => tree!.root.findAllByType(Button).find(b => b.props.children === "Download")!;
  const title = () => tree!.root.findByProps({className: "empty-title"}).children.join("");
  const alerts = () => tree!.root.findAllByProps({role: "alert"});
  try {
    await act(async () => { tree = create(render("over-preview")); });
    assert.equal(title(), "File too large to preview");
    assert.equal(requests.length, 0, "oversized selection must not fetch contents");
    assert.equal(button().props.disabled, false, "preview and download limits are independent");
    await select("download-limit");
    assert.equal(button().props.disabled, false);
    await select("over-download");
    assert.equal(button().props.disabled, true);
    await select("empty");
    assert.equal(title(), "Empty file");
    assert.equal(requests.length, 0);
    await select("preview-limit");
    assert.equal(title(), "Binary file", "exactly 1 MiB is previewable");
    await select("binary", true);
    assert.equal(title(), "Binary file");
    assert.equal(button().props.disabled, false);
    assert.equal(requests.at(-1)!.searchParams.get("hidden"), "true");

    failDownload = true;
    await act(async () => { await button().props.onClick(); });
    assert.match(alerts()[0].children.join(""), /HTTP 503.*try again/);
    assert.equal(button().props.loading, false);
    const beforeRetry = requests.length;
    await act(async () => { await button().props.onClick(); });
    assert.equal(requests.length, beforeRetry + 1);
    assert.equal(requests.at(-1)!.searchParams.get("hidden"), "true");
    await select("over-preview");
    assert.equal(alerts().length, 0, "another file must not inherit download errors");
  } finally {
    act(() => tree?.unmount());
    global.fetch = originalFetch;
    if (originalReact) Object.defineProperty(globalThis, "React", originalReact);
    else Reflect.deleteProperty(globalThis, "React");
  }
});
