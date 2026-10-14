import {Fragment, useState} from "react";
import {Button, CopyButton, Loader} from "@mantine/core";
import {IconArrowRight, IconCheck, IconDownload, IconFileText, IconFileUnknown, IconFolder, IconLink} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {fileUrl, useDirectory, useFileContents} from "../src/imageHooks";
import {LayerChip} from "./layerChip";
import {TypeDirectory, TypeSymlink} from "./fileTree";

const MaxLines = 5000;

function parentDir(path: string) {
  const i = path.replace(/\/$/, "").lastIndexOf("/");
  return i < 0 ? "/" : path.slice(0, i + 1);
}

// Resolves a symlink target relative to the link's directory, as a path in
// the index's form (no leading slash).
function resolveLink(path: string, target: string) {
  const base = target.startsWith("/") ? [] : parentDir(path).split("/").filter(Boolean);
  for (const part of target.split("/")) {
    if (part === "..") base.pop();
    else if (part && part !== ".") base.push(part);
  }
  return base.join("/");
}

function looksBinary(text: string) {
  const sample = text.slice(0, 8000);
  return sample.includes("\u0000") || (sample.match(/\uFFFD/g)?.length ?? 0) > sample.length / 100;
}

async function download(url: string, name: string) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const a = document.createElement("a");
  a.href = URL.createObjectURL(await res.blob());
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
}

function Empty({icon, title, children}: { icon: JSX.Element, title: string, children?: React.ReactNode }) {
  return <div className="empty">
    {icon}
    <div className="empty-title">{title}</div>
    {children && <div className="empty-body">{children}</div>}
  </div>;
}

function CodeView({text}: { text: string }) {
  const lines = text.replace(/\n$/, "").split("\n");
  const shown = lines.slice(0, MaxLines);
  return <>
    <div className="code" style={{"--gutter": `${String(shown.length).length + 1}ch`} as React.CSSProperties}>
      {shown.map((line, i) => <div className="code-line" key={i}>
        <span className="ln">{i + 1}</span><span className="lc">{line || " "}</span>
      </div>)}
    </div>
    {lines.length > MaxLines &&
      <div className="code-truncated">Showing the first {MaxLines.toLocaleString()} of {lines.length.toLocaleString()} lines. Download the file to see the rest.</div>}
  </>;
}

export function FileViewer({image, digest, path, onSelect}: { image: string, digest: string, path: string, onSelect: (path: string) => void }) {
  // The tree has already listed the parent directory, so this reuses its cached response.
  const dir = useDirectory(image, digest, parentDir(path), path !== "");
  const entry = dir.dirents.find(e => e.path === path || e.path === `${path}/`);
  const isFile = !!entry && entry.typ !== TypeDirectory && entry.typ !== TypeSymlink && entry.size > 0;
  const contents = useFileContents(image, digest, path, isFile);
  const [downloading, setDownloading] = useState(false);

  if (!path) {
    return <Empty icon={<IconFileText size={36}/>} title="Select a file to view it">
      The address bar always links to the file you&apos;re viewing, so you can share it.
    </Empty>;
  }

  const name = path.replace(/\/$/, "").split("/").pop()!;
  const segments = path.replace(/\/$/, "").split("/");

  let body;
  if (dir.isLoading) {
    body = <div className="viewer-loading"><Loader size="sm"/></div>;
  } else if (dir.error) {
    body = <Empty icon={<IconFileUnknown size={36}/>} title="Couldn't load this directory">{dir.error.message}</Empty>;
  } else if (!entry) {
    body = <Empty icon={<IconFileUnknown size={36}/>} title="Not in this image">/{path} doesn&apos;t exist in this image.</Empty>;
  } else if (entry.typ === TypeDirectory) {
    body = <Empty icon={<IconFolder size={36}/>} title="This is a directory">Its contents are expanded in the tree.</Empty>;
  } else if (entry.typ === TypeSymlink) {
    const target = resolveLink(path, entry.linkname);
    body = <Empty icon={<IconLink size={36}/>} title="Symbolic link">
      <code>{entry.linkname}</code>
      <Button mt="md" variant="light" rightIcon={<IconArrowRight size={14}/>} onClick={() => onSelect(target)}>Go to /{target}</Button>
    </Empty>;
  } else if (entry.size === 0) {
    body = <Empty icon={<IconFileText size={36}/>} title="Empty file"/>;
  } else if (contents.error) {
    body = <Empty icon={<IconFileUnknown size={36}/>} title="Couldn't load this file">{contents.error.message}</Empty>;
  } else if (contents.isLoading || contents.data === undefined) {
    body = <div className="viewer-loading"><Loader size="sm"/></div>;
  } else if (looksBinary(contents.data)) {
    body = <Empty icon={<IconFileUnknown size={36}/>} title="Binary file">Download it to inspect it locally.</Empty>;
  } else {
    body = <CodeView text={contents.data}/>;
  }

  return <div className="viewer">
    <div className="viewer-header">
      <nav className="breadcrumb" aria-label="Path">
        {segments.map((s, i) => <Fragment key={i}>
          <span className="crumb-sep">/</span>
          <span className={i === segments.length - 1 ? "crumb current" : "crumb"}>{s}</span>
        </Fragment>)}
      </nav>
      <div className="viewer-actions">
        <CopyButton value={typeof window === "undefined" ? "" : window.location.href}>
          {({copied, copy}) => <Button size="xs" variant="default" onClick={copy}
                                       leftIcon={copied ? <IconCheck size={14}/> : <IconLink size={14}/>}>
            {copied ? "Copied" : "Copy link"}
          </Button>}
        </CopyButton>
        <Button size="xs" leftIcon={<IconDownload size={14}/>} disabled={!isFile} loading={downloading}
                onClick={() => {
                  setDownloading(true);
                  download(fileUrl(image, digest, path), name).finally(() => setDownloading(false));
                }}>
          Download
        </Button>
      </div>
    </div>
    {entry && <div className="viewer-meta">
      {entry.typ !== TypeDirectory && <span>{formatBytes(entry.size)}</span>}
      <span>from layer <LayerChip digest={entry.layer}/></span>
      <span>mode {(entry.mode & 0o7777).toString(8).padStart(4, "0")}</span>
    </div>}
    <div className="viewer-body">{body}</div>
  </div>;
}
