import {CSSProperties, MouseEvent, useEffect, useRef, useState} from "react";
import {Loader} from "@mantine/core";
import {IconChevronRight, IconFile, IconFolder, IconLink} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {DirectoryEntry, useDirectory} from "../src/imageHooks";
import {hashFromPath} from "../src/useHashPath";
import {LayerChip} from "./layerChip";

export const TypeRegular = 48; // tar.TypeReg ('0')
export const TypeSymlink = 50; // tar.TypeSymlink ('2')
export const TypeDirectory = 53; // tar.TypeDir ('5')

interface TreeProps {
  image: string;
  digest: string;
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
  const {image, digest, dir, depth} = props;
  const {dirents, isLoading, error} = useDirectory(image, digest, dir, true);

  if (isLoading) {
    return <div className="tree-status" style={indent(depth)}><Loader size={12}/> Loading…</div>;
  }
  if (error) {
    return <div className="tree-status error" style={indent(depth)}>{error.message}</div>;
  }
  if (dirents.length === 0) {
    return <div className="tree-status" style={indent(depth)}>Empty directory</div>;
  }

  // Directories first, each group in the index's name order.
  const sorted = [...dirents].sort((a, b) => Number(b.typ === TypeDirectory) - Number(a.typ === TypeDirectory));

  return <>
    {sorted.map(child => child.typ === TypeDirectory ?
      <DirectoryRow {...props} key={child.id} entry={child}/> :
      <FileRow {...props} key={child.id} entry={child}/>
    )}
  </>;
}

function DirectoryRow(props: TreeProps & { dir: string, depth: number, entry: DirectoryEntry }) {
  const {entry, dir, depth, focusedPath} = props;
  const containsFocus = within(focusedPath, entry.path);
  const [expanded, setExpanded] = useState(containsFocus);

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
      <span className="tree-size"/>
      <LayerChip digest={entry.layer}/>
    </button>
    {expanded && <DirectoryChildren {...props} dir={entry.path} depth={depth + 1}/>}
  </>;
}

function FileRow({entry, dir, depth, focusedPath, scrollToPath, onSelect}: TreeProps & { dir: string, depth: number, entry: DirectoryEntry }) {
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
            className={`tree-row${selected ? " selected" : ""}`} style={indent(depth)}>
    <span className="tree-chevron-space"/>
    {symlink ? <IconLink size={16} className="tree-icon"/> : <IconFile size={16} className="tree-icon"/>}
    <span className="tree-name">
      {displayName(entry.path, dir)}
      {entry.linkname && <span className="tree-target"> → {entry.linkname}</span>}
    </span>
    <span className="tree-size">{entry.typ === TypeRegular ? formatBytes(entry.size) : ""}</span>
    <LayerChip digest={entry.layer}/>
  </a>;
}
