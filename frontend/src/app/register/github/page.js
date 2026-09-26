"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch } from "@/lib/api";
import NicknameField from "@/components/NicknameField";

// Last step of signing up with GitHub. GitHub gives us a verified email and
// a name, but an account here also needs a date of birth (and may have a
// nickname / about me), so a first-time GitHub user lands here with a
// one-time ?token=... from the backend's callback.
export default function GitHubSignUpPage() {
  const [token, setToken] = useState("");
  const [pending, setPending] = useState(null);
  const [loadError, setLoadError] = useState("");
  const [form, setForm] = useState({
    first_name: "",
    last_name: "",
    date_of_birth: "",
    nickname: "",
    about_me: "",
    use_github_avatar: true,
  });
  const [nicknameStatus, setNicknameStatus] = useState("idle");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    // window.location rather than useSearchParams(), which would need a
    // Suspense boundary for this page to build.
    const t = new URLSearchParams(window.location.search).get("token") || "";
    setToken(t);
    if (!t) {
      setLoadError("This sign-up link is missing its token - please continue with GitHub again.");
      return;
    }
    apiFetch(`/auth/oauth/pending?token=${encodeURIComponent(t)}`)
      .then(({ ok, data }) => {
        if (!ok) {
          setLoadError(data?.error || "This sign-up link has expired - please continue with GitHub again.");
          return;
        }
        setPending(data);
        setForm((prev) => ({
          ...prev,
          first_name: data.first_name || "",
          last_name: data.last_name || "",
          use_github_avatar: !!data.avatar_url,
        }));
      })
      .catch(() => setLoadError("Couldn't reach the server - please try again."));
  }, []);

  const update = (field) => (e) =>
    setForm({ ...form, [field]: e.target.type === "checkbox" ? e.target.checked : e.target.value });

  const today = new Date().toISOString().slice(0, 10);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");
    if (!form.first_name.trim() || !form.last_name.trim() || !form.date_of_birth) {
      setError("First name, last name and date of birth are required.");
      return;
    }
    if (form.nickname && nicknameStatus !== "available") {
      setError("Please choose an available nickname, or leave it empty.");
      return;
    }

    setSubmitting(true);
    try {
      const { ok, data } = await apiFetch("/auth/oauth/complete", {
        method: "POST",
        json: {
          token,
          first_name: form.first_name.trim(),
          last_name: form.last_name.trim(),
          date_of_birth: form.date_of_birth,
          nickname: form.nickname || null,
          about_me: form.about_me || null,
          use_github_avatar: form.use_github_avatar,
        },
      });
      if (!ok) {
        setError(data?.error || "Couldn't create your account - please try again.");
        return;
      }
      // Full reload so the navbar, socket and auth state all start fresh
      // with the new session - same as the password login page.
      window.location.href = "/";
    } catch {
      setError("Couldn't reach the server - please try again.");
    } finally {
      setSubmitting(false);
    }
  };

  if (loadError) {
    return (
      <div className='auth-oauth-page'>
        <div className='auth-oauth-card'>
          <h2 className='auth-oauth-title'>Sign up with GitHub</h2>
          <p className='auth-oauth-error'>{loadError}</p>
          <Link href='/login' className='auth-oauth-link'>
            Back to login
          </Link>
        </div>
      </div>
    );
  }

  if (!pending) {
    return (
      <div className='auth-oauth-page'>
        <div className='auth-oauth-card'>
          <p className='auth-oauth-muted'>Loading…</p>
        </div>
      </div>
    );
  }

  return (
    <div className='auth-oauth-page'>
      <div className='auth-oauth-card'>
        <h2 className='auth-oauth-title'>Almost there</h2>
        <p className='auth-oauth-muted'>
          Signed in with GitHub as <strong>{pending.username}</strong>. Add the
          details below to finish creating your account.
        </p>

        <form onSubmit={handleSubmit} noValidate className='auth-oauth-form'>
          <label className='auth-oauth-label'>
            Email (verified by GitHub)
            <input className='auth-oauth-input' type='email' value={pending.email} readOnly disabled />
          </label>

          <div className='auth-oauth-row'>
            <label className='auth-oauth-label'>
              First name *
              <input
                className='auth-oauth-input'
                name='first_name'
                value={form.first_name}
                onChange={update("first_name")}
                autoComplete='given-name'
                required
              />
            </label>
            <label className='auth-oauth-label'>
              Last name *
              <input
                className='auth-oauth-input'
                name='last_name'
                value={form.last_name}
                onChange={update("last_name")}
                autoComplete='family-name'
                required
              />
            </label>
          </div>

          <label className='auth-oauth-label'>
            Date of birth *
            <input
              className='auth-oauth-input'
              type='date'
              name='date_of_birth'
              value={form.date_of_birth}
              max={today}
              onChange={update("date_of_birth")}
              required
            />
          </label>

          <div>
            <NicknameField
              id='github-nickname'
              value={form.nickname}
              onValueChange={(value) => setForm((prev) => ({ ...prev, nickname: value }))}
              status={nicknameStatus}
              onStatusChange={setNicknameStatus}
            />
          </div>

          <label className='auth-oauth-label'>
            About me (optional)
            <textarea
              className='auth-oauth-input'
              name='about_me'
              rows={3}
              value={form.about_me}
              onChange={update("about_me")}
            />
          </label>

          {pending.avatar_url && (
            <label className='auth-oauth-avatar'>
              <input
                type='checkbox'
                checked={form.use_github_avatar}
                onChange={update("use_github_avatar")}
              />
              <img src={pending.avatar_url} alt='' width={36} height={36} />
              Use my GitHub picture as my avatar
            </label>
          )}

          {error && <p className='auth-oauth-error'>{error}</p>}

          <button type='submit' className='auth-oauth-submit' disabled={submitting}>
            {submitting ? "Creating account…" : "Create account"}
          </button>
        </form>
      </div>
    </div>
  );
}
