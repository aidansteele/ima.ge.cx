import {useCallback, useEffect, useState} from "react";

// The selected file and optional 1-based line live in the URL hash,
// e.g. /ruby@sha256:...#/etc/passwd:L42. Parse the suffix before decoding so
// encoded colons in file names cannot be mistaken for line numbers.

function selectionFromHash(): {path: string, line?: number} {
  const hash = typeof window === "undefined" ? "" : window.location.hash.replace(/^#\/?/, '');
  const suffix = hash.match(/:L([^/]*)$/);
  const number = suffix && /^\d+$/.test(suffix[1]) ? Number(suffix[1]) : 0;
  const line = Number.isSafeInteger(number) && number > 0 ? number : undefined;
  const path = suffix ? hash.slice(0, suffix.index) : hash;
  try {
    return {path: decodeURIComponent(path), line};
  } catch {
    return {path, line};
  }
}

export function hashFromPath(path: string, line?: number) {
  const suffix = line !== undefined && Number.isSafeInteger(line) && line > 0 ? `:L${line}` : "";
  return `#/${encodeURIComponent(path).replaceAll('%2F', '/')}${suffix}`;
}

function hiddenFromUrl() {
  return typeof window !== "undefined" && new URLSearchParams(window.location.search).get("hidden") === "true";
}

export function useHashPath(): [string, (path: string, line?: number) => void, boolean, (hidden: boolean) => void, number | undefined] {
  const [{path, line}, setSelection] = useState(selectionFromHash);
  const [hidden, setHidden] = useState(hiddenFromUrl);

  useEffect(() => {
    const onHashChange = () => {
      setSelection(selectionFromHash());
      setHidden(hiddenFromUrl());
    };
    window.addEventListener('hashchange', onHashChange);
    window.addEventListener('popstate', onHashChange);
    return () => {
      window.removeEventListener('hashchange', onHashChange);
      window.removeEventListener('popstate', onHashChange);
    };
  }, []);

  // Replace rather than push, so that viewing files doesn't fill the history.
  const selectPath = useCallback((path: string, line?: number) => {
    const url = new URL(window.location.href);
    url.hash = hashFromPath(path, line);
    // Next's Pages Router restores `as` on Back/Forward. Keep its cached URL
    // in sync while retaining the rest of the router's history entry.
    const state = {...window.history.state, as: `${url.pathname}${url.search}${url.hash}`};
    window.history.replaceState(state, "", url);
    // replaceState does not emit hashchange.
    setSelection(selectionFromHash());
  }, []);

  const selectHidden = useCallback((hidden: boolean) => {
    const url = new URL(window.location.href);
    if (hidden) url.searchParams.set("hidden", "true");
    else url.searchParams.delete("hidden");
    // A file belongs to one view, so clear selection when changing views.
    url.hash = "";
    window.history.replaceState(window.history.state, "", url);
    setSelection({path: ""});
    setHidden(hidden);
  }, []);

  return [path, selectPath, hidden, selectHidden, line];
}
