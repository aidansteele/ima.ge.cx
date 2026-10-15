import React from "react";
import {test} from "node:test";
import assert from "node:assert/strict";
import {act, create, ReactTestRenderer} from "react-test-renderer";
import {RouterContext} from "next/dist/shared/lib/router-context";
import {NextRouter} from "next/router";
import {ThemeProvider, useTheme} from "../src/theme";

test("system changes supersede saved and URL themes without losing workspace state", async () => {
  const originals = new Map(["React", "matchMedia", "localStorage", "document", "window", "location"]
    .map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  let tree: ReactTestRenderer | undefined;
  try {
    for (const initial of ["saved", "url", "system", "blocked-storage"]) {
      const media = Object.assign(new EventTarget(), {matches: true});
      const storage = new Map(initial === "saved" ? [["theme", "light"]] : []);
      const document = {documentElement: {dataset: {theme: ""}}};
      let url = new URL(`https://ima.ge.cx/fixture?hidden=true${initial === "url" ? "&theme=light" : ""}#/etc/os-release`);
      const router = {
        query: Object.fromEntries(url.searchParams),
        replace: async (path: string) => {
          url = new URL(path, url);
          router.query = Object.fromEntries(url.searchParams);
          tree!.update(render());
          return true;
        },
      };
      const localStorage = {
        getItem: (key: string) => {
          if (initial === "blocked-storage") throw new Error("Storage blocked");
          return storage.get(key) ?? null;
        },
        setItem: (key: string, value: string) => {
          if (initial === "blocked-storage") throw new Error("Storage blocked");
          storage.set(key, value);
        },
      };
      for (const [key, value] of Object.entries({React, matchMedia: () => media, localStorage,
        document, window: new EventTarget()})) {
        Object.defineProperty(globalThis, key, {configurable: true, value});
      }
      Object.defineProperty(globalThis, "location", {configurable: true, get: () => url});
      const Consumer = () => {
        const {scheme, toggle, themedPath} = useTheme();
        return <button onClick={toggle} data-path={themedPath("/other")}>{scheme}</button>;
      };
      const render = () => <RouterContext.Provider value={router as unknown as NextRouter}>
        <ThemeProvider><Consumer/></ThemeProvider>
      </RouterContext.Provider>;
      await act(async () => { tree = create(render()); });
      const check = (scheme: string) => {
        assert.deepEqual(tree!.root.findByType("button").children, [scheme], initial);
        assert.equal(document.documentElement.dataset.theme, scheme, initial);
        assert.equal(url.pathname, "/fixture");
        assert.equal(url.searchParams.get("hidden"), "true");
        assert.equal(url.hash, "#/etc/os-release");
      };
      check("light");
      for (const scheme of ["dark", "light"]) {
        await act(async () => {
          media.matches = scheme === "light";
          media.dispatchEvent(new Event("change"));
        });
        check(scheme);
        assert.equal(url.searchParams.get("theme"), scheme);
        assert.equal(tree!.root.findByType("button").props["data-path"], `/other?theme=${scheme}`);
      }
      await act(async () => { tree!.root.findByType("button").props.onClick(); });
      check("dark");
      // A manual choice still works, but does not pin subsequent OS changes.
      for (const scheme of ["dark", "light"]) {
        await act(async () => {
          media.matches = scheme === "light";
          media.dispatchEvent(new Event("change"));
        });
        check(scheme);
      }
      act(() => tree!.unmount());
      tree = undefined;
    }
  } finally {
    act(() => tree?.unmount());
    for (const [key, descriptor] of Array.from(originals)) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
});
