import {useState} from "react";
import {ActionIcon, CopyButton, TextInput, Tooltip} from "@mantine/core";
import {IconCheck, IconCopy, IconSearch} from "@tabler/icons";
import {ImageConfig} from "../src/imageHooks";

export function EnvVars({config}: { config?: ImageConfig }) {
  const [filter, setFilter] = useState("");
  // Env can be null, see mcr.microsoft.com/windows/server:ltsc2022@sha256:c1ab506591d9cf636a7b37202568db62b7d1bbfcdb6181fad093660f5a6b8b15
  const env = (config?.config?.Env ?? []).map(kv => {
    const [key, ...vals] = kv.split("=");
    return {key, val: vals.join("=")};
  }).sort((a, b) => a.key.localeCompare(b.key));
  const shown = env.filter(({key, val}) => `${key}=${val}`.toLowerCase().includes(filter.toLowerCase()));

  return <div className="page-section">
    <div className="section-header">
      <h2>{env.length} environment variables</h2>
      <TextInput size="xs" icon={<IconSearch size={14}/>} placeholder="Filter" value={filter}
                 onChange={e => setFilter(e.currentTarget.value)} style={{width: 240}}/>
    </div>
    <div className="env">
      {shown.map(({key, val}) => <div className="env-row" key={key}>
        <code className="env-key">{key}</code>
        <code className="env-val">{val}</code>
        <CopyButton value={`${key}=${val}`}>
          {({copied, copy}) => <Tooltip label={copied ? "Copied" : "Copy"} withinPortal>
            <ActionIcon size="sm" variant="subtle" onClick={copy} aria-label={`Copy ${key}`}>
              {copied ? <IconCheck size={14}/> : <IconCopy size={14}/>}
            </ActionIcon>
          </Tooltip>}
        </CopyButton>
      </div>)}
      {shown.length === 0 && <div className="muted env-row">No matching variables.</div>}
    </div>
  </div>;
}
