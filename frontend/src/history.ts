import {ImageInfo} from "./imageHooks";

export interface BuildStep {
  instruction: string;
  command: string;
  buildArgs: string[];
  created?: string;
  comment?: string;
  layer?: { digest: string, size: number, mediaType: string };
}

const instructions = new Set([
  "ADD", "ARG", "CMD", "COPY", "ENTRYPOINT", "ENV", "EXPOSE", "HEALTHCHECK", "LABEL",
  "MAINTAINER", "ONBUILD", "RUN", "SHELL", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR",
]);

// RUN steps with build args are recorded as "|<count> ARG1=a ARG2=b <command>".
function stripBuildArgs(command: string): [string, string[]] {
  if (!command.startsWith("|")) return [command, []];
  const [prefix, ...words] = command.split(" ");
  const count = parseInt(prefix.slice(1));
  return [words.slice(count).join(" "), words.slice(0, count)];
}

// Turns a history entry's created_by into a Dockerfile instruction. Handles
// both the legacy builder ("/bin/sh -c #(nop) ENV ...", "/bin/sh -c apt-get ...")
// and BuildKit ("RUN /bin/sh -c apt-get ... # buildkit", "ENV ...").
export function parseCreatedBy(createdBy?: string): Pick<BuildStep, "instruction" | "command" | "buildArgs"> {
  if (!createdBy) return {instruction: "?", command: "(not recorded)", buildArgs: []};

  let [rest, buildArgs] = stripBuildArgs(createdBy.trim().replace(/\s+# buildkit$/, ""));
  const nop = rest.match(/^\/bin\/sh -c #\(nop\)\s*([\s\S]*)$/);
  rest = nop ? nop[1].trim() : rest.replace(/^\/bin\/sh -c /, "");

  let instruction = "RUN";
  const word = rest.split(/\s/, 1)[0];
  if (instructions.has(word)) {
    instruction = word;
    rest = rest.slice(word.length).trim();
    if (instruction === "RUN") {
      let args;
      [rest, args] = stripBuildArgs(rest);
      buildArgs = buildArgs.concat(args);
      rest = rest.replace(/^\/bin\/sh -c /, "");
    }
  }

  return {instruction, command: rest, buildArgs};
}

// The image's build steps, newest first. History entries that aren't empty
// layers correspond, in order, to the manifest's layers.
export function buildSteps(info: ImageInfo): BuildStep[] {
  const layers = info.Manifest?.layers ?? [];
  const history = info.Config?.history ?? [];
  // History is optional. A partial history cannot safely identify which layer
  // each command produced; keep every manifest layer inspectable without guessing.
  if (history.filter(h => !h.empty_layer).length !== layers.length) {
    return layers.map(layer => ({
      instruction: "?", command: "(build history missing or incomplete)", buildArgs: [], layer,
    })).reverse();
  }
  let layerIdx = 0;

  return history.map(h => ({
    ...parseCreatedBy(h.created_by),
    created: h.created,
    comment: h.comment,
    layer: h.empty_layer ? undefined : layers[layerIdx++],
  })).reverse();
}

// Puts each "&&" of a long RUN command on its own line.
export function formatCommand(step: BuildStep) {
  const command = step.command.replace(/\s+/g, " ");
  return step.instruction === "RUN" ? command.replace(/\s*&&\s*/g, " \\\n    && ") : command;
}
