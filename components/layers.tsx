import {Table} from "@mantine/core";
import {formatBytes} from "../src/formatBytes";
import {ImageInfo} from "../src/imageHooks";

// Turns a history entry's created_by into a Dockerfile-like command and its build args.
function parseCreatedBy(createdBy = "# BUG see https://github.com/aidansteele/ima.ge.cx/issues/9") {
  let cmd = createdBy;
  let buildArgs: string[] = [];

  // RUN instructions with build args are recorded as "|<count> ARG1=a ARG2=b /bin/sh -c ..."
  if (createdBy.startsWith("|")) {
    const [prefix, ...rest] = createdBy.split(" ");
    const count = parseInt(prefix.slice(1));
    buildArgs = rest.slice(0, count);
    cmd = rest.slice(count).join(" ");
  }

  cmd = cmd.replace(/^\/bin\/sh -c /, '');
  cmd = `RUN ${cmd}`.replace(/^RUN #\(nop\)/, '').trim();
  cmd = cmd.replaceAll(/\s+/g, ' ');
  cmd = cmd.replaceAll(/&&\s+/g, "&&\n  ");

  return {cmd, buildArgs};
}

export function Layers({info}: { info: ImageInfo }) {
  const layers = info.Manifest?.layers ?? [];
  let layerIdx = 0;

  // History entries that aren't empty layers correspond, in order, to the manifest's layers.
  const rows = (info.Config?.history ?? []).map(h => ({
    layer: h.empty_layer ? undefined : layers[layerIdx++],
    ...parseCreatedBy(h.created_by),
  })).reverse();

  return <Table striped>
    <thead>
    <tr>
      <th>Layer ID</th>
      <th>Size</th>
      <th>Command</th>
      <th>Build args</th>
    </tr>
    </thead>
    <tbody>
    {rows.map(({layer, cmd, buildArgs}, idx) => <tr key={idx}>
      <td style={{verticalAlign: 'top'}}>
        <pre>{layer ? layer.digest.slice(7, 15) : ""}</pre>
      </td>
      <td style={{verticalAlign: 'top', textAlign: 'right'}}>
        <pre>{layer ? formatBytes(layer.size) : ""}</pre>
      </td>
      <td style={{verticalAlign: 'top'}}>
        <pre>{cmd}</pre>
      </td>
      <td style={{verticalAlign: 'top'}}>
        <pre>{buildArgs.join("\n")}</pre>
      </td>
    </tr>)}
    </tbody>
  </Table>;
}
