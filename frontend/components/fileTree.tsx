import React, {CSSProperties, MouseEvent, useEffect, useRef, useState} from "react";
import {Loader} from "@mantine/core";
import {IconChevronRight, IconEyeOff, IconFile, IconFolder, IconLink} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {DirectoryEntry, entrySize, useDirectory} from "../src/imageHooks";
import {hashFromPath} from "../src/useHashPath";
import {LayerChip} from "./layerChip";

export const TypeRegular = 48; // tar.TypeReg ('0')
export const TypeSymlink = 50; // tar.TypeSymlink ('2')
export const TypeDirectory = 53; // tar.TypeDir ('5')

interface TreeProps {
  image: string;
  digest: string;
  hidden: boolean;
  // The selected path, which is highlighted and whose ancestor directories are expanded.
  focusedPath: string;
  // A file to scroll into view when its row first appears (the file a link pointed at).
  scrollToPath: string;
  onSelect: (path: string) => void;
}

const indent = (depth: number): CSSProperties => ({paddingLeft: 10 + depth * 16});

function displayName(path: string, parent: string) {
  const name = path.startsWith(parent) ? path.slice(parent.length) : path;
  return name.replace(/\/$/, "");
}

// Whether path is dir or inside it. Directory paths end with "/", so "etc/"
// doesn't match "etcetera/", and a path naming the directory itself
// ("usr/bin", e.g. a symlink target) matches too.
function within(path: string, dir: string) {
  return `${path}/`.startsWith(dir);
}

export function FileTree(props: TreeProps) {
  return <div className="tree" role="tree">
    <DirectoryChildren {...props} dir="/" depth={0}/>
  </div>;
}

function DirectoryChildren(props: TreeProps & { dir: string, depth: number }) {
  const {image, digest, dir, depth, hidden} = props;
  const {dirents, isLoading, error} = useDirectory(image, digest, dir, true, hidden);

  if (isLoading) {
    return <div className="tree-status" style={indent(depth)}><Loader size={12}/> Loading…</div>;
  }
  if (error) {
    return <div className="tree-status error" style={indent(depth)}>{error.message}</div>;
  }
  if (dirents.length === 0) {
    return <div className="tree-status" style={indent(depth)}>
      {hidden && dir === "/" ? "No hidden files in this image." : "Empty directory"}
    </div>;
  }

  // Each entry's bar shows its share of this directory's total size.
  const total = dirents.reduce((n, e) => n + entrySize(e), 0);
  // Directories first, each group in the index's name order.
  const sorted = [...dirents].sort((a, b) => Number(b.typ === TypeDirectory) - Number(a.typ === TypeDirectory));

  return <>
    {sorted.map(child => child.typ === TypeDirectory ?
      <DirectoryRow {...props} key={child.id} entry={child} autoExpand={dirents.length === 1}
                    share={total ? entrySize(child) / total : 0}/> :
      <FileRow {...props} key={child.id} entry={child} share={total ? entrySize(child) / total : 0}/>
    )}
  </>;
}

type RowProps = TreeProps & { dir: string, depth: number, entry: DirectoryEntry, share: number };

function Size({entry, share}: { entry: DirectoryEntry, share: number }) {
  const size = entrySize(entry);
  const title = entry.typ === TypeDirectory ? `${size.toLocaleString()} bytes in ${entry.dirFiles.toLocaleString()} files` : `${size.toLocaleString()} bytes`;
  return <>
    <span className="tree-size" title={title}>{entry.typ === TypeRegular || entry.typ === TypeDirectory ? formatBytes(size) : ""}</span>
    <span className="tree-bar" title={`${(share * 100).toFixed(1)}% of the parent directory`}>
      {share > 0 && <span style={{width: `${Math.max(4, share * 100)}%`}}/>}
    </span>
  </>;
}

function DirectoryRow(props: RowProps & { autoExpand: boolean }) {
  const {entry, dir, depth, focusedPath, share, autoExpand} = props;
  const containsFocus = within(focusedPath, entry.path);
  // Only choose the initial state: users can still collapse an auto-opened row.
  const [expanded, setExpanded] = useState(containsFocus || autoExpand);

  // Reveal the selected file when it changes, e.g. when a link is pasted into this tab.
  useEffect(() => {
    if (containsFocus) setExpanded(true);
  }, [containsFocus]);

  return <>
    <button className="tree-row" role="treeitem" aria-expanded={expanded} style={indent(depth)}
            onClick={() => setExpanded(!expanded)}>
      <IconChevronRight size={14} className={`tree-chevron${expanded ? " open" : ""}`}/>
      <IconFolder size={16} className="tree-icon folder"/>
      <span className="tree-name">{displayName(entry.path, dir)}</span>
      <Size entry={entry} share={share}/>
      {entry.layer && <LayerChip digest={entry.layer}/>}
    </button>
    {expanded && <DirectoryChildren {...props} dir={entry.path} depth={depth + 1}/>}
  </>;
}

function FileRow({entry, dir, depth, focusedPath, scrollToPath, onSelect, share, hidden}: RowProps) {
  const row = useRef<HTMLAnchorElement>(null);
  const selected = entry.path === focusedPath;
  const symlink = entry.typ === TypeSymlink;

  useEffect(() => {
    if (entry.path === scrollToPath) row.current?.scrollIntoView({block: "center"});
  }, [entry.path, scrollToPath]);

  // Follows onSelect instead of the href, so selecting a file doesn't add a
  // history entry. The href still lets the link be copied or opened in a new tab.
  const onClick = (e: MouseEvent) => {
    e.preventDefault();
    onSelect(entry.path);
  };

  return <a ref={row} href={hashFromPath(entry.path)} onClick={onClick} role="treeitem" aria-selected={selected}
            className={`tree-row${selected ? " selected" : ""}${hidden ? " hidden-file" : ""}`} style={indent(depth)}
            title={hidden ? "Deleted from the final image; retained in its original layer" : undefined}>
    <span className="tree-chevron-space"/>
    {hidden ? <IconEyeOff size={16} className="tree-icon"/> :
      symlink ? <IconLink size={16} className="tree-icon"/> : <IconFile size={16} className="tree-icon"/>}
    <span className="tree-name">
      {displayName(entry.path, dir)}
      {entry.linkname && <span className="tree-target"> → {entry.linkname}</span>}
    </span>
    <Size entry={entry} share={share}/>
    <LayerChip digest={entry.layer}/>
  </a>;
}
