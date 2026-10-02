import { Link, Outlet } from "@tanstack/react-router";

export function Root() {
  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">Yulai</div>
        <nav className="row">
          <Link to="/characters" className="nav-link" activeProps={{ className: "nav-link active" }}>
            Characters
          </Link>
        </nav>
        <Link to="/accounts" className="icon-button" title="Accounts" activeProps={{ className: "icon-button active" }}>
          <PersonIcon />
        </Link>
      </header>
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}

function PersonIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21c0-4 3.6-7 8-7s8 3 8 7" />
    </svg>
  );
}
