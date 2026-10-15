import React from "react";
import {test} from "node:test";
import assert from "node:assert/strict";
import {act, create, ReactTestRenderer} from "react-test-renderer";
import {hashFromPath, useHashPath} from "../src/useHashPath";

test("file and line links round-trip without losing URL state or adding history", () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "window");
  let tree: ReactTestRenderer | undefined;
  let url = new URL("https://ima.ge.cx/example:v1@sha256:abc?hidden=true&theme=light#/etc/config:L42");
  const state = {__N: true, key: "existing-router-entry"};
  const history = {
    state,
    replaceState: (next: typeof state, _: string, href: string | URL) => {
      history.state = next;
      url = new URL(href, url);
    },
    pushState: () => assert.fail("selection must not add a history entry"),
  };
  const window = Object.assign(new EventTarget(), {history});
  Object.defineProperty(window, "location", {get: () => url});
  Object.defineProperty(globalThis, "window", {configurable: true, value: window});
  const Consumer = () => {
    const [path, select, hidden, selectHidden, line] = useHashPath();
    return <div data-path={path} data-line={line} data-hidden={hidden}>
      <button onClick={() => select("etc/config", 17)}>Line</button>
      <button onClick={() => select("etc/other")}>File</button>
      <button onClick={() => selectHidden(false)}>Hidden</button>
    </div>;
  };
  try {
    act(() => { tree = create(<Consumer/>); });
    const check = (path: string, line?: number) => {
      const props = tree!.root.findByType("div").props;
      assert.equal(props["data-path"], path);
      assert.equal(props["data-line"], line);
    };
    check("etc/config", 42);
    act(() => tree!.root.findAllByType("button")[0].props.onClick());
    check("etc/config", 17);
    assert.equal(url.href, "https://ima.ge.cx/example:v1@sha256:abc?hidden=true&theme=light#/etc/config:L17");
    assert.deepEqual(history.state, {...state, as: "/example:v1@sha256:abc?hidden=true&theme=light#/etc/config:L17"});
    act(() => tree!.root.findAllByType("button")[1].props.onClick());
    check("etc/other");
    assert.equal(url.hash, "#/etc/other");

    const special = "etc/雪 space%?#:L42";
    assert.equal(hashFromPath(special, 9), "#/etc/%E9%9B%AA%20space%25%3F%23%3AL42:L9");
    for (const [hash, path, line] of [
      [hashFromPath(special, 9), special, 9],
      [hashFromPath(special), special, undefined],
      ["#/etc/config:L1", "etc/config", 1],
      ["#etc/config", "etc/config", undefined],
      ["#/bad%ZZ:L3", "bad%ZZ", 3],
      ...["0", "-1", "1.5", "abc", "9007199254740992"].map(n => [`#/etc/config:L${n}`, "etc/config", undefined]),
    ] as [string, string, number | undefined][]) {
      act(() => { url.hash = hash; window.dispatchEvent(new Event("hashchange")); });
      check(path, line);
    }
    act(() => { url.hash = "#/etc/config:L8"; window.dispatchEvent(new Event("popstate")); });
    check("etc/config", 8);
    act(() => tree!.root.findAllByType("button")[2].props.onClick());
    check("");
    assert.equal(url.hash, "");
    assert.equal(url.search, "?theme=light");
    assert.equal(tree!.root.findByType("div").props["data-hidden"], false);
  } finally {
    act(() => tree?.unmount());
    if (original) Object.defineProperty(globalThis, "window", original);
    else Reflect.deleteProperty(globalThis, "window");
  }
});
