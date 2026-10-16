import {ActionIcon, CopyButton, Tooltip} from "@mantine/core";
import {IconBox, IconCheck, IconCopy, IconFiles, IconFingerprint, IconStack2} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {entryFiles, entrySize, ImageInfo, useDirectory} from "../src/imageHooks";

const statistic = new Intl.NumberFormat("en", {maximumSignificantDigits: 3});

function Stat({icon, value, label}: { icon: JSX.Element, value: string, label: string }) {
  return <div className="stat">
    {icon}
    <div>
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
    </div>
  </div>;
}

export function SummaryStrip({info, image, digest}: { info: ImageInfo, image: string, digest: string }) {
  // The file tree lists the root too, so this reuses its cached response.
  const root = useDirectory(image, digest, "/", true).dirents;
  const size = root.reduce((n, e) => n + entrySize(e), 0);
  const files = root.reduce((n, e) => n + entryFiles(e), 0);
  const compressedGB = typeof info.TotalSize === "number" && Number.isFinite(info.TotalSize) && info.TotalSize >= 0
    ? info.TotalSize / 1e9 : undefined;
  const duration = info.Status === "SUCCEEDED" && typeof info.DurationSeconds === "number"
    && Number.isFinite(info.DurationSeconds) && info.DurationSeconds > 0 ? info.DurationSeconds : undefined;
  const throughput = duration !== undefined && compressedGB !== undefined ? compressedGB * 1e3 / duration : undefined;

  return <div className="summary">
    <Stat icon={<IconBox size={20}/>} value={compressedGB === undefined ? "unknown" : `${statistic.format(compressedGB)} GB`} label="compressed size"/>
    {size > 0 && <Stat icon={<IconFiles size={20}/>} value={formatBytes(size, 1)} label={`in ${files.toLocaleString()} files`}/>}
    <Stat icon={<IconStack2 size={20}/>} value={String(info.Manifest?.layers?.length ?? 0)} label="layers"/>
    <div className="stat">
      <span className="status-dot"/>
      <div>
        <div className="stat-value">{duration === undefined ? "Indexed" : `Indexed in ${statistic.format(duration)} s`}</div>
        {throughput !== undefined && <div className="stat-label">{statistic.format(throughput)} MB/s</div>}
      </div>
    </div>
    <div className="summary-digest">
      <IconFingerprint size={16}/>
      <code>{digest.slice(0, 19)}…</code>
      <CopyButton value={digest}>
        {({copied, copy}) => <Tooltip label={copied ? "Copied" : "Copy digest"} withinPortal>
          <ActionIcon size="sm" variant="subtle" onClick={copy} aria-label="Copy digest">
            {copied ? <IconCheck size={14}/> : <IconCopy size={14}/>}
          </ActionIcon>
        </Tooltip>}
      </CopyButton>
    </div>
  </div>;
}
