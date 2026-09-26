"use client";

import { useState, useEffect, useRef } from "react";
import { apiFetch } from "@/lib/api";
import { toast } from "@/context/ToastContext";
import { useAuth } from "@/context/AuthContext";

const fullName = (u) => `${u.first_name || ""} ${u.last_name || ""}`.trim();

// "+ Invite" button on a group's page, for any active member (not only the
// creator). The backend lets a member invite anyone they have a follow
// relationship with in either direction, so the list is the member's
// followers plus the people they follow, minus anyone already in the group
// or already invited. onInvited(users) is called with the users that were
// invited successfully, so the page can show them as "Invited" right away.
export default function GroupInviteButton({ groupId, members = [], onInvited }) {
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const [candidates, setCandidates] = useState([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState(new Set());
  const [sending, setSending] = useState(false);
  const wrapperRef = useRef(null);

  // Close on an outside click.
  useEffect(() => {
    if (!open) return;
    const handleClick = (e) => {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  // Load followers + following the first time the list is opened.
  useEffect(() => {
    if (!open || loaded || !user) return;
    (async () => {
      setLoading(true);
      try {
        const [followers, following] = await Promise.all([
          apiFetch(`/user/followers?id=${user.id}`),
          apiFetch(`/user/following?id=${user.id}`),
        ]);
        const byId = new Map();
        for (const res of [followers, following]) {
          if (res.ok && Array.isArray(res.data)) {
            for (const f of res.data) if (f.user) byId.set(f.user.id, f.user);
          }
        }
        setCandidates(
          [...byId.values()].sort((a, b) =>
            fullName(a).toLowerCase().localeCompare(fullName(b).toLowerCase()),
          ),
        );
      } catch (err) {
        console.error("Error loading people to invite:", err);
      } finally {
        setLoading(false);
        setLoaded(true);
      }
    })();
  }, [open, loaded, user]);

  // Anyone already in the group (active or invited) can't be invited again.
  const inGroup = new Set(members.map((m) => m.user?.id));
  const invitable = candidates.filter((u) => !inGroup.has(u.id));

  const toggle = (id) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const sendInvites = async () => {
    const invitees = invitable.filter((u) => selected.has(u.id));
    if (invitees.length === 0) return;

    setSending(true);
    const results = await Promise.all(
      invitees.map((u) =>
        apiFetch("/group/invite", {
          method: "POST",
          json: { group_id: parseInt(groupId), user_id: u.id },
        })
          .then(({ ok, data }) => ({ user: u, ok, error: data?.error }))
          .catch(() => ({ user: u, ok: false, error: "Network error" })),
      ),
    );
    setSending(false);

    const invited = results.filter((r) => r.ok).map((r) => r.user);
    const failed = results.filter((r) => !r.ok);
    if (invited.length > 0) {
      toast.success(
        invited.length === 1
          ? `Invited ${fullName(invited[0])}`
          : `Invited ${invited.length} people`,
      );
      onInvited?.(invited);
    }
    failed.forEach((r) =>
      toast.error(`${fullName(r.user)}: ${r.error || "could not be invited"}`),
    );

    setSelected(new Set());
    if (failed.length === 0) setOpen(false);
  };

  return (
    <div className='groups-dropdown-wrapper group-invite' ref={wrapperRef}>
      <button
        type='button'
        className='group-invite-btn'
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}>
        + Invite
      </button>

      {open && (
        <div className='groups-dropdown-panel group-invite-panel'>
          {loading ? (
            <div className='groups-dropdown-empty'>Loading…</div>
          ) : invitable.length === 0 ? (
            <div className='groups-dropdown-empty'>
              {candidates.length === 0
                ? "You can invite people who follow you or that you follow - you don't have any yet."
                : "Everyone you follow or who follows you is already in this group."}
            </div>
          ) : (
            <>
              {invitable.map((u) => (
                <div
                  key={u.id}
                  className='groups-dropdown-row'
                  onClick={() => toggle(u.id)}>
                  <input
                    type='checkbox'
                    className='groups-checkbox'
                    checked={selected.has(u.id)}
                    onChange={() => toggle(u.id)}
                    onClick={(e) => e.stopPropagation()}
                    aria-label={`Invite ${fullName(u)}`}
                  />
                  <div className='groups-avatar'>
                    {u.first_name?.[0]}
                    {u.last_name?.[0]}
                  </div>
                  <div className='groups-user-info'>
                    <div className='groups-user-name'>{fullName(u)}</div>
                    {u.nickname && (
                      <div className='groups-user-nick'>@{u.nickname}</div>
                    )}
                  </div>
                </div>
              ))}
              <div className='group-invite-footer'>
                <button
                  type='button'
                  className='group-invite-send'
                  disabled={selected.size === 0 || sending}
                  onClick={sendInvites}>
                  {sending
                    ? "Sending…"
                    : `Send invite${selected.size === 1 ? "" : "s"}${
                        selected.size ? ` (${selected.size})` : ""
                      }`}
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}
