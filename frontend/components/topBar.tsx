import {ReactNode} from "react";
import Link from "next/link";
import {IconBrandGithub, IconMoon, IconSun} from "@tabler/icons";
import {useTheme} from "../src/theme";

export function TopBar({children, beta}: { children?: ReactNode, beta?: boolean }) {
  const {scheme, toggle, themedPath} = useTheme();
  const themeLabel = `Switch to ${scheme === "dark" ? "light" : "dark"} mode`;
  return <header className="topbar">
    <Link href={themedPath("/")} className="wordmark">
      <img src="/favicon.svg" width="26" height="26" alt=""/>
      <span>ima.ge.cx</span>
      {beta && <span className="beta">beta</span>}
    </Link>
    <div className="topbar-center">{children}</div>
    <div className="topbar-actions">
      <button type="button" className="icon-link" onClick={toggle} aria-label={themeLabel} title={themeLabel}>
        {scheme === "dark" ? <IconSun size={20}/> : <IconMoon size={20}/>}
      </button>
      <a className="icon-link" href="https://github.com/aidansteele/ima.ge.cx" aria-label="GitHub repository">
        <IconBrandGithub size={20}/>
      </a>
    </div>
  </header>;
}
