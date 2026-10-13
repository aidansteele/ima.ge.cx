import type {NextPage} from 'next'
import {Badge, Container, Paper, Text} from "@mantine/core";
import Head from "next/head";
import Link from "next/link";
import {SiteShell} from "../components/siteShell";

const exampleImages = [
  "ubuntu",
  "golang:1.19",
  "jetbrains/teamcity-server:2022.10",
  "mcr.microsoft.com/dotnet/sdk:6.0",
  "ruby@sha256:030eaf505bc7d39bfe1c485762603a95b5d43277f98cd3334d85e84547073abb"
];

const Home: NextPage = () => {
  return <SiteShell headerContent={<h1>ima.ge.cx <Badge color={"red"}>BETA</Badge></h1>}>
    <Head>
      <title>ima.ge.cx</title>
    </Head>
    <Container>
      <Paper shadow="xs" p="md" withBorder>
        <Text>
          ima.ge.cx is a site that allows you to inspect the contents of Docker
          images. You just specify the image in the path, for example:
        </Text>
        <ul>
          {exampleImages.map(ex => <li key={ex}><Link href={`/${ex}`}>ima.ge.cx/{ex}</Link></li>)}
        </ul>
        <Text>
          The frontend needs a lot of polish. Help
          is very much appreciated in that regard. Click the GitHub link in the
          top-right corner if you can help fix this mess.
        </Text>
      </Paper>
    </Container>
  </SiteShell>
}

export default Home
