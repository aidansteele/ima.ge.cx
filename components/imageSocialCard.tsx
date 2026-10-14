import React from "react";
import type {ImageInfo} from "../src/imageHooks";
import type {FilesystemSummary} from "../src/imageMetadata";
import {formatBytes} from "../src/formatBytes";
import {registryOf, splitTag} from "../src/tags";

const accent = "#7c8cff";
const muted = "#9ba4ba";

function Cube({size = 38}: {size?: number}) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
    <path d="M12 2.5 20.5 7.25v9.5L12 21.5 3.5 16.75v-9.5L12 2.5Z M3.8 7.4 12 12l8.2-4.6M12 12v9.2"
          stroke={accent} strokeWidth="1.5" strokeLinejoin="round"/>
  </svg>;
}

export function ImageSocialCard({image, digest, info, filesystem}: {
  image: string;
  digest: string;
  info?: ImageInfo;
  filesystem?: FilesystemSummary;
}) {
  const {repo, tag} = splitTag(image);
  const registry = registryOf(repo);
  const parts = repo.split("/");
  if (parts.length > 1 && parts[0] === registry) parts.shift();
  const name = parts.pop()!;
  const namespace = parts.join("/") || (registry === "docker.io" ? "library" : "");
  const context = [registry, namespace].filter(Boolean).join("/");
  const displayTag = tag ?? (digest ? undefined : "latest");
  const platform = [info?.Config?.os, info?.Config?.architecture, info?.Config?.variant].filter(Boolean).join("/");
  const layers = info?.Manifest?.layers;
  const compressed = info?.TotalSize ?? (layers ? layers.reduce((n, layer) => n + layer.size, 0) : undefined);
  const footer = [
    compressed !== undefined ? `${formatBytes(compressed, 1).trim()} compressed` : "",
    layers ? `${layers.length} ${layers.length === 1 ? "layer" : "layers"}` : "",
  ].filter(Boolean).join(" · ");

  return <div style={{display: "flex", position: "relative", width: 1200, height: 630,
    backgroundColor: "#0b0d12", color: "#e6e8ee", fontFamily: "Inter"}}>
    <div style={{display: "flex", position: "absolute", top: 44, left: 48, alignItems: "center", gap: 12}}>
      <Cube/><span style={{fontSize: 32, fontWeight: 700}}>ima.ge.cx</span>
    </div>
    <div style={{position: "absolute", top: 57, right: 48, fontSize: 16, color: muted, letterSpacing: 2}}>FILESYSTEM EXPLORER</div>

    <div style={{display: "flex", position: "absolute", top: 137, left: 48, width: 532, flexDirection: "column"}}>
      {/* Satori requires display:block on the text itself for lineClamp. */}
      <div style={{display: "block", fontSize: 19, color: muted, lineHeight: 1.25, height: 48,
        wordBreak: "break-word", lineClamp: 2}}>{context}</div>
      <div style={{display: "block", fontSize: name.length <= 12 ? 72 : name.length <= 22 ? 54 : 36, fontWeight: 700,
        lineHeight: 1.05, letterSpacing: -1.5, wordBreak: "break-word", lineClamp: name.length <= 22 ? 1 : 2}}>{name}</div>
      {displayTag && <div style={{display: "block", fontSize: displayTag.length <= 26 ? 30 : 22, color: accent, marginTop: 12,
        lineHeight: 1.25, wordBreak: "break-word", lineClamp: 3}}>{`:${displayTag}`}</div>}
    </div>

    {platform && <div style={{display: "flex", position: "absolute", top: 389, left: 48, border: "1px solid #414c75",
      borderRadius: 20, padding: "6px 14px", color: "#aab5ff", fontSize: 20, maxWidth: 530}}>{platform}</div>}

    {filesystem ? <>
      <div style={{display: "flex", position: "absolute", top: 452, left: 48, width: 542, gap: 28}}>
        <div style={{display: "flex", flexDirection: "column", paddingRight: 28, borderRight: "1px solid #30384a"}}>
          <div style={{fontSize: filesystem.files >= 1000000 ? 38 : 46, fontWeight: 700, letterSpacing: -1}}>{filesystem.files.toLocaleString("en-US")}</div>
          <div style={{fontSize: 18, color: muted, marginTop: 6}}>files to explore</div>
        </div>
        <div style={{display: "flex", flexDirection: "column"}}>
          <div style={{fontSize: 46, fontWeight: 700, letterSpacing: -1}}>{formatBytes(filesystem.bytes, 1).trim()}</div>
          <div style={{fontSize: 18, color: muted, marginTop: 6}}>uncompressed</div>
        </div>
      </div>
      <div style={{display: "flex", position: "absolute", left: 636, top: 137, width: 516, height: 384,
        backgroundColor: "#111620", border: "1px solid #2b3349", borderRadius: 18, padding: 28, flexDirection: "column"}}>
        <div style={{fontSize: 29, fontWeight: 700}}>Largest directories</div>
        <div style={{fontSize: 18, color: muted, marginTop: 7}}>Uncompressed size</div>
        <div style={{display: "flex", flexDirection: "column", marginTop: 24}}>
          {filesystem.directories.map(dir => <div key={dir.path} style={{display: "flex", alignItems: "center", height: 60}}>
            <svg width="32" height="28" viewBox="0 0 32 28" fill="none">
              <path d="M2 7V4a2 2 0 0 1 2-2h8l4 5h12a2 2 0 0 1 2 2v15a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V7Z"
                    stroke={accent} strokeWidth="2" strokeLinejoin="round"/>
            </svg>
            <div style={{fontSize: 23, marginLeft: 16, width: 250, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis"}}>{dir.path}</div>
            <div style={{fontSize: 22, color: "#c1cae0", marginLeft: "auto"}}>{formatBytes(dir.bytes, 1).trim()}</div>
          </div>)}
          {filesystem.directories.length === 0 && <div style={{fontSize: 23, color: muted, marginTop: 24}}>
            {filesystem.files === 0 ? "Empty filesystem" : "Files live at the image root"}
          </div>}
        </div>
        <div style={{position: "absolute", left: 28, right: 28, bottom: 25, borderTop: "1px solid #30384a",
          paddingTop: 17, color: muted, fontSize: 19}}>Browse the complete filesystem →</div>
      </div>
    </> : <>
      <div style={{display: "flex", position: "absolute", left: 700, top: 173, opacity: 0.5}}><Cube size={280}/></div>
      <div style={{display: "flex", flexDirection: "column", position: "absolute", left: 48, top: 460,
        width: 540, color: muted, fontSize: 26, lineHeight: 1.4}}>
        <span>Browse files, layers & config.</span><span>No Docker required.</span>
      </div>
    </>}

    <div style={{display: "flex", position: "absolute", left: 48, right: 48, top: 561, paddingTop: 22,
      borderTop: "1px solid #30384a", justifyContent: "space-between", fontSize: 18, color: muted}}>
      <div>{footer || "See what’s inside →"}</div>
      <div>{digest ? `${digest.slice(0, 19)}…` : "ima.ge.cx"}</div>
    </div>
  </div>;
}
