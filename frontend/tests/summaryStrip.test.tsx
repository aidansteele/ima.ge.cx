import React from "react";
import {test} from "node:test";
import assert from "node:assert/strict";
import {renderToStaticMarkup} from "react-dom/server";
import {SWRConfig} from "swr";
import {SummaryStrip} from "../components/summaryStrip";
import {ImageInfo} from "../src/imageHooks";

test("summary shows effective compressed GB/s only for completed indexing with a valid duration", () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, "React");
  Object.defineProperty(globalThis, "React", {configurable: true, value: React});
  try {
    const render = (overrides: Partial<ImageInfo> = {}) => renderToStaticMarkup(
      <SWRConfig value={{provider: () => new Map()}}>
        <SummaryStrip image="fixture" digest="sha256:test"
                      info={{Status: "SUCCEEDED", TotalSize: 2_500_000_000, DurationSeconds: 8, ...overrides}}/>
      </SWRConfig>,
    );
    const html = render();
    assert.match(html, />2\.5 GB</);
    assert.match(html, />Indexed in 8 s</);
    assert.match(html, />0\.313 GB\/s effective</);
    // Small images and subsecond cached runs must not round to zero.
    const small = render({TotalSize: 40_000, DurationSeconds: 0.025});
    assert.match(small, />0\.00004 GB</);
    assert.match(small, />Indexed in 0\.025 s</);
    assert.match(small, />0\.0016 GB\/s effective</);
    for (const DurationSeconds of [undefined, 0, -1, NaN, Infinity]) {
      const legacy = render({DurationSeconds});
      assert.match(legacy, />Indexed</);
      assert.doesNotMatch(legacy, /Indexed in|GB\/s/);
    }
    for (const Status of ["PENDING", "RUNNING", "FAILED"] as const) {
      assert.doesNotMatch(render({Status}), /Indexed in|GB\/s/);
    }
    for (const TotalSize of [undefined, -1, NaN, Infinity]) {
      const unknown = render({TotalSize});
      assert.match(unknown, />unknown</);
      assert.match(unknown, />Indexed in 8 s</);
      assert.doesNotMatch(unknown, /GB\/s/);
    }
    assert.match(render({TotalSize: 0}), />0 GB\/s effective</);
  } finally {
    if (original) Object.defineProperty(globalThis, "React", original);
    else Reflect.deleteProperty(globalThis, "React");
  }
});
