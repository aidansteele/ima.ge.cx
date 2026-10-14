import {ActionIcon, CopyButton, Tooltip} from "@mantine/core";
import {IconBox, IconCheck, IconCopy, IconCpu, IconFiles, IconFingerprint, IconStack2} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {entryFiles, entrySize, ImageInfo, useDirectory} from "../src/imageHooks";

const relativeTime = new Intl.RelativeTimeFormat(undefined, {numeric: "auto"});

function ago(iso?: string) {
  if (!iso) return undefined;
  const days = Math.round((new Date(iso).getTime() - Date.now()) / 86_400_000);
  return days === 0 ? "today" : relativeTime.format(days, "day");
}

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
  const config = info.Config;
  const platform = [config?.os, config?.architecture, config?.variant].filter(Boolean).join("/") || "unknown";
  // The file tree lists the root too, so this reuses its cached response.
  const root = useDirectory(image, digest, "/", true).dirents;
  const size = root.reduce((n, e) => n + entrySize(e), 0);
  const files = root.reduce((n, e) => n + entryFiles(e), 0);

  return <div className="summary">
    <Stat icon={<IconBox size={20}/>} value={formatBytes(info.TotalSize ?? 0, 1)} label="compressed size"/>
    {size > 0 && <Stat icon={<IconFiles size={20}/>} value={formatBytes(size, 1)} label={`in ${files.toLocaleString()} files`}/>}
    <Stat icon={<IconStack2 size={20}/>} value={String(info.Manifest?.layers?.length ?? 0)} label="layers"/>
    <Stat icon={<IconCpu size={20}/>} value={platform} label="platform"/>
    <div className="stat">
      <span className="status-dot"/>
      <div>
        <div className="stat-value">Indexed</div>
        <div className="stat-label">{ago(info.Retrieved) ?? "ready"}</div>
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
