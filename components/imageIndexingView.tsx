import {formatBytes} from "../src/formatBytes";
import {Progress, Table} from "@mantine/core";
import {ImageInfo} from "../src/imageHooks";

// Returns completed as a percentage of total, or 0 when total isn't known yet.
function percent(completed: number, total: number) {
  return total > 0 ? 100 * completed / total : 0;
}

export function ImageIndexingView({info}: { info: ImageInfo }) {
  // Copy before sorting: info is SWR's cached data and must not be mutated.
  const progresses = [...(info.Progresses ?? [])].sort((a, b) => b.TotalBytes - a.TotalBytes);

  const completed = info.CompletedSize ?? 0;
  const total = info.TotalSize ?? 0;

  return <div>
    <p>
      Indexing image for the first time. This only needs to happen once.
      Status: <strong>{formatBytes(completed, 2)}</strong> indexed
      of <strong>{formatBytes(total, 2)}</strong> total.
    </p>

    <Progress value={percent(completed, total)} size={"xl"} radius={"xl"} animate/>

    <Table>
      <thead>
      <tr>
        <th>Layer</th>
        <th>Size</th>
        <th>Progress</th>
      </tr>
      </thead>
      <tbody>
      {progresses.map(lp => <tr key={lp.Layer}>
        <td>{lp.Layer.slice(7, 15)}</td>
        <td>{formatBytes(lp.TotalBytes, 2)}</td>
        <td>
          <Progress color={lp.CompletedBytes === lp.TotalBytes ? 'green' : 'blue'}
                    value={percent(lp.CompletedBytes, lp.TotalBytes)}/>
        </td>
      </tr>)}
      </tbody>
    </Table>
  </div>;
}
