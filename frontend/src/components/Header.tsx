import { useState } from "react";
import { Link, NavLink, useNavigate } from "react-router";
import { api, errorMessage, type DeviceMember } from "../api";
import { useSession } from "../session";

/** App header: the active member, plus a switcher when this browser has several. */
export default function Header() {
  const { session, setMe } = useSession();
  const navigate = useNavigate();
  const [switching, setSwitching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function switchTo(memberId: string) {
    setSwitching(true);
    setError(null);
    try {
      setMe(await api.switchMember(memberId));
      navigate("/today");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSwitching(false);
    }
  }

  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link to="/" className="brand">
          LittleVillage
        </Link>
        {session.status === "ready" && (
          <nav className="site-nav" aria-label="Main">
            <NavLink to="/today">Today</NavLink>
            <NavLink to="/family">Family</NavLink>
            <NavLink to="/tasks">Tasks</NavLink>
            <NavLink to="/learn">Learn</NavLink>
          </nav>
        )}
        {session.status === "ready" &&
          (session.me.device_members.length > 1 ? (
            <label className="switcher">
              <span>You are</span>
              <select
                value={session.me.member.id}
                onChange={(e) => void switchTo(e.target.value)}
                disabled={switching}
                aria-busy={switching}
              >
                {session.me.device_members.map((m) => (
                  <option key={m.id} value={m.id}>
                    {optionLabel(m, session.me.device_members)}
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <span className="switcher">
              You are <strong>{session.me.member.name}</strong>
            </span>
          ))}
      </div>
      {switching && (
        <p className="header-note" role="status">
          Switching…
        </p>
      )}
      {error && (
        <p className="header-note header-error" role="alert">
          {error}
        </p>
      )}
    </header>
  );
}

function optionLabel(m: DeviceMember, all: DeviceMember[]): string {
  const severalFamilies = new Set(all.map((x) => x.family_name)).size > 1;
  return severalFamilies ? `${m.name} (${m.relationship}) · ${m.family_name}` : `${m.name} (${m.relationship})`;
}
