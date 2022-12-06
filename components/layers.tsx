import {Table} from "@mantine/core";
import styles from "../styles/Home.module.css";
import {formatBytes} from "../src/formatBytes";

export function Layers({info}: { info: any }) {
  const layers = info.Manifest.layers;
  let layerIdx = 0;

  const useful = info.Config.history.map((h: any) => {
    let {created_by, ...rest} = h;
    const ret = rest.empty_layer ? rest : {...rest, ...layers[layerIdx++]};

    if (created_by.startsWith("|")) {
      const [prefix, ...rest] = created_by.split(" ");
      const count = parseInt(prefix.slice(1));
      ret.buildArgs = rest.slice(0, count);
      ret.cmd = rest.slice(count).join(" ");
    } else {
      ret.cmd = created_by;
    }

    ret.cmd = ret.cmd.replace(/^\/bin\/sh -c /, '');
    ret.cmd = `RUN ${ret.cmd}`.replace(/^RUN #\(nop\)/, '').trim();
    ret.cmd = ret.cmd.replaceAll(/\s+/g, ' ')
    ret.cmd = ret.cmd.replaceAll(/&&\s+/g, "&&\n  ");

    // debugger;
    return ret;
  }).reverse();

  return <>
    <Table striped>
      <thead>
      <tr>
        <th>Layer ID</th>
        <th>Size</th>
        <th>Command</th>
        <th>Build args</th>
      </tr>
      </thead>
      <tbody>
      {useful.map((u: any, idx: number) => <tr key={idx} className={styles.fsitem}>
        <td style={{verticalAlign: 'top'}}>
          <pre>{u.empty_layer ? "" : u.digest.slice(7, 15)}</pre>
        </td>
        <td style={{verticalAlign: 'top', textAlign: 'right'}}>
          <pre>{u.empty_layer ? "" : formatBytes(u.size)}</pre>
        </td>
        <td style={{verticalAlign: 'top'}}>
          <pre>{u.cmd}</pre>
        </td>
        <td style={{verticalAlign: 'top'}}>
          <pre>{(u.buildArgs || []).join("\n")}</pre>
        </td>
      </tr>)}
      </tbody>
    </Table>
  </>;
}
