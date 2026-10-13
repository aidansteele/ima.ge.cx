import {Code, Table} from "@mantine/core";
import {ImageConfig} from "../src/imageHooks";

export function EnvVars({config}: { config?: ImageConfig }) {
  // Env can be null, see mcr.microsoft.com/windows/server:ltsc2022@sha256:c1ab506591d9cf636a7b37202568db62b7d1bbfcdb6181fad093660f5a6b8b15
  const rawEnv = config?.config?.Env ?? [];

  const env = rawEnv.map(kv => {
    const [key, ...vals] = kv.split("=");
    return {key, val: vals.join("=")};
  }).sort((a, b) => a.key.localeCompare(b.key));

  return <Table striped>
    <tbody>
    {env.map(({key, val}) => <tr key={key}>
      <td>
        <Code>{key}</Code>
      </td>
      <td>
        <Code>{val}</Code>
      </td>
    </tr>)}
    </tbody>
  </Table>;
}
