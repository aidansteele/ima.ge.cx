import {MouseEvent, useState} from "react";
import {Code, Loader} from "@mantine/core";
import {formatBytes} from "../src/formatBytes";
import {DirectoryEntry, useDirectory} from "../src/imageHooks";

const TypeRegular = 48; // tar.TypeReg ('0')
const TypeDirectory = 53; // tar.TypeDir ('5')

function basename(path: string, parent: string) {
  return path.startsWith(parent) ? path.slice(parent.length) : path;
}

// Runs onClick without following the placeholder href, which would scroll to the top.
function clickHandler(onClick: () => void) {
  return (e: MouseEvent) => {
    e.preventDefault();
    onClick();
  };
}

function LayerCell({layer}: { layer: string }) {
  return <td><Code>{layer.slice(7, 15)}</Code></td>;
}

interface FileNodeProps {
  parent: string;
  dirent: DirectoryEntry;
  depth: number;
  setFocus: (path: string) => void;
}

export function FileNode({dirent, parent, depth, setFocus}: FileNodeProps) {
  return <tr>
    <td>
      <Code ml={depth * 10}>
        <a className={"fs-item fs-item-file"} href="#" onClick={clickHandler(() => setFocus(dirent.path))}>{basename(dirent.path, parent)}</a>
        {dirent.linkname.length > 0 ? ` -> ${dirent.linkname}` : ''}
      </Code>
    </td>
    <td>
      <Code>{dirent.typ === TypeRegular ? formatBytes(dirent.size) : ''}</Code>
    </td>
    <LayerCell layer={dirent.layer}/>
  </tr>;
}

interface DirectoryNodeProps {
  image: string;
  digest: string;
  parent: string;
  dirent: DirectoryEntry;
  depth: number;
  startExpanded?: boolean;
  setFocus: (path: string) => void;
}

export function DirectoryNode({image, digest, dirent, parent, depth, setFocus, startExpanded}: DirectoryNodeProps) {
  const [expanded, setExpanded] = useState(!!startExpanded);
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
      <DirectoryNode
        key={child.id}
        image={image}
        digest={digest}
        dirent={child}
        parent={dirent.path}
        depth={depth + 1}
        setFocus={setFocus}
      /> :
      <FileNode
        key={child.id}
        parent={dirent.path}
        dirent={child}
        depth={depth + 1}
        setFocus={setFocus}
      />
    )}
  </>
}
