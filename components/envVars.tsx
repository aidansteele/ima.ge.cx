import {Code, Table} from "@mantine/core";
import styles from "../styles/Home.module.css";

export function EnvVars({config}: { config: any }) {
  const rawEnv = config.config.Env || []; // Env can be null, see mcr.microsoft.com/windows/server:ltsc2022@sha256:c1ab506591d9cf636a7b37202568db62b7d1bbfcdb6181fad093660f5a6b8b15

  const env: { key: string, val: string }[] = rawEnv.map((kv: string) => {
    const [key, ...vals] = kv.split("=");
    return {key, val: vals.join("=")};
  }).sort((a: any, b: any) => a.key.localeCompare(b.key));

  return <Table striped>
    <tbody>
    {env.map(({key, val}) => <tr key={key} className={styles.fsitem}>
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
