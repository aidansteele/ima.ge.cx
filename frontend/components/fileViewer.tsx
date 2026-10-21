import {Fragment, useEffect, useRef, useState} from "react";
import {Button, CopyButton, Loader} from "@mantine/core";
import {IconArrowRight, IconCheck, IconDownload, IconEyeOff, IconFileText, IconFileUnknown, IconFolder, IconLink} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {fileUrl, useDirectory, useFileContents} from "../src/imageHooks";
import {fetchFile, MaxDownloadBytes, MaxPreviewBytes} from "../src/fileContents";
import {LayerChip} from "./layerChip";
import {TypeDirectory, TypeSymlink} from "./fileTree";
import {CodeView} from "./codeView";

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

async function download(url: string, name: string, signal: AbortSignal) {
  const blob = await fetchFile(url, MaxDownloadBytes, signal);
  if (signal.aborted) return;
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  document.body.appendChild(a);
  try {
    a.click();
  } finally {
    a.remove();
    // Give the browser time to consume the URL before releasing its bytes.
    setTimeout(() => URL.revokeObjectURL(a.href), 60_000);
  }
}

function Empty({icon, title, children}: { icon: JSX.Element, title: string, children?: React.ReactNode }) {
  return <div className="empty">
    {icon}
    <div className="empty-title">{title}</div>
    {children && <div className="empty-body">{children}</div>}
  </div>;
}

export function FileViewer({image, digest, path, line, onSelect, hidden}: {
  image: string, digest: string, path: string, line?: number, onSelect: (path: string, line?: number) => void, hidden: boolean
}) {
  // The tree has already listed the parent directory, so this reuses its cached response.
  const dir = useDirectory(image, digest, parentDir(path), path !== "", hidden);
  const entry = dir.dirents.find(e => e.path === path || e.path === `${path}/`);
  const isFile = !!entry && entry.typ !== TypeDirectory && entry.typ !== TypeSymlink && entry.size > 0;
  const previewTooLarge = !!entry && entry.size > MaxPreviewBytes;
  const downloadTooLarge = !!entry && entry.size > MaxDownloadBytes;
  const contents = useFileContents(image, digest, path, isFile && !previewTooLarge, hidden);
  const url = fileUrl(image, digest, path, hidden);
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState<string>();
  const downloadRequest = useRef<AbortController>();

  useEffect(() => {
    setDownloading(false);
    setDownloadError(undefined);
    return () => downloadRequest.current?.abort();
  }, [url]);

  if (!path) {
    return <Empty icon={hidden ? <IconEyeOff size={36}/> : <IconFileText size={36}/>}
                  title={hidden ? "Inspect files left behind" : "Select a file to view it"}>
      {hidden ? "These files were deleted from the final image, but their contents remain in earlier layers. Select one to inspect or download it." :
        "The address bar always links to the file you're viewing, so you can share it."}
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
    body = <Empty icon={<IconFileUnknown size={36}/>} title={hidden ? "Not a hidden file" : "Not in this image"}>
      /{path} {hidden ? "is not in the hidden-files view." : "doesn't exist in this image."}
    </Empty>;
  } else if (entry.typ === TypeDirectory) {
    body = <Empty icon={<IconFolder size={36}/>} title="This is a directory">
      {entry.dirFiles ? `${formatBytes(entry.dirSize, 1)} in ${entry.dirFiles.toLocaleString()} files. ` : "It contains no files. "}
      Its contents are expanded in the tree.
    </Empty>;
  } else if (entry.typ === TypeSymlink) {
    const target = resolveLink(path, entry.linkname);
    body = <Empty icon={<IconLink size={36}/>} title="Symbolic link">
      <code>{entry.linkname}</code>
      <Button mt="md" variant="light" rightIcon={<IconArrowRight size={14}/>} onClick={() => onSelect(target)}>Go to /{target}</Button>
    </Empty>;
  } else if (entry.size === 0) {
    body = <Empty icon={<IconFileText size={36}/>} title="Empty file"/>;
  } else if (previewTooLarge) {
    body = <Empty icon={<IconFileUnknown size={36}/>} title="File too large to preview">
      Automatic previews are limited to 1 MiB.
      {downloadTooLarge ? " This viewer supports downloads up to 4 MiB. Pull the image locally to inspect this file." :
        " Download the complete file to inspect it locally."}
    </Empty>;
  } else if (contents.error) {
    body = <Empty icon={<IconFileUnknown size={36}/>} title="Couldn't load this file">{contents.error.message}</Empty>;
  } else if (contents.isLoading || contents.data === undefined) {
    body = <div className="viewer-loading"><Loader size="sm"/></div>;
  } else if (looksBinary(contents.data)) {
    body = <Empty icon={<IconFileUnknown size={36}/>} title="Binary file">Download it to inspect it locally.</Empty>;
  } else {
    body = <CodeView path={path} text={contents.data} line={line} onSelectLine={line => onSelect(path, line)}/>;
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
        <Button size="xs" leftIcon={<IconDownload size={14}/>} disabled={!isFile || downloadTooLarge} loading={downloading}
                onClick={async () => {
                  const controller = new AbortController();
                  downloadRequest.current = controller;
                  setDownloading(true);
                  setDownloadError(undefined);
                  try {
                    await download(url, name, controller.signal);
                  } catch (error) {
                    if (!controller.signal.aborted) setDownloadError(error instanceof Error ? error.message : "Unknown error");
                  } finally {
                    if (!controller.signal.aborted) setDownloading(false);
                  }
                }}>
          Download
        </Button>
      </div>
    </div>
    {entry && <div className="viewer-meta">
      <span>{formatBytes(entry.typ === TypeDirectory ? entry.dirSize : entry.size, 1)}</span>
      {entry.layer && <>
        <span>from layer <LayerChip digest={entry.layer}/></span>
        <span>mode {(entry.mode & 0o7777).toString(8).padStart(4, "0")}</span>
      </>}
    </div>}
    {hidden && entry && entry.typ !== TypeDirectory && <div className="hidden-notice">
      <IconEyeOff size={18} aria-hidden="true"/>
      <span><strong>Hidden file</strong> · Deleted from the final image. Contents remain in the original layer.</span>
    </div>}
    {downloadError && <div className="download-error" role="alert">
      Couldn’t download this file: {downloadError} Click Download to try again.
    </div>}
    <div className="viewer-body">{body}</div>
  </div>;
}
