import {useCallback, useEffect, useState} from "react";

// The selected file lives in the URL hash, e.g. /ruby@sha256:...#/etc/passwd,
// so that links can point at a specific file in an image.

function pathFromHash() {
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

export function useHashPath(): [string, (path: string) => void] {
  const [path, setPath] = useState(pathFromHash);

  useEffect(() => {
    const onHashChange = () => setPath(pathFromHash());
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, []);

  // Replace rather than push, so that viewing files doesn't fill the history.
  // The hashchange listener updates the state.
  const selectPath = useCallback((path: string) => window.location.replace(hashFromPath(path)), []);

  return [path, selectPath];
}
