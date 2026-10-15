import {test} from "node:test";
import assert from "node:assert/strict";
import {sortTags} from "../src/tags";

test("tag search ranks exact matches before prefixes and other matches, preserving version order", () => {
  const tags = ["6.0-26.03.25", "8.0-26.07.29", "6.0", "latest", "6.0-26.07.29", "7.0-26.07.29"];
  const original = [...tags];
  assert.deepEqual(sortTags(tags, "6.0"), [
    "6.0", "6.0-26.07.29", "6.0-26.03.25", "latest", "8.0-26.07.29", "7.0-26.07.29",
  ]);
  assert.deepEqual(sortTags(["xRC", "rc-10", "RC", "rc-2"], " rC "), ["RC", "rc-10", "rc-2", "xRC"]);
  for (const query of [undefined, "", "  "]) {
    assert.deepEqual(sortTags(tags, query), [
      "latest", "8.0-26.07.29", "7.0-26.07.29", "6.0-26.07.29", "6.0-26.03.25", "6.0",
    ]);
  }
  assert.deepEqual(tags, original);
});
