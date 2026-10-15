import {useCallback, useEffect, useState} from "react";

// The selected file lives in the URL hash, e.g. /ruby@sha256:...#/etc/passwd,
// so that links can point at a specific file in an image.

function pathFromHash() {
  if (typeof window === "undefined") return "";
  const hash = window.location.hash.replace(/^#\/?/, '');
  try {
    return decodeURIComponent(hash);
  } catch {
    return hash;
  }
}

export function hashFromPath(path: string) {
  return `#/${encodeURIComponent(path).replaceAll('%2F', '/')}`;
}

function hiddenFromUrl() {
  return typeof window !== "undefined" && new URLSearchParams(window.location.search).get("hidden") === "true";
}

export function useHashPath(): [string, (path: string) => void, boolean, (hidden: boolean) => void] {
  const [path, setPath] = useState(pathFromHash);
  const [hidden, setHidden] = useState(hiddenFromUrl);

  useEffect(() => {
    const onHashChange = () => {
      setPath(pathFromHash());
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
  // The hashchange listener updates the state.
  const selectPath = useCallback((path: string) => window.location.replace(hashFromPath(path)), []);

  const selectHidden = useCallback((hidden: boolean) => {
    const url = new URL(window.location.href);
    if (hidden) url.searchParams.set("hidden", "true");
    else url.searchParams.delete("hidden");
    // A file belongs to one view, so clear selection when changing views.
    url.hash = "";
    window.history.replaceState(window.history.state, "", url);
    setPath("");
    setHidden(hidden);
  }, []);

  return [path, selectPath, hidden, selectHidden];
}
