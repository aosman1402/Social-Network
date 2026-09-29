"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";
import { useRequireAuth } from "@/hooks/useRequireAuth";
import { getAvatarColor } from "@/utils/helpers";

export default function GroupsPage() {
  const { user } = useRequireAuth();

  // ---- Create-group form state ------------------------------------------
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [creating, setCreating] = useState(false);

  // ---- Invite state -------------------------------------------------------
  // Picking invitees from the follower list (rather than looking each one
  // up by email) - the list is fetched lazily, the first time the invite
  // dropdown is opened, then kept for the rest of the visit.
  const [inviteOpen, setInviteOpen] = useState(false);
  const [followers, setFollowers] = useState([]);
  const [followersLoaded, setFollowersLoaded] = useState(false);
  const [loadingFollowers, setLoadingFollowers] = useState(false);
  const [selectedFollowerIds, setSelectedFollowerIds] = useState(new Set());
  const inviteRef = useRef(null);

  // ---- Groups list ------------------------------------------------------
  const [groups, setGroups] = useState([]);
  const [loadingGroups, setLoadingGroups] = useState(true);
  const [filter, setFilter] = useState("all"); // all | mine | invites

  const fetchGroups = async () => {
    const { ok, data } = await apiFetch("/groups");
    setGroups(ok && Array.isArray(data) ? data : []);
  };

  // Load groups once the shared auth check has resolved a signed-in user.
  useEffect(() => {
    if (!user) return;
    fetchGroups().finally(() => setLoadingGroups(false));
  }, [user]);

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

  // Fetch the signed-in user's followers the first time the invite
  // dropdown is opened.
  useEffect(() => {
    if (!inviteOpen || followersLoaded || !user) return;

    (async () => {
      setLoadingFollowers(true);
      try {
        const { ok, data } = await apiFetch(`/user/followers?id=${user.id}`);
        if (ok && Array.isArray(data)) {
          const sorted = [...data].sort((a, b) =>
            `${a.user.first_name} ${a.user.last_name}`
              .toLowerCase()
              .localeCompare(
                `${b.user.first_name} ${b.user.last_name}`.toLowerCase(),
              ),
          );
          setFollowers(sorted);
        }
      } catch (err) {
        console.error(err);
      } finally {
        setLoadingFollowers(false);
        setFollowersLoaded(true);
      }
    })();
  }, [inviteOpen, followersLoaded, user]);

  const toggleFollower = (id) => {
    setSelectedFollowerIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const allFollowersSelected =
    followers.length > 0 && selectedFollowerIds.size === followers.length;

  const toggleSelectAllFollowers = () => {
    setSelectedFollowerIds(
      allFollowersSelected
        ? new Set()
        : new Set(followers.map((f) => f.user.id)),
    );
  };

  // ---- Create group + send all invites to selected followers ------------
  const handleCreateGroup = async (e) => {
    e.preventDefault();
    if (!title.trim()) {
      toast.error("Group title is required");
      return;
    }

    setCreating(true);
    try {
      // 1. Create the group
      const { ok: createOk, data: created } = await apiFetch("/group/create", {
        method: "POST",
        json: {
          title: title.trim(),
          description: description.trim(),
        },
      });

      if (!createOk) {
        toast.error(created.error || "Failed to create group");
        return;
      }

      const groupId = created.id ?? created.group_id ?? created.ID;
      if (!groupId) {
        toast.error("Server did not return a group id");
        return;
      }

      // 2. Invite every selected follower
      const invitees = followers.filter((f) =>
        selectedFollowerIds.has(f.user.id),
      );
      if (invitees.length > 0) {
        const results = await Promise.allSettled(
          invitees.map((f) =>
            apiFetch("/group/invite", {
              method: "POST",
              json: { group_id: Number(groupId), user_id: Number(f.user.id) },
            }).then((r) => r.ok),
          ),
        );
        const sent = results.filter(
          (r) => r.status === "fulfilled" && r.value === true,
        ).length;
        const failed = invitees.length - sent;

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
      setSelectedFollowerIds(new Set());
      setInviteOpen(false);

      await fetchGroups();
    } catch (err) {
      console.error(err);
      toast.error("Network error");
    } finally {
      setCreating(false);
    }
  };

  // ---- Render -----------------------------------------------------------
  const myGroups = groups.filter((g) => g.is_member);
  const invitations = groups.filter((g) => g.is_invited);
  const shown = filter === "mine" ? myGroups : filter === "invites" ? invitations : groups;
  const tabs = [
    { key: "all", label: "All groups", count: groups.length },
    { key: "mine", label: "My groups", count: myGroups.length },
    ...(invitations.length > 0 ? [{ key: "invites", label: "Invitations", count: invitations.length }] : []),
  ];

  // What the viewer's relationship to a group is, for the card's badge.
  const groupStatus = (g) => {
    if (g.creator_id === user?.id) return { label: "★ Creator", tone: "creator" };
    if (g.is_member) return { label: "✓ Member", tone: "member" };
    if (g.is_invited) return { label: "✉ Invited", tone: "invited" };
    if (g.has_pending_request) return { label: "⏳ Requested", tone: "pending" };
    return { label: "Join →", tone: "join" };
  };

  return (
    <div className='groups-page'>
      {/* ---- Hero ---- */}
      <header className='groups-hero'>
        <div className='groups-hero-text'>
          <h1 className='groups-title'>Groups</h1>
          <p className='groups-subtitle'>
            Find your people - join a group, or start your own and invite your followers.
          </p>
          <div className='groups-hero-stats'>
            <span className='groups-hero-chip'>👥 {groups.length} groups</span>
            <span className='groups-hero-chip'>✓ You&apos;re in {myGroups.length}</span>
            {invitations.length > 0 && (
              <span className='groups-hero-chip groups-hero-chip--alert'>
                ✉ {invitations.length} invitation{invitations.length === 1 ? "" : "s"}
              </span>
            )}
          </div>
        </div>
      </header>

      <div className='groups-layout'>
      {/* ---- Create a group ---- */}
      <form onSubmit={handleCreateGroup} className='groups-form'>
        <div className='groups-form-head'>
          <span className='groups-form-icon' aria-hidden='true'>＋</span>
          <div>
            <h2 className='groups-form-title'>Create a group</h2>
            <p className='groups-form-hint'>Give it a name, then invite followers.</p>
          </div>
        </div>

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
              {selectedFollowerIds.size > 0 && (
                <span className='groups-invite-count'>
                  {" "}
                  ({selectedFollowerIds.size})
                </span>
              )}
            </span>
            <span className='groups-invite-arrow'>
              {inviteOpen ? "▲" : "▼"}
            </span>
          </button>

          {inviteOpen && (
            <div className='groups-dropdown-panel'>
              {loadingFollowers ? (
                <div className='groups-dropdown-empty'>
                  Loading your followers…
                </div>
              ) : followers.length === 0 ? (
                <div className='groups-dropdown-empty'>
                  You don&apos;t have any followers yet to invite.
                </div>
              ) : (
                <>
                  <div
                    className='groups-dropdown-row groups-dropdown-row--all'
                    onClick={toggleSelectAllFollowers}>
                    <input
                      type='checkbox'
                      className='groups-checkbox'
                      checked={allFollowersSelected}
                      onChange={toggleSelectAllFollowers}
                      onClick={(e) => e.stopPropagation()}
                      aria-label='Select all followers'
                    />
                    <div className='groups-user-info'>
                      <div className='groups-user-name'>All Followers</div>
                    </div>
                  </div>

                  {followers.map((f) => (
                    <div
                      key={f.user.id}
                      className='groups-dropdown-row'
                      onClick={() => toggleFollower(f.user.id)}>
                      <input
                        type='checkbox'
                        className='groups-checkbox'
                        checked={selectedFollowerIds.has(f.user.id)}
                        onChange={() => toggleFollower(f.user.id)}
                        onClick={(e) => e.stopPropagation()}
                        aria-label={`Invite ${f.user.first_name} ${f.user.last_name}`}
                      />
                      <div className='groups-avatar'>
                        {f.user.first_name?.[0]}
                        {f.user.last_name?.[0]}
                      </div>
                      <div className='groups-user-info'>
                        <div className='groups-user-name'>
                          {f.user.first_name} {f.user.last_name}
                        </div>
                        {f.user.nickname && (
                          <div className='groups-user-nick nickname'>
                            @{f.user.nickname}
                          </div>
                        )}
                      </div>
                    </div>
                  ))}
                </>
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
      <section className='groups-list-section'>
        <div className='groups-tabs' role='tablist'>
          {tabs.map((t) => (
            <button
              key={t.key}
              type='button'
              role='tab'
              aria-selected={filter === t.key}
              onClick={() => setFilter(t.key)}
              className={`groups-tab ${filter === t.key ? "groups-tab--active" : ""}`}>
              {t.label}
              <span className='groups-tab-count'>{t.count}</span>
            </button>
          ))}
        </div>

        {loadingGroups ? (
          <div className='groups-loading'>Loading…</div>
        ) : shown.length === 0 ? (
          <div className='groups-empty'>
            {filter === "mine"
              ? "You're not in any groups yet - join one, or create your own."
              : filter === "invites"
                ? "No invitations right now."
                : "No groups yet. Create the first one!"}
          </div>
        ) : (
          <div className='groups-list'>
            {shown.map((g) => {
              const status = groupStatus(g);
              return (
                <Link key={g.id} href={`/groups/${g.id}`} className='groups-card'>
                  <div className='groups-card-top'>
                    <span
                      className='groups-card-icon'
                      style={{ background: getAvatarColor(`group-${g.id}`) }}
                      aria-hidden='true'>
                      {g.title?.trim()?.[0]?.toUpperCase() || "G"}
                    </span>
                    <div className='groups-card-heading'>
                      <h3 className='groups-card-title'>{g.title}</h3>
                      <span className={`groups-status groups-status--${status.tone}`}>
                        {status.label}
                      </span>
                    </div>
                  </div>
                  <p className='groups-card-desc'>
                    {g.description || "No description yet."}
                  </p>
                  <div className='groups-card-meta'>
                    <span className='groups-card-pill'>
                      👥 {g.member_count ?? 0} member{(g.member_count ?? 0) === 1 ? "" : "s"}
                    </span>
                  </div>
                </Link>
              );
            })}
          </div>
        )}
      </section>
      </div>
    </div>
  );
}
