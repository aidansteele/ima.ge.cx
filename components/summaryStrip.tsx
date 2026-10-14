import {ActionIcon, CopyButton, Tooltip} from "@mantine/core";
import {IconBox, IconCheck, IconCopy, IconCpu, IconFiles, IconFingerprint, IconStack2} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {ImageInfo} from "../src/imageHooks";

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

export function SummaryStrip({info, digest}: { info: ImageInfo, digest: string }) {
  const config = info.Config;
  const platform = [config?.os, config?.architecture, config?.variant].filter(Boolean).join("/") || "unknown";
  const files = (info.Progresses ?? []).reduce((n, p) => n + p.CompletedFiles, 0);

  return <div className="summary">
    <Stat icon={<IconBox size={20}/>} value={formatBytes(info.TotalSize ?? 0, 1)} label="compressed size"/>
    <Stat icon={<IconStack2 size={20}/>} value={String(info.Manifest?.layers?.length ?? 0)} label="layers"/>
    {files > 0 && <Stat icon={<IconFiles size={20}/>} value={files.toLocaleString()} label="entries across layers"/>}
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
