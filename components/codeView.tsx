import {useEffect, useRef, useState} from "react";
import {Button} from "@mantine/core";
import {IconFold, IconFoldDown} from "@tabler/icons";
import {Compartment, EditorState} from "@codemirror/state";
import {
  drawSelection, EditorView, highlightActiveLine, highlightActiveLineGutter, keymap, lineNumbers,
} from "@codemirror/view";
import {
  codeFolding, foldAll, foldGutter, foldKeymap, foldService, HighlightStyle, LanguageDescription,
  StreamLanguage, syntaxHighlighting, unfoldAll,
} from "@codemirror/language";
import {languages} from "@codemirror/language-data";
import {highlightSelectionMatches, search, searchKeymap} from "@codemirror/search";
import {tags as t} from "@lezer/highlight";

const interpreters: Record<string, string> = {
  sh: "Shell", bash: "Shell", dash: "Shell", ash: "Shell", zsh: "Shell", ksh: "Shell",
  python: "Python", python2: "Python", python3: "Python", node: "JavaScript",
  perl: "Perl", ruby: "Ruby", lua: "Lua", php: "PHP",
};

// Picks a language from the file name, then from a "#!" line for scripts without an extension.
function detectLanguage(path: string, text: string) {
  const name = path.split("/").pop() ?? "";
  const byName = LanguageDescription.matchFilename(languages, name);
  if (byName) return byName;

  const shebang = text.match(/^#!\s*(\S+)(?:\s+(\S+))?/);
  if (!shebang) return null;
  const program = (shebang[1].endsWith("/env") ? shebang[2] ?? "" : shebang[1]).split("/").pop()!;
  const language = interpreters[program] ?? interpreters[program.replace(/[\d.]+$/, "")];
  return language ? LanguageDescription.matchLanguageName(languages, language) : null;
}

function indentation(line: string) {
  return line.match(/^\s*/)![0].replace(/\t/g, "    ").length;
}

// Folds blocks of more-indented lines. Used for languages without a syntax
// tree (shell, config files, plain text), which can't fold otherwise.
const indentFolding = foldService.of((state, lineStart) => {
  const line = state.doc.lineAt(lineStart);
  if (!line.text.trim()) return null;
  const base = indentation(line.text);
  let end = -1;
  const last = Math.min(state.doc.lines, line.number + 10_000);
  for (let n = line.number + 1; n <= last; n++) {
    const next = state.doc.line(n);
    if (!next.text.trim()) continue;
    if (indentation(next.text) <= base) break;
    end = next.to;
  }
  return end > line.to ? {from: line.to, to: end} : null;
});

const highlightStyle = HighlightStyle.define([
  {tag: [t.keyword, t.modifier, t.controlKeyword, t.operatorKeyword], color: "#c4a7ff"},
  {tag: [t.string, t.special(t.string), t.character], color: "#9ce3b5"},
  {tag: [t.number, t.bool, t.null, t.atom], color: "#ffb86b"},
  {tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: "#5f6880", fontStyle: "italic"},
  {tag: [t.function(t.variableName), t.function(t.propertyName), t.macroName], color: "#82c8ff"},
  {tag: [t.definition(t.variableName), t.definition(t.function(t.variableName))], color: "#82c8ff"},
  {tag: [t.typeName, t.className, t.namespace], color: "#f7d07a"},
  {tag: [t.propertyName, t.attributeName], color: "#b8c1ff"},
  {tag: [t.tagName], color: "#ff8fa8"},
  {tag: [t.regexp, t.escape], color: "#5fd4f4"},
  {tag: [t.operator, t.punctuation, t.separator, t.bracket], color: "#97a0b5"},
  {tag: [t.meta, t.processingInstruction, t.annotation], color: "#97a0b5"},
  {tag: t.heading, color: "#e6e8ee", fontWeight: "700"},
  {tag: t.strong, fontWeight: "700"},
  {tag: t.emphasis, fontStyle: "italic"},
  {tag: [t.link, t.url], color: "#8f9cff", textDecoration: "underline"},
  {tag: t.invalid, color: "#ff8fa8"},
]);

const theme = EditorView.theme({
  "&": {
    height: "100%",
    fontSize: "13px",
    color: "var(--text)",
    backgroundColor: "var(--surface)",
    border: "1px solid var(--border-soft)",
    borderRadius: "10px",
  },
  "&.cm-focused": {outline: "none"},
  ".cm-scroller": {fontFamily: "var(--mono)", lineHeight: "1.6", borderRadius: "10px"},
  ".cm-content": {padding: "12px 0", caretColor: "transparent"},
  ".cm-gutters": {backgroundColor: "var(--surface)", color: "var(--faint)", border: "none", paddingLeft: "8px"},
  ".cm-lineNumbers .cm-gutterElement": {padding: "0 6px 0 8px"},
  ".cm-activeLine": {backgroundColor: "rgba(255, 255, 255, 0.035)"},
  ".cm-activeLineGutter": {backgroundColor: "transparent", color: "var(--muted)"},
  ".cm-foldGutter .cm-gutterElement": {width: "18px", display: "flex", alignItems: "center", justifyContent: "center"},
  ".cm-foldPlaceholder": {
    backgroundColor: "var(--accent-soft)", border: "none", color: "var(--accent)",
    borderRadius: "4px", padding: "0 6px", margin: "0 4px",
  },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": {backgroundColor: "rgba(124, 140, 255, 0.28) !important"},
  ".cm-selectionMatch": {backgroundColor: "rgba(124, 140, 255, 0.16)"},
  ".cm-searchMatch": {backgroundColor: "rgba(245, 165, 36, 0.25)", outline: "1px solid rgba(245, 165, 36, 0.5)"},
  ".cm-searchMatch-selected": {backgroundColor: "rgba(245, 165, 36, 0.5)"},
  ".cm-panels": {backgroundColor: "var(--surface-2)", color: "var(--text)", borderRadius: "10px 10px 0 0"},
  ".cm-panels-top": {borderBottom: "1px solid var(--border)"},
  ".cm-search": {display: "flex", alignItems: "center", flexWrap: "wrap", gap: "6px", padding: "8px 36px 8px 10px", fontSize: "12px", fontFamily: "var(--sans)"},
  ".cm-search br": {display: "none"},
  ".cm-search label": {display: "flex", alignItems: "center", gap: "4px", color: "var(--muted)", textTransform: "capitalize"},
  ".cm-textfield": {
    backgroundColor: "var(--bg)", border: "1px solid var(--border)", borderRadius: "6px", color: "var(--text)",
    fontFamily: "var(--mono)", fontSize: "13px", padding: "4px 8px", margin: "0", width: "220px",
  },
  ".cm-textfield:focus": {outline: "none", borderColor: "var(--accent)"},
  ".cm-button": {
    backgroundImage: "none", backgroundColor: "var(--surface)", border: "1px solid var(--border)", borderRadius: "6px",
    color: "var(--text)", fontSize: "12px", padding: "3px 10px", margin: "0", textTransform: "capitalize", cursor: "pointer",
  },
  ".cm-button:hover": {borderColor: "var(--accent)"},
  ".cm-search [name=close]": {color: "var(--muted)", fontSize: "18px", top: "6px", right: "10px", cursor: "pointer"},
}, {dark: true});

function foldMarker(open: boolean) {
  const marker = document.createElement("span");
  marker.className = `fold-marker${open ? " open" : ""}`;
  marker.title = open ? "Collapse" : "Expand";
  return marker;
}

// A read-only editor: highlighted, foldable, searchable (⌘F) and virtualised,
// so long files stay fast.
const searchShortcut = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘F" : "Ctrl+F";

export function CodeView({path, text}: { path: string, text: string }) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView>();
  const [languageName, setLanguageName] = useState<string>();

  useEffect(() => {
    const language = new Compartment();
    const editor = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: text,
        extensions: [
          EditorState.readOnly.of(true),
          lineNumbers(),
          codeFolding({placeholderText: "⋯"}),
          foldGutter({markerDOM: foldMarker}),
          highlightActiveLine(),
          highlightActiveLineGutter(),
          drawSelection(),
          search({top: true}),
          highlightSelectionMatches(),
          keymap.of([...searchKeymap, ...foldKeymap]),
          syntaxHighlighting(highlightStyle),
          theme,
          language.of(indentFolding),
        ],
      }),
    });
    view.current = editor;

    let cancelled = false;
    const description = detectLanguage(path, text);
    setLanguageName(description?.name ?? "Plain text");
    description?.load().then(support => {
      if (cancelled) return;
      // Languages with a syntax tree fold by syntax; legacy modes fold by indentation.
      const folding = support.language instanceof StreamLanguage ? indentFolding : [];
      editor.dispatch({effects: language.reconfigure([support, folding])});
    }).catch(() => {
      // Leave the file unhighlighted if its language fails to load.
    });

    return () => {
      cancelled = true;
      editor.destroy();
    };
  }, [path, text]);

  const run = (command: typeof foldAll) => () => {
    if (view.current) command(view.current);
  };

  return <div className="code-view">
    <div className="code-toolbar">
      <span className="code-language">{languageName}</span>
      <span className="code-hint">{searchShortcut} to search</span>
      <Button size="xs" compact variant="subtle" color="gray" leftIcon={<IconFold size={14}/>} onClick={run(foldAll)}>Collapse all</Button>
      <Button size="xs" compact variant="subtle" color="gray" leftIcon={<IconFoldDown size={14}/>} onClick={run(unfoldAll)}>Expand all</Button>
    </div>
    <div className="code-host" ref={host}/>
  </div>;
}
