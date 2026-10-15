import type {GetServerSideProps, NextPage} from 'next'
import {useEffect, useState} from 'react'
import Link from "next/link";
import {Loader} from "@mantine/core";
import {Prism} from "@mantine/prism";
import {IconAlertTriangle, IconCpu} from "@tabler/icons";
import {ImageInfo, useImageInfo, useImageOptions} from "../src/imageHooks";
import {useHashPath} from "../src/useHashPath";
import {TopBar} from "../components/topBar";
import {ImageRef, platformLabel} from "../components/imageRef";
import {SummaryStrip} from "../components/summaryStrip";
import {FileTree} from "../components/fileTree";
import {FileViewer} from "../components/fileViewer";
import {Layers} from "../components/layers";
import {EnvVars} from "../components/envVars";
import {ImageIndexingView} from "../components/imageIndexingView";
import {SocialMetadata} from "../components/socialMetadata";
import {imageDescription} from "../src/imageMetadata";
import {useTheme} from "../src/theme";

const tabs = [
  {value: "files", label: "Files"},
  {value: "layers", label: "Layers"},
  {value: "env", label: "Environment"},
  {value: "config", label: "Config"},
] as const;
type Tab = typeof tabs[number]["value"];

function Workspace({info, image, digest}: { info: ImageInfo, image: string, digest: string }) {
  const [selectedPath, setSelectedPath] = useHashPath();
  // The file the page was opened with, which is scrolled into view once its directory loads.
  const [linkedPath] = useState(selectedPath);
  const [tab, setTab] = useState<Tab>("files");
  const [showTree, setShowTree] = useState(!selectedPath);

  // Shared links and browser history open the viewer on narrow screens.
  useEffect(() => setShowTree(!selectedPath), [selectedPath]);
  const selectFile = (path: string) => {
    setSelectedPath(path);
    setShowTree(false);
  };

  return <>
    <SummaryStrip info={info} image={image} digest={digest}/>
    <nav className="tabs" role="tablist">
      {tabs.map(t => <button key={t.value} role="tab" aria-selected={tab === t.value}
                             className={tab === t.value ? "tab active" : "tab"} onClick={() => setTab(t.value)}>
        {t.label}
      </button>)}
    </nav>
    <main className="workspace">
      {tab === "files" && <div className={`files ${showTree ? "show-tree" : "show-viewer"}`}>
        <nav className="file-navigation" aria-label="File browser">
          <button aria-pressed={showTree} onClick={() => setShowTree(true)}>Browse files</button>
          <button aria-pressed={!showTree} disabled={!selectedPath} onClick={() => setShowTree(false)}>View file</button>
        </nav>
        <aside className="pane tree-pane">
          <FileTree image={image} digest={digest} focusedPath={selectedPath} scrollToPath={linkedPath} onSelect={selectFile}/>
        </aside>
        <section className="pane viewer-pane">
          <FileViewer image={image} digest={digest} path={selectedPath} onSelect={selectFile}/>
        </section>
      </div>}
      {tab === "layers" && <Layers info={info}/>}
      {tab === "env" && <div className="pane"><EnvVars config={info.Config}/></div>}
      {tab === "config" && <div className="pane page-section">
        <Prism language="json" radius="md">{JSON.stringify(info.Config, null, 2)}</Prism>
      </div>}
    </main>
  </>;
}

function Message({icon, title, children}: { icon: JSX.Element, title: string, children?: React.ReactNode }) {
  return <main className="workspace"><div className="empty">{icon}<div className="empty-title">{title}</div>
    {children && <div className="empty-body">{children}</div>}</div></main>;
}

function ImageState({image, digest}: { image: string, digest: string }) {
  const {data: info, error} = useImageInfo(image, digest);

  if (error) return <Message icon={<IconAlertTriangle size={36}/>} title="Couldn't load this image">{error.message}</Message>;
  if (!info || info.Status === "PENDING") return <Message icon={<Loader/>} title="Looking up image…"/>;

  switch (info.Status) {
    case "RUNNING":
      return <ImageIndexingView image={image} info={info}/>;
    case "SUCCEEDED":
      // Keyed by digest so that switching platforms resets the page's state.
      return <Workspace key={digest} info={info} image={image} digest={digest}/>;
    case "FAILED":
      return <Message icon={<IconAlertTriangle size={36}/>} title="Indexing this image failed">
        Only images with gzip-compressed layers can be indexed.
      </Message>;
    default:
      return <Message icon={<IconAlertTriangle size={36}/>} title={`Unexpected status: ${info.Status}`}/>;
  }
}

function PlatformPicker({image}: { image: string }) {
  const {themedPath} = useTheme();
  const {data} = useImageOptions(image);
  const options = data?.Options ?? [];

  if (data?.Error) return <Message icon={<IconAlertTriangle size={36}/>} title="Couldn't find this image">{data.Error}</Message>;
  if (!data || options.length === 1) return <Message icon={<Loader/>} title="Looking up image…"/>;

  return <Message icon={<IconCpu size={36}/>} title="Choose a platform">
    <div className="platforms">
      {options.map(o => <Link key={o.Digest} className="platform" href={themedPath(`/${image}@${o.Digest}`)}>
        <span>{platformLabel(o.Platform)}</span>
        <code>{o.Digest.slice(7, 19)}</code>
      </Link>)}
    </div>
  </Message>;
}

interface ImagePageProps {
  image: string;
  digest: string;
  path: string;
  description: string;
}

const ImagePage: NextPage<ImagePageProps> = ({image, digest, path, description}) => {
  return <div className="app">
    <SocialMetadata title={`${image}${digest ? ` @ ${digest.slice(0, 19)}…` : ""} · ima.ge.cx`}
                    description={description} path={path} imageReference={image} digest={digest}/>
    <TopBar><ImageRef image={image} digest={digest}/></TopBar>
    {digest ? <ImageState image={image} digest={digest}/> : <PlatformPicker image={image}/>}
  </div>;
}

// Social crawlers don't execute the client-side API hooks. Include metadata in
// the initial HTML, but leave the interactive workspace's loading flow intact.
export const getServerSideProps: GetServerSideProps<ImagePageProps> = async ({params}) => {
  const slug = params!.slug as string[];
  const [image, digest = ""] = slug.join("/").split('@');
  return {props: {
    image,
    digest,
    path: `/${slug.map(encodeURIComponent).join("/")}`,
    description: await imageDescription(image, digest),
  }};
};

export default ImagePage
