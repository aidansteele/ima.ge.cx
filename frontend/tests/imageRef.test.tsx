import React from "react";
import {test} from "node:test";
import assert from "node:assert/strict";
import {act, create, ReactTestRenderer} from "react-test-renderer";
import {Select, SelectProps, Tooltip, TooltipProps} from "@mantine/core";
import {SWRConfig} from "swr";
import {RouterContext} from "next/dist/shared/lib/router-context";
import {NextRouter} from "next/router";
import {ImageRef} from "../components/imageRef";

test("tag selection updates immediately and follows successful, cancelled and failed navigation", async () => {
  const originals = new Map(["React", "window", "fetch"]
    .map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  const renders = [Select, Tooltip].map(component => ({component, descriptor: Object.getOwnPropertyDescriptor(component, "render")!}));
  let tree: ReactTestRenderer | undefined;
  try {
    // Use native controls at the UI-library boundary; portals need a browser.
    Object.defineProperty(Select, "render", {value: (props: SelectProps) =>
      <select aria-label={props["aria-label"]} value={props.value ?? ""}
        onChange={e => props.onChange?.(e.target.value)}/>});
    Object.defineProperty(Tooltip, "render", {value: (props: TooltipProps) => props.children});
    Object.defineProperty(globalThis, "React", {configurable: true, value: React});
    Object.defineProperty(globalThis, "window", {configurable: true, value: {location: {hash: "#/etc/os-release"}}});
    global.fetch = async input => new Response(JSON.stringify(String(input).includes("/tags?") ?
      {Tags: ["6.0", "7.0", "8.0"]} : {Options: []}));
    let finish!: (ok: boolean) => void;
    let fail!: (error: Error) => void;
    const router = {push: () => new Promise<boolean>((resolve, reject) => { finish = resolve; fail = reject; })};
    const cache = new Map();
    const render = (tag: string) => <RouterContext.Provider value={router as unknown as NextRouter}>
      <SWRConfig value={{provider: () => cache}}><ImageRef image={`fixture:${tag}`}/></SWRConfig>
    </RouterContext.Provider>;
    await act(async () => { tree = create(render("6.0")); });
    const field = () => tree!.root.findAllByType("select").find(n => n.props["aria-label"] === "Tag")!;
    const displayed = () => field().props.value;
    const select = (tag: string) => {
      act(() => { field().props.onChange({target: {value: tag}}); });
      assert.equal(displayed(), tag, "selection must not wait for navigation");
    };
    select("7.0");
    await act(async () => { tree!.update(render("7.0")); finish(true); });
    assert.equal(displayed(), "7.0");
    select("8.0");
    await act(async () => { finish(false); });
    assert.equal(displayed(), "7.0", "cancelled navigation restores the current tag");
    select("6.0");
    await act(async () => { fail(new Error("Navigation failed")); });
    assert.equal(displayed(), "7.0", "failed navigation restores the current tag");
    select("8.0");
    const cancelEarlier = finish;
    select("6.0");
    await act(async () => { cancelEarlier(false); });
    assert.equal(displayed(), "6.0", "an earlier cancellation must not undo a newer selection");
    await act(async () => { tree!.update(render("6.0")); finish(true); });
    await act(async () => { tree!.update(render("7.0")); });
    assert.equal(displayed(), "7.0", "browser history remains authoritative");
  } finally {
    act(() => tree?.unmount());
    for (const {component, descriptor} of renders) Object.defineProperty(component, "render", descriptor);
    for (const [key, descriptor] of Array.from(originals)) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
});
