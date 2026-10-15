import React from "react";
import {test} from "node:test";
import assert from "node:assert/strict";
import {act, create, ReactTestRenderer} from "react-test-renderer";
import {SWRConfig} from "swr";
import {FileTree, TypeDirectory, TypeRegular, TypeSymlink} from "../components/fileTree";

test("opening a directory follows sole-child chains, stops at siblings, and remains collapsible", async () => {
  const entry = (Name: string, Typeflag = TypeDirectory) => ({
    Hdr: {Name, Typeflag, Size: 0, Mode: 493, Linkname: ""}, Offset: 0, Layer: "",
  });
  const directories: Record<string, ReturnType<typeof entry>[]> = {
    "/": [entry("/"), entry("chain/"), entry("mixed/"), entry("link/"), entry("empty/")],
    "chain/": [entry("chain/one/")],
    "chain/one/": [entry("chain/one/two/")],
    "chain/one/two/": [entry("chain/one/two/a/"), entry("chain/one/two/b/")],
    "mixed/": [entry("mixed/child/"), entry("mixed/file", TypeRegular)],
    "link/": [entry("link/target", TypeSymlink)],
    "empty/": [],
  };
  const originalFetch = global.fetch;
  let tree: ReactTestRenderer | undefined;
  global.fetch = async input => {
    const path = new URL(String(input)).searchParams.get("path")!;
    assert.ok(path in directories, `unexpected directory load: ${path}`);
    return new Response(JSON.stringify(directories[path]));
  };
  try {
    await act(async () => {
      tree = create(<SWRConfig value={{provider: () => new Map(), dedupingInterval: 0}}>
        <FileTree image="fixture" digest="sha256:test" hidden={false} focusedPath=""
                  scrollToPath="" onSelect={() => {}}/>
      </SWRConfig>);
    });
    const rows = () => tree!.root.findAllByProps({role: "treeitem"});
    const row = (name: string) => rows().find(r =>
      r.findByProps({className: "tree-name"}).children[0] === name)!;
    const click = async (name: string) => {
      await act(async () => { row(name).props.onClick(); });
    };

    assert.equal(row("chain").props["aria-expanded"], false);
    await click("chain");
    for (const name of ["chain", "one", "two"]) assert.equal(row(name).props["aria-expanded"], true, name);
    for (const name of ["a", "b"]) assert.equal(row(name).props["aria-expanded"], false, name);
    await click("one");
    assert.equal(row("one").props["aria-expanded"], false);
    assert.equal(row("two"), undefined);
    await click("one");
    assert.equal(row("two").props["aria-expanded"], true);
    await click("chain");
    await click("chain");
    assert.equal(row("two").props["aria-expanded"], true, "cached children expand on reopening");
    await click("mixed");
    assert.equal(row("child").props["aria-expanded"], false, "a file sibling stops expansion");
    await click("link");
    assert.equal(row("target").type, "a", "a lone symlink is not expanded");
    await click("empty");
    assert.ok(tree!.root.findAllByProps({className: "tree-status"}).some(r => r.children.includes("Empty directory")));
  } finally {
    act(() => tree?.unmount());
    global.fetch = originalFetch;
  }
});
