import {ReactNode} from "react";
import Link from "next/link";
import {IconBrandGithub} from "@tabler/icons";

function Logo() {
  return <svg width="22" height="22" viewBox="0 0 24 24" fill="none" aria-hidden>
    <path d="M12 2.5 20.5 7.25v9.5L12 21.5 3.5 16.75v-9.5L12 2.5Z" stroke="var(--accent)" strokeWidth="1.8" strokeLinejoin="round"/>
    <path d="M3.8 7.4 12 12l8.2-4.6M12 12v9.2" stroke="var(--accent)" strokeWidth="1.8" strokeLinejoin="round"/>
  </svg>;
}

export function TopBar({children, beta}: { children?: ReactNode, beta?: boolean }) {
  return <header className="topbar">
    <Link href="/" className="wordmark">
      <Logo/>
      <span>ima.ge.cx</span>
      {beta && <span className="beta">beta</span>}
    </Link>
    <div className="topbar-center">{children}</div>
    <a className="icon-link" href="https://github.com/aidansteele/ima.ge.cx" aria-label="GitHub repository">
      <IconBrandGithub size={20}/>
    </a>
  </header>;
}
