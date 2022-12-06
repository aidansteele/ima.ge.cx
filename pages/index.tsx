import type {NextPage} from 'next'
import {ActionIcon, AppShell, Badge, Container, Footer, Grid, Header, Paper, SimpleGrid, Text} from "@mantine/core";
import Head from "next/head";
import Link from "next/link";
import {ImageSelector} from "../components/imageSelector";
import {IconBrandGithub} from "@tabler/icons";

const Home: NextPage = () => {
  const exampleImages = [
    "ubuntu",
    "golang:1.19",
    "jetbrains/teamcity-server:2022.10",
    "mcr.microsoft.com/dotnet/sdk:6.0",
    "ruby@sha256:030eaf505bc7d39bfe1c485762603a95b5d43277f98cd3334d85e84547073abb"
  ]

  return <AppShell
    padding="md"
    header={<Header height={90} p="xs">
      <Grid grow>
        <Grid.Col span={6}>
          <h1>ima.ge.cx <Badge color={"red"}>BETA</Badge> </h1>
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
  </AppShell>
}

export default Home
