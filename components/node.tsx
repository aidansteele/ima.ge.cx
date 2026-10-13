import {MouseEvent, useEffect, useRef, useState} from "react";
import {Code, Loader} from "@mantine/core";
import {formatBytes} from "../src/formatBytes";
import {DirectoryEntry, useDirectory} from "../src/imageHooks";
import {hashFromPath} from "../src/useHashPath";

const TypeRegular = 48; // tar.TypeReg ('0')
const TypeDirectory = 53; // tar.TypeDir ('5')

function basename(path: string, parent: string) {
  return path.startsWith(parent) ? path.slice(parent.length) : path;
}

// Runs onClick instead of following the href: a placeholder "#" would scroll to the
// top, and following a file's hash link would add a history entry.
function clickHandler(onClick: () => void) {
  return (e: MouseEvent) => {
    e.preventDefault();
    onClick();
  };
}

function LayerCell({layer}: { layer: string }) {
  return <td><Code>{layer.slice(7, 15)}</Code></td>;
}

interface FocusProps {
  // The selected file, which is highlighted and whose ancestor directories are expanded.
  focusedPath: string;
  // A file to scroll into view when its row first appears (the file a link pointed at).
  scrollToPath: string;
  setFocus: (path: string) => void;
}

interface FileNodeProps extends FocusProps {
  parent: string;
  dirent: DirectoryEntry;
  depth: number;
}

export function FileNode({dirent, parent, depth, focusedPath, scrollToPath, setFocus}: FileNodeProps) {
  const row = useRef<HTMLTableRowElement>(null);

  useEffect(() => {
    if (dirent.path === scrollToPath) {
      row.current?.scrollIntoView({block: "center"});
    }
  }, [dirent.path, scrollToPath]);

  return <tr ref={row}>
    <td>
      <Code ml={depth * 10} {...(dirent.path === focusedPath && {color: "blue", sx: {fontWeight: 700}})}>
        <a className={"fs-item fs-item-file"} href={hashFromPath(dirent.path)} onClick={clickHandler(() => setFocus(dirent.path))}>{basename(dirent.path, parent)}</a>
        {dirent.linkname.length > 0 ? ` -> ${dirent.linkname}` : ''}
      </Code>
    </td>
    <td>
      <Code>{dirent.typ === TypeRegular ? formatBytes(dirent.size) : ''}</Code>
    </td>
    <LayerCell layer={dirent.layer}/>
  </tr>;
}

interface DirectoryNodeProps extends FocusProps {
  image: string;
  digest: string;
  parent: string;
  dirent: DirectoryEntry;
  depth: number;
  startExpanded?: boolean;
}

export function DirectoryNode(props: DirectoryNodeProps) {
  const {image, digest, dirent, parent, depth, startExpanded, focusedPath} = props;
  // Directory paths end with "/", so this doesn't match siblings like "etcetera/".
  const containsFocus = focusedPath.startsWith(dirent.path);
  const [expanded, setExpanded] = useState(!!startExpanded || containsFocus);

  // Reveal the selected file when it changes, e.g. when a link is pasted into this tab.
  useEffect(() => {
    if (containsFocus) setExpanded(true);
  }, [containsFocus]);

  const {dirents, isLoading, error} = useDirectory(image, digest, dirent.path, expanded);
  const name = basename(dirent.path, parent);

  if (error) {
    return <tr>
      <td><Code ml={depth * 10}>{name}</Code> {error.message}</td>
      <td>error</td>
      <LayerCell layer={dirent.layer}/>
    </tr>;
  }

  if (isLoading) {
    return <tr>
      <td><Code ml={depth * 10}>{name}</Code> <Loader size={"xs"}/></td>
      <td></td>
      <LayerCell layer={dirent.layer}/>
    </tr>;
  }

  return <>
    <tr>
      <td><Code ml={depth * 10}><a className={"fs-item fs-item-dir"} href="#" onClick={clickHandler(() => setExpanded(!expanded))}>{name}</a></Code></td>
      <td></td>
      <LayerCell layer={dirent.layer}/>
    </tr>
    {dirents.map((child) => child.typ === TypeDirectory ?
      <DirectoryNode {...props} key={child.id} startExpanded={false} dirent={child} parent={dirent.path} depth={depth + 1}/> :
      <FileNode {...props} key={child.id} dirent={child} parent={dirent.path} depth={depth + 1}/>
    )}
  </>
}
