import {useState} from "react";
import {Code, Loader} from "@mantine/core";
import {formatBytes} from "../src/formatBytes";
import {DirectoryEntry, useDirectory} from "../src/imageHooks";

interface FileNodeProps {
  image: string;
  digest: string;
  parent: string;
  dirent: DirectoryEntry;
  depth: number;
  setFocus: (path: string) => void;
}

export function FileNode({dirent, parent, depth, setFocus}: FileNodeProps) {
  let basename = dirent.path;
  if (basename.startsWith(parent)) {
    basename = basename.slice(parent.length);
  }

  const fullName = <>
    <a className={"fs-item fs-item-file"} href="#" onClick={() => setFocus(dirent.path)}>{basename}</a>
    {dirent.linkname.length > 0 ? ` -> ${dirent.linkname}` : ''}
  </>;

  return <>
    <tr>
      <td>
        <Code ml={depth * 10}>{fullName}</Code>
      </td>
      <td>
        <Code>{dirent.typ == 48 ? formatBytes(dirent.size) : ''}</Code>
      </td>
      <td>
        <Code>{dirent.layer.slice(7, 15)}</Code>
      </td>
    </tr>
  </>
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

  let basename = dirent.path;
  if (basename.startsWith(parent)) {
    basename = basename.slice(parent.length);
  }

  const layerCell = <td><Code>{dirent.layer.slice(7, 15)}</Code></td>;

  if (error) {
    return <>
      <tr>
        <td>{error}</td>
        <td>error</td>
        {layerCell}
      </tr>
    </>;
  }

  if (isLoading) {
    return <>
      <tr>
        <td><Code ml={depth * 10}>{basename}</Code> <Loader size={"xs"}/></td>
        <td></td>
        {layerCell}
      </tr>
    </>;
  }

  return <>
    <tr>
      <td><Code ml={depth * 10}><a className={"fs-item fs-item-dir"} href="#" onClick={() => setExpanded(!expanded)}>{basename}</a></Code></td>
      <td></td>
      {layerCell}
    </tr>
    {dirents.map((child) => {
      if (child.typ == 53) {
        return <DirectoryNode
          key={child.id}
          image={image}
          digest={digest}
          dirent={child}
          parent={dirent.path}
          depth={depth+1}
          setFocus={setFocus}
        />
      } else {
        return <FileNode
          key={child.id}
          image={image}
          digest={digest}
          parent={dirent.path}
          dirent={child}
          depth={depth+1}
          setFocus={setFocus}
        />;
      }
    })
    }
  </>
}
