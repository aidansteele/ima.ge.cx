import {CSSProperties} from "react";
import {Loader} from "@mantine/core";
import {formatBytes} from "../src/formatBytes";
import {ImageInfo} from "../src/imageHooks";
import {LayerChip, layerColor} from "./layerChip";

// Returns completed as a percentage of total, or 0 when total isn't known yet.
function percent(completed: number, total: number) {
  return total > 0 ? 100 * completed / total : 0;
}

export function ImageIndexingView({image, info}: { image: string, info?: ImageInfo }) {
  // Copy before sorting: info is SWR's cached data and must not be mutated.
  const progresses = [...(info?.Progresses ?? [])].sort((a, b) => b.TotalBytes - a.TotalBytes);
  const completed = info?.CompletedSize ?? 0;
  const total = info?.TotalSize ?? 0;
  const pct = percent(completed, total);

  return <div className="indexing">
    <div className="indexing-card">
      <div className="indexing-title"><Loader size="sm"/> Indexing <code>{image}</code></div>
      <p className="muted">This only happens the first time anyone opens this image. You can leave and come back.</p>

      <div className="indexing-total">
        <span className="indexing-pct">{Math.floor(pct)}%</span>
        <span className="muted">{formatBytes(completed, 1)} of {total ? formatBytes(total, 1) : "…"}</span>
      </div>
      <div className="bar big"><span style={{width: `${pct}%`}}/></div>

      <div className="indexing-layers">
        {progresses.map(lp => <div className="indexing-layer" key={lp.Layer}>
          <LayerChip digest={lp.Layer}/>
          <span className="muted">{formatBytes(lp.TotalBytes, 1)}</span>
          <div className="bar" style={{"--chip": layerColor(lp.Layer)} as CSSProperties}>
            <span style={{width: `${percent(lp.CompletedBytes, lp.TotalBytes)}%`}}/>
          </div>
        </div>)}
      </div>
    </div>
  </div>;
}
