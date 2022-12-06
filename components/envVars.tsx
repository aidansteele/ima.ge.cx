import {Code, Table} from "@mantine/core";
import styles from "../styles/Home.module.css";

export function EnvVars({config}: { config: any }) {
  const env: { key: string, val: string }[] = config.config.Env.map((kv: string) => {
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
