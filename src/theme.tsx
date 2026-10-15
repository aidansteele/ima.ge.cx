import {createContext, ReactNode, useCallback, useContext, useEffect, useState} from "react";
import {useRouter} from "next/router";

export type ColorScheme = "light" | "dark";
export function colorScheme(value: unknown): ColorScheme | undefined {
  return value === "light" || value === "dark" ? value : undefined;
}

export function themePath(path: string, theme: unknown) {
  const scheme = colorScheme(theme);
  if (!scheme) return path;
  const url = new URL(path, "https://ima.ge.cx");
  url.searchParams.set("theme", scheme);
  return `${url.pathname}${url.search}${url.hash}`;
}

// Runs before the body is painted. A shared URL wins over a saved preference;
// otherwise follow the OS. Storage can be unavailable in private browsers.
export const themeScript = `(() => {
  const valid = value => value === 'light' || value === 'dark';
  const themes = new URLSearchParams(location.search).getAll('theme');
  let theme = themes.length === 1 ? themes[0] : undefined;
  if (!valid(theme)) { try { theme = localStorage.getItem('theme'); } catch {} }
  if (!valid(theme)) theme = matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  document.documentElement.dataset.theme = theme;
})();`;

const ThemeContext = createContext({
  scheme: "dark" as ColorScheme,
  toggle: () => {},
  themedPath: (path: string) => path,
});

export const useTheme = () => useContext(ThemeContext);

export function ThemeProvider({children}: {children: ReactNode}) {
  const router = useRouter();
  const explicit = colorScheme(router.query.theme);
  const [scheme, setScheme] = useState<ColorScheme>(explicit ?? "dark");

  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: light)");
    const update = () => {
      let saved: ColorScheme | undefined;
      try { saved = colorScheme(localStorage.getItem("theme")); } catch {}
      const next = explicit ?? saved ?? (media.matches ? "light" : "dark");
      document.documentElement.dataset.theme = next;
      setScheme(next);
    };
    update();
    media.addEventListener("change", update);
    window.addEventListener("storage", update);
    return () => {
      media.removeEventListener("change", update);
      window.removeEventListener("storage", update);
    };
  }, [explicit]);

  const toggle = () => {
    const next = scheme === "dark" ? "light" : "dark";
    try { localStorage.setItem("theme", next); } catch {}
    document.documentElement.dataset.theme = next;
    setScheme(next);
    // Keep the file hash and workspace state, and make copied links use the
    // selected social-card palette without requiring crawlers to run JavaScript.
    const path = `${location.pathname}${location.search}${location.hash}`;
    void router.replace(themePath(path, next), undefined, {shallow: true, scroll: false});
  };
  const themedPath = useCallback((path: string) => themePath(path, explicit), [explicit]);

  return <ThemeContext.Provider value={{scheme, toggle, themedPath}}>{children}</ThemeContext.Provider>;
}
