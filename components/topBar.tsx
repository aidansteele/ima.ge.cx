import {ReactNode} from "react";
import Link from "next/link";
import {IconBrandGithub, IconMoon, IconSun} from "@tabler/icons";
import {useTheme} from "../src/theme";

function Logo() {
  return <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden>
    <path d="M12 2.5 20.5 7.25v9.5L12 21.5 3.5 16.75v-9.5L12 2.5Z" stroke="var(--accent)" strokeWidth="1.8" strokeLinejoin="round"/>
    <path d="M3.8 7.4 12 12l8.2-4.6M12 12v9.2" stroke="var(--accent)" strokeWidth="1.8" strokeLinejoin="round"/>
  </svg>;
}

export function TopBar({children, beta}: { children?: ReactNode, beta?: boolean }) {
  const {scheme, toggle, themedPath} = useTheme();
  const themeLabel = `Switch to ${scheme === "dark" ? "light" : "dark"} mode`;
  return <header className="topbar">
    <Link href={themedPath("/")} className="wordmark">
      <Logo/>
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
