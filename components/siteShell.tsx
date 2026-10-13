import {ReactNode} from "react";
import {ActionIcon, AppShell, Grid, Header} from "@mantine/core";
import {IconBrandGithub} from "@tabler/icons";

export function SiteShell({headerContent, children}: { headerContent: ReactNode, children: ReactNode }) {
  return <AppShell
    padding="md"
    header={<Header height={90} p="xs">
      <Grid grow>
        <Grid.Col span={6}>
          {headerContent}
        </Grid.Col>
        <Grid.Col span={5}></Grid.Col>
        <Grid.Col span={1}>
          <ActionIcon component="a" href="https://github.com/aidansteele/ima.ge.cx" color="blue" variant="light" aria-label="GitHub repository">
            <IconBrandGithub size={24}/>
          </ActionIcon>
        </Grid.Col>
      </Grid>
    </Header>}
    styles={(theme) => ({
      main: {backgroundColor: theme.colorScheme === 'dark' ? theme.colors.dark[8] : theme.colors.gray[0]},
    })}
  >
    {children}
  </AppShell>;
}
