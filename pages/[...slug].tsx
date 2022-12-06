import type {NextPage} from 'next'
import {useEffect, useState} from 'react'
import {useRouter} from "next/router";
import {
  ActionIcon,
  AppShell,
  Code,
  Container,
  Flex, Grid,
  Group,
  Header,
  Loader,
  Paper,
  SimpleGrid, Space,
  Table,
  Tabs,
  Text
} from "@mantine/core";
import Head from "next/head";
import {Prism} from "@mantine/prism";
import {Layers} from "../components/layers";
import {DirectoryNode} from "../components/node";
import {DirectoryEntry, useFileContents, useImageInfo} from "../src/imageHooks";
import {ImageIndexingView} from "../components/imageIndexingView";
import {ImageSelector} from "../components/imageSelector";
import {EnvVars} from "../components/envVars";
import {IconBrandGithub, IconBrandTwitter} from "@tabler/icons";

function FocusedContent({image, digest, path}: { image: string, digest: string, path: string }) {
  const { data, isLoading, error } = useFileContents(image, digest, path);

  if (!path) {
    return <Paper mt={"lg"} shadow="xs" p="md" withBorder>
      Select a file on the left to view its contents here.
    </Paper>
  }

  if (error) {
    return <div>oh no an error occurred</div>;
  }

  if (isLoading) {
    return <Loader />
  }

  return <Code block>{data}</Code>;
}

const ImageView = ({info, image, digest}: { info: any, image: string, digest: string }) => {
  const [selectedFile, setSelectedFile] = useState("");

  const rootDirent: DirectoryEntry = {
    id: `root-${digest}`,
    path: '/',
    size: 0,
    typ: 53,
    linkname: '',
    deletion: false,
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
                <DirectoryNode image={image} digest={digest} parent={""} dirent={rootDirent} depth={0} startExpanded={true} setFocus={setSelectedFile}/>
              </tbody>
            </Table>
          </div>
          <div>
            <FocusedContent image={image} digest={digest} path={selectedFile} />
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
  const {data: info, isLoading, error} = useImageInfo(image, digest);
  if (error) return <div>failed to load fully qualified view</div>;
  if (isLoading) return <Loader/>;

  switch (info.Status) {
    case "PENDING":
      return <Loader/>;
    case "RUNNING":
      return <ImageIndexingView info={info}/>;
    case "SUCCEEDED":
      return <ImageView info={info} image={image} digest={digest}/>;
    case "FAILED":
      return <div>fqv failed {JSON.stringify(info)}</div>;
    default:
      return <div>fqv default</div>
  }
}

const Home: NextPage = () => {
  const router = useRouter();

  const [image, setImage] = useState("");
  const [digest, setDigest] = useState("")

  useEffect(() => {
    if (!router.isReady) return;
    const q = router.query.slug as string[];
    const [image, digest] = q.join("/").split('@');
    setImage(image);
    setDigest(digest);
  }, [router.isReady, router.query]);

  if (image === "") {
    return <div>no image specified</div>;
  } else {
    return <AppShell
      padding="md"
      header={<Header height={90} p="xs">
        <Grid grow>
          <Grid.Col span={6}>
            <ImageSelector image={image} digest={digest}/>
          </Grid.Col>
          <Grid.Col span={5}></Grid.Col>
          <Grid.Col span={1}>
            <a href="https://github.com/aidansteele/ima.ge.cx"><ActionIcon color="blue" variant="light"><IconBrandGithub size={24} /></ActionIcon></a>
          </Grid.Col>
        </Grid>
      </Header>}
      styles={(theme) => ({
        main: {backgroundColor: theme.colorScheme === 'dark' ? theme.colors.dark[8] : theme.colors.gray[0]},
      })}
    >
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
    </AppShell>
  }
}

export default Home
