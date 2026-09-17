"use client";

import { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "@/context/ToastContext";

const API_BASE = "http://localhost:8081";

export default function GroupsPage() {
  const router = useRouter();

  // ---- Create-group form state ------------------------------------------
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [creating, setCreating] = useState(false);

  // ---- Invite state -----------------------------------------------------
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [pendingInvites, setPendingInvites] = useState([]); // [{id, email, first_name, last_name}]
  const [lookingUp, setLookingUp] = useState(false);
  const inviteRef = useRef(null);

  // ---- Groups list ------------------------------------------------------
  const [groups, setGroups] = useState([]);
  const [loadingGroups, setLoadingGroups] = useState(true);

  // Load groups on mount
  useEffect(() => {
    (async () => {
      try {
        const authRes = await fetch(`${API_BASE}/api/check-auth`, {
          credentials: "include",
        });
        const authData = await authRes.json();
        if (!authData.authenticated) {
          router.push("/login");
          return;
        }
        const res = await fetch(`${API_BASE}/api/groups`, {
          credentials: "include",
        });
        const data = res.ok ? await res.json() : [];
        setGroups(Array.isArray(data) ? data : data.groups || []);
      } catch (e) {
        console.error(e);
      } finally {
        setLoadingGroups(false);
      }
    })();
  }, [router]);

  // Close invite popover on outside click
  useEffect(() => {
    const onClick = (e) => {
      if (inviteRef.current && !inviteRef.current.contains(e.target)) {
        setInviteOpen(false);
      }
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

  // ---- Add an email to the pending list (looks up the user first) ------
  const handleLookupEmail = async (e) => {
    e.preventDefault();
    const email = inviteEmail.trim();
    if (!email) return;

    // Avoid duplicates in the pending list
    if (pendingInvites.some((p) => p.email === email)) {
      toast.error("That email is already on the list");
      return;
    }

    setLookingUp(true);
    try {
      const res = await fetch(
        `${API_BASE}/api/user/search?email=${encodeURIComponent(email)}`,
        { credentials: "include" },
      );

      if (res.status === 404) {
        toast.error("No user found with that email");
        return;
      }
      if (!res.ok) {
        toast.error("Lookup failed");
        return;
      }

      const user = await res.json();
      if (!user || !user.id) {
        toast.error("User not found");
        return;
      }

      setPendingInvites((prev) => [
        ...prev,
        {
          id: user.id,
          email: user.email || email,
          first_name: user.first_name || "",
          last_name: user.last_name || "",
        },
      ]);
      setInviteEmail("");
    } catch (err) {
      console.error(err);
      toast.error("Network error");
    } finally {
      setLookingUp(false);
    }
  };

  const removePendingInvite = (id) => {
    setPendingInvites((prev) => prev.filter((p) => p.id !== id));
  };

  // ---- Create group + send all pending invites --------------------------
  const handleCreateGroup = async (e) => {
    e.preventDefault();
    if (!title.trim()) {
      toast.error("Group title is required");
      return;
    }

    setCreating(true);
    try {
      // 1. Create the group
      const createRes = await fetch(`${API_BASE}/api/group/create`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          title: title.trim(),
          description: description.trim(),
        }),
      });

      if (!createRes.ok) {
        const err = await createRes.json().catch(() => ({}));
        toast.error(err.error || "Failed to create group");
        return;
      }

      const created = await createRes.json();
      const groupId = created.id ?? created.group_id ?? created.ID;
      if (!groupId) {
        toast.error("Server did not return a group id");
        return;
      }

      // 2. Send invites to every email the user added
      if (pendingInvites.length > 0) {
        const results = await Promise.allSettled(
          pendingInvites.map((p) =>
            fetch(`${API_BASE}/api/group/invite`, {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              credentials: "include",
              body: JSON.stringify({
                group_id: Number(groupId),
                user_id: Number(p.id),
              }),
            }).then((r) => r.ok),
          ),
        );
        const sent = results.filter(
          (r) => r.status === "fulfilled" && r.value === true,
        ).length;
        const failed = pendingInvites.length - sent;

        if (failed === 0) {
          toast.success(
            `Group created. ${sent} invitation${sent === 1 ? "" : "s"} sent.`,
          );
        } else if (sent === 0) {
          toast.error("Group created, but all invitations failed.");
        } else {
          toast.success(`Group created. ${sent} sent, ${failed} failed.`);
        }
      } else {
        toast.success("Group created");
      }

      // Reset form
      setTitle("");
      setDescription("");
      setPendingInvites([]);
      setInviteEmail("");
      setInviteOpen(false);

      // Refresh groups list
      const groupsRes = await fetch(`${API_BASE}/api/groups`, {
        credentials: "include",
      });
      const groupsData = groupsRes.ok ? await groupsRes.json() : [];
      setGroups(
        Array.isArray(groupsData) ? groupsData : groupsData.groups || [],
      );
    } catch (err) {
      console.error(err);
      toast.error("Network error");
    } finally {
      setCreating(false);
    }
  };

  // ---- Render -----------------------------------------------------------
  return (
    <div className='groups-page'>
      <div className='groups-header'>
        <h1 className='groups-title'>Groups</h1>
        <button
          type='button'
          onClick={() => router.back()}
          className='groups-cancel'>
          Cancel
        </button>
      </div>

      <form onSubmit={handleCreateGroup} className='groups-form'>
        <input
          type='text'
          placeholder='Group Title *'
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          className='groups-input'
          required
        />

        <textarea
          placeholder='Description (optional)'
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={4}
          className='groups-textarea'
        />

        {/* ---- + Invite button and popover ---- */}
        <div className='groups-dropdown-wrapper' ref={inviteRef}>
          <button
            type='button'
            onClick={() => setInviteOpen((v) => !v)}
            className='groups-invite-btn'>
            <span>
              + Invite
              {pendingInvites.length > 0 && (
                <span className='groups-invite-count'>
                  {" "}
                  ({pendingInvites.length})
                </span>
              )}
            </span>
            <span className='groups-invite-arrow'>
              {inviteOpen ? "▲" : "▼"}
            </span>
          </button>

          {inviteOpen && (
            <div className='groups-dropdown-panel'>
              <form onSubmit={handleLookupEmail} className='groups-invite-form'>
                <input
                  type='email'
                  placeholder='user@example.com'
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  className='groups-input'
                  autoFocus
                />
                <button
                  type='submit'
                  disabled={lookingUp || !inviteEmail.trim()}
                  className='groups-invite-add-btn'>
                  {lookingUp ? "…" : "Add"}
                </button>
              </form>

              {pendingInvites.length === 0 ? (
                <div className='groups-dropdown-empty'>
                  No one added yet — type an email above.
                </div>
              ) : (
                pendingInvites.map((p) => (
                  <div key={p.id} className='groups-dropdown-row'>
                    <div className='groups-avatar'>
                      {p.first_name?.[0]}
                      {p.last_name?.[0]}
                    </div>
                    <div className='groups-user-info'>
                      <div className='groups-user-name'>
                        {p.first_name} {p.last_name}
                      </div>
                      <div className='groups-user-nick'>{p.email}</div>
                    </div>
                    <button
                      type='button'
                      onClick={() => removePendingInvite(p.id)}
                      className='groups-invite-remove-btn'
                      aria-label='Remove'>
                      ×
                    </button>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        <div className='groups-submit-row'>
          <button
            type='submit'
            disabled={creating}
            className='groups-submit-btn'>
            {creating ? "Creating…" : "Create Group"}
          </button>
        </div>
      </form>

      {/* ---- Groups list ---- */}
      <div className='groups-list-section'>
        <h2 className='groups-list-title'>All Groups</h2>
        {loadingGroups ? (
          <div className='groups-loading'>Loading…</div>
        ) : groups.length === 0 ? (
          <div className='groups-empty'>
            No groups yet. Create the first one!
          </div>
        ) : (
          <div className='groups-list'>
            {groups.map((g) => (
              <Link key={g.id} href={`/groups/${g.id}`} className='groups-card'>
                <h3 className='groups-card-title'>{g.title}</h3>
                {g.description && (
                  <p className='groups-card-desc'>{g.description}</p>
                )}
                <div className='groups-card-meta'>
                  <span>👥 {g.member_count ?? 0} members</span>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
