import {CSSProperties, useState} from "react";
import {ActionIcon, CopyButton, Tooltip} from "@mantine/core";
import {IconCheck, IconCopy} from "@tabler/icons";
import {formatBytes} from "../src/formatBytes";
import {ImageInfo} from "../src/imageHooks";
import {buildSteps, formatCommand} from "../src/history";
import {LayerChip, layerColor} from "./layerChip";

const dateFormat = new Intl.DateTimeFormat(undefined, {dateStyle: "medium", timeStyle: "short"});

export function Layers({info}: { info: ImageInfo }) {
  const steps = buildSteps(info);
  const maxSize = Math.max(1, ...steps.map(s => s.layer?.size ?? 0));
  const [selected, setSelected] = useState(() => Math.max(0, steps.findIndex(s => s.layer)));
  const step = steps[selected];
  const files = step?.layer && info.Progresses?.find(p => p.Layer === step.layer!.digest)?.CompletedFiles;

  if (!step) return <div className="empty"><div className="empty-title">This image has no build history.</div></div>;

  return <div className="layers">
    <ol className="timeline pane">
      {steps.map((s, i) => <li key={i}>
        <button className={`step${s.layer ? "" : " meta"}${i === selected ? " selected" : ""}`} onClick={() => setSelected(i)}
                style={s.layer ? {"--chip": layerColor(s.layer.digest)} as CSSProperties : undefined}>
          <span className="step-dot"/>
          <span className="step-digest">{s.layer?.digest.slice(7, 15)}</span>
          <span className={`badge badge-${s.instruction.toLowerCase()}`}>{s.instruction}</span>
          <span className="step-command">{s.command}</span>
          {s.layer && <>
            <span className="step-size">{formatBytes(s.layer.size)}</span>
            <span className="step-bar"><span style={{width: `${Math.max(1.5, 100 * s.layer.size / maxSize)}%`}}/></span>
          </>}
        </button>
      </li>)}
    </ol>

    <section className="step-detail pane">
      <h2>
        {step.layer ? <>Layer <LayerChip digest={step.layer.digest}/> <span className="muted">· {formatBytes(step.layer.size, 1)}{files ? ` · ${files.toLocaleString()} files` : ""}</span></> : "Configuration step"}
      </h2>
      {!step.layer && <p className="muted">This step changes the image&apos;s configuration and adds no files.</p>}

      <div className="command">
        <span className={`badge badge-${step.instruction.toLowerCase()}`}>{step.instruction}</span>
        <pre>{formatCommand(step)}</pre>
      </div>

      <dl className="facts">
        {step.created && <><dt>Created</dt><dd>{dateFormat.format(new Date(step.created))}</dd></>}
        {step.buildArgs.length > 0 && <><dt>Build args</dt><dd><pre>{step.buildArgs.join("\n")}</pre></dd></>}
        {step.comment && <><dt>Comment</dt><dd>{step.comment}</dd></>}
        {step.layer && <>
          <dt>Digest</dt>
          <dd className="digest">
            <code>{step.layer.digest}</code>
            <CopyButton value={step.layer.digest}>
              {({copied, copy}) => <Tooltip label={copied ? "Copied" : "Copy"} withinPortal>
                <ActionIcon size="sm" variant="subtle" onClick={copy} aria-label="Copy digest">
                  {copied ? <IconCheck size={14}/> : <IconCopy size={14}/>}
                </ActionIcon>
              </Tooltip>}
            </CopyButton>
          </dd>
          <dt>Media type</dt><dd><code>{step.layer.mediaType}</code></dd>
          <dt>In the file tree</dt><dd>Files from this layer are tagged <LayerChip digest={step.layer.digest}/></dd>
        </>}
      </dl>
    </section>
  </div>;
}
