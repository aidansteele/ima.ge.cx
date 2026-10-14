import {CSSProperties} from "react";

const palette = ["#7c8cff", "#3ecf8e", "#f5a524", "#f76b8a", "#4cc9f0", "#b388ff", "#ff8a4c", "#a3e635"];

// A stable colour per layer, so the same layer is recognisable across the tree and timeline.
export function layerColor(digest: string) {
  return palette[parseInt(digest.slice(7, 15), 16) % palette.length] ?? palette[0];
}

// A stable colour for any other string, such as an image name.
export function colorFor(text: string) {
  let hash = 0;
  for (const c of text) hash = (hash * 31 + c.charCodeAt(0)) >>> 0;
  return palette[hash % palette.length];
}

export function LayerChip({digest}: { digest: string }) {
  if (!digest) return null;
  return <span className="layer-chip" title={digest} style={{"--chip": layerColor(digest)} as CSSProperties}>
    {digest.slice(7, 15)}
  </span>;
}
