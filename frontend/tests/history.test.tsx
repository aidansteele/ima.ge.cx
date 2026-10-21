import {test} from "node:test";
import assert from "node:assert/strict";
import {buildSteps} from "../src/history";
import {HistoryEntry, ImageInfo} from "../src/imageHooks";

const layers = [
  {digest: `sha256:${"a".repeat(64)}`, size: 123, mediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
  {digest: `sha256:${"b".repeat(64)}`, size: 456, mediaType: "application/vnd.oci.image.layer.v1.tar+gzip"},
];
const info = (history?: HistoryEntry[]): ImageInfo => ({Status: "SUCCEEDED", Manifest: {layers}, Config: {history}});

test("layer history preserves alignment across configuration steps and missing commands", () => {
  const steps = buildSteps(info([
    {created_by: "ADD base / # buildkit"},
    {created_by: "ENV MODE=prod", empty_layer: true},
    {created: "0001-01-01T00:00:00Z"}, // Crossplane records layers without created_by.
  ]));
  assert.deepEqual(steps.map(s => [s.instruction, s.command, s.layer?.digest]), [
    ["?", "(not recorded)", layers[1].digest],
    ["ENV", "MODE=prod", undefined],
    ["ADD", "base /", layers[0].digest],
  ]);
});

test("missing or mismatched history still exposes every layer without guessing instruction associations", () => {
  for (const history of [undefined, [], [{created_by: "COPY app /app"}],
    [{created_by: "RUN first"}, {created_by: "RUN second"}, {created_by: "RUN third"}],
  ]) {
    const steps = buildSteps(info(history));
    assert.deepEqual(steps.map(s => s.layer), [layers[1], layers[0]]);
    assert.ok(steps.every(s => s.instruction === "?"));
  }
  assert.deepEqual(buildSteps({Status: "SUCCEEDED"}), []);
  const metadataOnly = buildSteps({Status: "SUCCEEDED", Config: {history: [{created_by: "ENV X=1", empty_layer: true}]}});
  assert.equal(metadataOnly[0].instruction, "ENV");
  assert.equal(metadataOnly[0].layer, undefined);
});
