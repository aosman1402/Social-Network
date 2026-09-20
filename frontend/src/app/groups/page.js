"use client";

import { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";
import { useRequireAuth } from "@/hooks/useRequireAuth";

export default function GroupsPage() {
  const router = useRouter();
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
                          <div className='groups-user-nick'>
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
