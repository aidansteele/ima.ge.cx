import type {NextPage} from 'next'
import {useState} from 'react'
import {useRouter} from "next/router";
import {Button, Code, CopyButton, Group, Loader, Paper, SimpleGrid, Table, Tabs, Text} from "@mantine/core";
import {IconCheck, IconLink} from "@tabler/icons";
import Head from "next/head";
import {Prism} from "@mantine/prism";
import {Layers} from "../components/layers";
import {DirectoryNode} from "../components/node";
import {DirectoryEntry, ImageInfo, useFileContents, useImageInfo} from "../src/imageHooks";
import {ImageIndexingView} from "../components/imageIndexingView";
import {ImageSelector} from "../components/imageSelector";
import {EnvVars} from "../components/envVars";
import {SiteShell} from "../components/siteShell";
import {useHashPath} from "../src/useHashPath";

function FocusedContent({image, digest, path}: { image: string, digest: string, path: string }) {
  const {data, isLoading, error} = useFileContents(image, digest, path);

  if (!path) {
    return <Paper mt={"lg"} shadow="xs" p="md" withBorder>
      Select a file on the left to view its contents here.
    </Paper>
  }

  let content;
  if (error) {
    content = <Text color="red">Failed to load {path}: {error.message}</Text>;
  } else if (isLoading) {
    content = <Loader/>;
  } else {
    content = <Code block>{data}</Code>;
  }

  return <>
    <Group position="apart" my="xs" noWrap>
      <Code>/{path}</Code>
      <CopyButton value={window.location.href}>
        {({copied, copy}) => <Button
          size="xs"
          variant="light"
          color={copied ? "teal" : "blue"}
          leftIcon={copied ? <IconCheck size={14}/> : <IconLink size={14}/>}
          onClick={copy}
        >
          {copied ? "Copied" : "Copy link"}
        </Button>}
      </CopyButton>
    </Group>
    {content}
  </>;
}

// Keeps the selected file's contents in view while scrolling through a long file tree.
const stickyBelowHeader = {
  position: 'sticky',
  top: 'calc(var(--mantine-header-height, 90px) + 16px)',
  alignSelf: 'start',
  maxHeight: 'calc(100vh - var(--mantine-header-height, 90px) - 32px)',
  overflow: 'auto',
} as const;

function ImageView({info, image, digest}: { info: ImageInfo, image: string, digest: string }) {
  const [selectedFile, setSelectedFile] = useHashPath();
  // The file the page was opened with, which is scrolled into view once its directory loads.
  const [linkedFile] = useState(selectedFile);

  const rootDirent: DirectoryEntry = {
    id: `root-${digest}`,
    path: '/',
    size: 0,
    typ: 53,
    linkname: '',
    layer: ''
  }

  return (
    <Tabs defaultValue={"filesystem"}>
      <Tabs.List>
        <Tabs.Tab value={"filesystem"}>File system</Tabs.Tab>
        <Tabs.Tab value={"envvars"}>Environment variables</Tabs.Tab>
        <Tabs.Tab value={"layers"}>Layers</Tabs.Tab>
        <Tabs.Tab value={"config"}>Raw Config</Tabs.Tab>
      </Tabs.List>

      <Tabs.Panel value={"filesystem"}>
        <SimpleGrid cols={2}>
          <div>
            <Table verticalSpacing={0} striped>
              <thead>
              <tr>
                <th>Path</th>
                <th>Size</th>
                <th>Layer</th>
              </tr>
              </thead>
              <tbody>
                <DirectoryNode image={image} digest={digest} parent={""} dirent={rootDirent} depth={0} startExpanded={true}
                               focusedPath={selectedFile} scrollToPath={linkedFile} setFocus={setSelectedFile}/>
              </tbody>
            </Table>
          </div>
          <div style={stickyBelowHeader}>
            <FocusedContent image={image} digest={digest} path={selectedFile}/>
          </div>
        </SimpleGrid>
      </Tabs.Panel>

      <Tabs.Panel value={"envvars"}>
        <EnvVars config={info.Config}/>
      </Tabs.Panel>

      <Tabs.Panel value={"layers"}>
        <Layers info={info}/>
      </Tabs.Panel>

      <Tabs.Panel value={"config"}>
        <Prism language={"json"}>
          {JSON.stringify(info.Config, null, 2)}
        </Prism>
      </Tabs.Panel>
    </Tabs>
  )
}

function FullyQualifiedView({image, digest}: { image: string, digest: string }) {
  const {data: info, error} = useImageInfo(image, digest);
  if (error) return <Text color="red">Failed to load image: {error.message}</Text>;
  if (!info) return <Loader/>;

  switch (info.Status) {
    case "PENDING":
      return <Loader/>;
    case "RUNNING":
      return <ImageIndexingView info={info}/>;
    case "SUCCEEDED":
      // Keyed by digest so that switching variants resets the selected file.
      return <ImageView key={digest} info={info} image={image} digest={digest}/>;
    case "FAILED":
      return <Text color="red">Indexing this image failed.</Text>;
    default:
      return <Text color="red">Unexpected image status: {info.Status}</Text>;
  }
}

const ImagePage: NextPage = () => {
  const router = useRouter();
  if (!router.isReady) return null;

  const slug = router.query.slug as string[];
  const [image, digest] = slug.join("/").split('@');

  return <SiteShell headerContent={<ImageSelector image={image} digest={digest}/>}>
    <Head>
      <title>{image}</title>
    </Head>
    {digest ?
      <FullyQualifiedView image={image} digest={digest}/> :
      <Paper shadow="xs" p="md" withBorder>
        <Text>
          Select an image from the above dropdown to view its content and metadata.
        </Text>
      </Paper>
    }
  </SiteShell>
}

export default ImagePage
