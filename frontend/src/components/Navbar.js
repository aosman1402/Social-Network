"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";
import { toast } from "@/context/ToastContext";
import NotificationBell from "@/components/NotificationBell";
import Avatar from "@/components/Avatar";

function HomeIcon({ size = 18 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <path d='M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z' />
      <polyline points='9 22 9 12 15 12 15 22' />
    </svg>
  );
}

function ProfileIcon({ size = 18 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <path d='M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2' />
      <circle cx='12' cy='7' r='4' />
    </svg>
  );
}

function GroupsIcon({ size = 18 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <path d='M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2' />
      <circle cx='9' cy='7' r='4' />
      <path d='M23 21v-2a4 4 0 0 0-3-3.87' />
      <path d='M16 3.13a4 4 0 0 1 0 7.75' />
    </svg>
  );
}

function ChatIcon({ size = 18 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <path d='M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z' />
    </svg>
  );
}

function SettingsIcon({ size = 18 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <circle cx='12' cy='12' r='3' />
      <path d='M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z' />
    </svg>
  );
}

function MenuIcon({ size = 22 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <line x1='3' y1='12' x2='21' y2='12' />
      <line x1='3' y1='6' x2='21' y2='6' />
      <line x1='3' y1='18' x2='21' y2='18' />
    </svg>
  );
}

function CloseIcon({ size = 22 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <line x1='18' y1='6' x2='6' y2='18' />
      <line x1='6' y1='6' x2='18' y2='18' />
    </svg>
  );
}

function ChevronDownIcon({ size = 14 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <polyline points='6 9 12 15 18 9' />
    </svg>
  );
}

function LogoutIcon({ size = 16 }) {
  return (
    <svg width={size} height={size} viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2' strokeLinecap='round' strokeLinejoin='round' aria-hidden='true'>
      <path d='M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4' />
      <polyline points='16 17 21 12 16 7' />
      <line x1='21' y1='12' x2='9' y2='12' />
    </svg>
  );
}

function getNavLinks(userId) {
  return [
    { href: "/", label: "Home", Icon: HomeIcon, isActive: (p) => p === "/" },
    { href: `/profile/${userId}`, label: "Profile", Icon: ProfileIcon, isActive: (p) => p.startsWith("/profile/") },
    { href: "/groups", label: "Groups", Icon: GroupsIcon, isActive: (p) => p.startsWith("/groups") },
    { href: "/chat", label: "Chat", Icon: ChatIcon, isActive: (p) => p.startsWith("/chat") },
  ];
}

export default function Navbar() {
  const { user, loading, logout } = useAuth();
  const { subscribe } = useWebSocket();
  const pathname = usePathname();
  const router = useRouter();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef(null);

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (menuRef.current && !menuRef.current.contains(event.target)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  useEffect(() => {
    setMobileOpen(false);
    setMenuOpen(false);
  }, [pathname]);

  // A "chat" event only ever gets picked up by chat/[id]/page.js, and only
  // while that exact conversation happens to be open - anywhere else in the
  // app (the chat list, home, a profile), a new message arrived with no
  // visible sign of it at all. This surfaces it everywhere except the one
  // place that already shows the message inline.
  useEffect(() => {
    if (!user) return;
    return subscribe("chat", (msg) => {
      if (msg.sender_id === user.id) return; // our own message, echoed
      if (pathname === `/chat/${msg.sender_id}`) return; // already visible

      const name = msg.sender
        ? `${msg.sender.first_name} ${msg.sender.last_name}`.trim()
        : "Someone";
      toast.success(`${name}: ${msg.content}`);
    });
  }, [subscribe, user, pathname]);

  const handleLogout = async () => {
    const result = await logout();
    if (result.success) {
      router.push("/login");
    }
  };

  if (loading) {
    return (
      <nav className='navbar'>
        <div className='navbar-inner'>
          <Link href='/' className='navbar-brand'>
            Social Network for Studies
          </Link>
          <span className='navbar-loading'>Loading...</span>
        </div>
      </nav>
    );
  }

  const links = user ? getNavLinks(user.id) : [];

  return (
    <nav className='navbar'>
      <div className='navbar-inner'>
        <Link href='/' className='navbar-brand'>
          Social Network for Studies
        </Link>

        {user ? (
          <>
            <button
              type='button'
              className='navbar-toggle'
              aria-label='Toggle navigation menu'
              aria-expanded={mobileOpen}
              onClick={() => setMobileOpen((open) => !open)}>
              {mobileOpen ? <CloseIcon /> : <MenuIcon />}
            </button>

            <div className={`navbar-links ${mobileOpen ? "navbar-links--open" : ""}`}>
              {links.map(({ href, label, Icon, isActive }) => (
                <Link
                  key={href}
                  href={href}
                  className={`navbar-link ${isActive(pathname) ? "navbar-link--active" : ""}`}>
                  <Icon size={18} />
                  <span>{label}</span>
                </Link>
              ))}
            </div>

            <div className='navbar-actions'>
              <NotificationBell />

              <div className='navbar-menu' ref={menuRef}>
                <button
                  type='button'
                  className='navbar-menu-trigger'
                  aria-haspopup='true'
                  aria-expanded={menuOpen}
                  onClick={() => setMenuOpen((open) => !open)}>
                  <Avatar user={user} size='sm' />
                  <span className='navbar-menu-name'>{user.first_name}</span>
                  <ChevronDownIcon size={14} />
                </button>

                {menuOpen && (
                  <div className='navbar-menu-panel'>
                    <div className='navbar-menu-header'>
                      <Avatar user={user} size='md' />
                      <div className='navbar-menu-header-text'>
                        <p className='navbar-menu-fullname'>
                          {user.first_name} {user.last_name}
                        </p>
                        <p className='navbar-menu-email'>{user.email}</p>
                      </div>
                    </div>

                    <Link href={`/profile/${user.id}`} className='navbar-menu-item'>
                      <ProfileIcon size={16} />
                      View Profile
                    </Link>
                    <Link href='/settings' className='navbar-menu-item'>
                      <SettingsIcon size={16} />
                      Settings
                    </Link>
                    <button
                      type='button'
                      className='navbar-menu-item navbar-menu-item--danger'
                      onClick={handleLogout}>
                      <LogoutIcon size={16} />
                      Logout
                    </button>
                  </div>
                )}
              </div>
            </div>
          </>
        ) : (
          <Link href='/login' className='navbar-login'>
            Login
          </Link>
        )}
      </div>
    </nav>
  );
}
