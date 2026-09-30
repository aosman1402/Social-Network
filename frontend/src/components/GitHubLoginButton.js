"use client";

import { useEffect, useState } from "react";
import { API_BASE, apiFetch } from "@/lib/api";
import Icon from "@/components/Icon";

// "Continue with GitHub" - a plain link to the backend, which redirects to
// GitHub and back (see backend/pkg/handlers/oauth_github.go).
//
// It needs GitHub keys on the server (GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET
// in backend/.env - never committed). Without them - e.g. a fresh clone -
// the button stays visible but disabled, with a short explanation for
// people logging in and the exact setup steps for whoever runs the server.
//
// divider: text for the "or" line; dividerPosition: "before" (login page,
// under the form) or "after" (register page, above the form).
export default function GitHubLoginButton({
  label = "Continue with GitHub",
  divider = "or",
  dividerPosition = "before",
}) {
  const [enabled, setEnabled] = useState(null); // null = still checking

  useEffect(() => {
    let cancelled = false;
    apiFetch("/auth/providers")
      .then(({ ok, data }) => {
        if (!cancelled) setEnabled(ok && !!data?.github);
      })
      .catch(() => {
        if (!cancelled) setEnabled(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Nothing until we know, so the button doesn't flicker between states.
  if (enabled === null) return null;

  const icon = (
    <svg viewBox='0 0 16 16' width='18' height='18' aria-hidden='true' fill='currentColor'>
      <path d='M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z' />
    </svg>
  );

  const line = (
    <div className='auth-divider'>
      <span>{divider}</span>
    </div>
  );

  // The URLs to register with GitHub, worked out from where this app is
  // actually running, so they're right on any machine.
  const homepageUrl = typeof window !== "undefined" ? window.location.origin : "http://localhost:3000";
  const callbackUrl = `${API_BASE}/auth/github/callback`;

  return (
    <>
      {dividerPosition === "before" && line}

      {enabled ? (
        <a href={`${API_BASE}/auth/github/login`} className='auth-github-btn'>
          {icon}
          <span>{label}</span>
        </a>
      ) : (
        <div className='auth-github-off'>
          <button
            type='button'
            className='auth-github-btn'
            disabled
            aria-describedby='github-off-note'>
            {icon}
            <span>{label}</span>
          </button>

          <div className='auth-github-info' id='github-off-note'>
            <p className='auth-github-info-title'>
              <Icon name='info' /> GitHub sign-in isn&apos;t connected on this server yet
            </p>
            <p className='auth-github-info-text'>
              You can still log in or create an account with your email and password.
            </p>

            <details className='auth-github-setup'>
              <summary>How to turn it on (for whoever runs this server)</summary>
              <ol>
                <li>
                  On GitHub, go to <strong>Settings → Developer settings → OAuth Apps → New OAuth App</strong>.
                </li>
                <li>
                  Use <code>{homepageUrl}</code> as the Homepage URL and{" "}
                  <code>{callbackUrl}</code> as the Redirect (callback) URL.
                </li>
                <li>
                  Put the app&apos;s Client ID and a new Client secret in{" "}
                  <code>backend/.env</code> as <code>GITHUB_CLIENT_ID</code> and{" "}
                  <code>GITHUB_CLIENT_SECRET</code> (see <code>backend/.env.example</code>).
                  With Docker, use a <code>.env</code> file next to <code>docker-compose.yml</code>.
                </li>
                <li>Restart the backend - this button then switches on.</li>
              </ol>
            </details>
          </div>
        </div>
      )}

      {dividerPosition === "after" && line}
    </>
  );
}
