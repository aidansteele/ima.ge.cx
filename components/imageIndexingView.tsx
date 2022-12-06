import {formatBytes} from "../src/formatBytes";
import {Progress, Table} from "@mantine/core";

export function ImageIndexingView({info}: { info: any }) {
  const progresses = info.Progresses ?? [];
  progresses.sort((a: any, b: any) => b.TotalBytes - a.TotalBytes);

  const completed = info.CompletedSize || 0;
  const total = info.TotalSize;
  const percent = new Intl.NumberFormat("en-US", {style: "percent"}).format(completed / total);
  const completedFiles = progresses.reduce((acc: number, lp: any) => acc + lp.CompletedFiles, 0);

  return <div>
    <p>
      Indexing image for the first time. This only needs to happen once.
      Status: <strong>{formatBytes(completed, 2)}</strong> indexed
      of <strong>{formatBytes(total, 2)}</strong> total.
    </p>

    <Progress value={100 * completed / total} size={"xl"} radius={"xl"} animate/>

    <Table>
      <thead>
      <tr>
        <th>Layer</th>
        <th>Size</th>
        <th>Progress</th>
      </tr>
      </thead>
      <tbody>
      {progresses.map((lp: any) => <tr key={lp.Layer}>
        <td>{lp.Layer.slice(7, 15)}</td>
        <td>{formatBytes(lp.TotalBytes, 2)}</td>
        <td>
          <Progress color={lp.CompletedBytes == lp.TotalBytes ? 'green' : 'blue'}
                    value={100 * lp.CompletedBytes / lp.TotalBytes}/>
        </td>
      </tr>)}
      </tbody>
    </Table>
  </div>;
}
