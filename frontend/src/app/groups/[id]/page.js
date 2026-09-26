"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import { toast } from "@/context/ToastContext";
import { apiFetch } from "@/lib/api";
import { useRequireAuth } from "@/hooks/useRequireAuth";
import { useWebSocket } from "@/context/WebSocketContext";
import { useLivePosts } from "@/hooks/useLivePosts";
import PostForm from "@/components/PostForm";
import PostCard from "@/components/PostCard";
import GroupChat from "@/components/GroupChat";
import GroupEvents from "@/components/GroupEvents";
import GroupInviteButton from "@/components/GroupInviteButton";

// Notifications that change who's in a group (related_id = the group).
const GROUP_MEMBERSHIP_NOTIFICATIONS = new Set([
  "group_request",
  "group_request_accepted",
  "group_request_declined",
  "group_invitation",
  "group_invitation_accepted",
  "group_invitation_declined",
]);

export default function GroupDetailPage() {
  const params = useParams();
  const router = useRouter();
  const groupId = params.id;
  const { user, loading: authLoading } = useRequireAuth();
  const { subscribe } = useWebSocket();

  const [group, setGroup] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [members, setMembers] = useState([]);
  const [pendingRequests, setPendingRequests] = useState([]);
  const [posts, setPosts] = useState([]);
  const [postsLoading, setPostsLoading] = useState(false);
  // Set if the chat reports the viewer isn't (or is no longer) an active
  // member - e.g. they left the group in another tab - to hide the panel.
  const [chatUnavailable, setChatUnavailable] = useState(false);
  // Which half is shown on phones, where posts and chat can't sit side by
  // side. Ignored on wider screens (see .group-layout in globals.css).
  const [mobileTab, setMobileTab] = useState("posts");

  const handleChatUnavailable = useCallback(() => setChatUnavailable(true), []);

  // Show newly invited people in the member list straight away, marked
  // "Invited", without reloading the page.
  const handleInvited = (users) => {
    setMembers((prev) => [
      ...prev,
      ...users
        .filter((u) => !prev.some((m) => m.user?.id === u.id))
        .map((u) => ({ user: u, status: "invited" })),
    ]);
  };

  // An "event_created" notification links to /groups/<id>#events. The
  // section only exists once the group has loaded, so the browser's own
  // jump-to-hash misses it - scroll there ourselves (showing the posts half
  // on phones, where the Events section lives).
  const isMemberLoaded = !!group?.is_member;
  useEffect(() => {
    if (!isMemberLoaded || window.location.hash !== "#events") return;
    setMobileTab("posts");
    requestAnimationFrame(() => {
      document.getElementById("events")?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  }, [isMemberLoaded]);

  useLivePosts(setPosts, {
    accept: (post) => String(post.group_id) === String(groupId),
    enabled: !!group?.is_member,
  });

  const fetchGroupPosts = useCallback(async () => {
    setPostsLoading(true);
    try {
      const { ok, data } = await apiFetch(`/group/posts?group_id=${groupId}`);
      setPosts(ok && Array.isArray(data) ? data : []);
    } catch (err) {
      console.error("Error fetching group posts:", err);
      setPosts([]);
    } finally {
      setPostsLoading(false);
    }
  }, [groupId]);

  // (Re)load the group and the viewer's place in it. Called on open, after
  // the viewer's own join/accept/decline actions, and whenever a live
  // notification says their membership (or the pending requests /
  // invitations they manage) changed - so e.g. a "Request Pending" page
  // turns into the member view the moment the creator accepts.
  const loadGroup = useCallback(async () => {
    try {
      const { ok, status, data } = await apiFetch(`/group/detail?id=${groupId}`);

      if (!ok) {
        if (status === 404) setError("Group not found");
        else setError("Failed to load group");
        return;
      }

      setGroup(data);
      setMembers(data.members || []);
      setPendingRequests(data.pending_requests || []);
      setChatUnavailable(false);

      if (data.is_member) {
        fetchGroupPosts();
      } else {
        setPosts([]);
      }
    } catch (err) {
      console.error("Error fetching group:", err);
      setError("Network error");
    } finally {
      setLoading(false);
    }
  }, [groupId, fetchGroupPosts]);

  useEffect(() => {
    if (!user || !groupId) return;
    loadGroup();
  }, [groupId, user, loadGroup]);

  // Live membership changes for this group, from the notifications the
  // backend already pushes: the viewer's join request answered, an
  // invitation received, a new join request (creator), an invitation
  // answered (inviter).
  useEffect(() => {
    return subscribe("notification", (n) => {
      if (!GROUP_MEMBERSHIP_NOTIFICATIONS.has(n.type)) return;
      if (String(n.related_id) !== String(groupId)) return;
      loadGroup();
    });
  }, [subscribe, groupId, loadGroup]);

  const handlePostCreated = () => {
    fetchGroupPosts();
  };

  const handlePostDeleted = (postId) => {
    setPosts((prev) => prev.filter((p) => p.id !== postId));
  };

  const handleJoinGroup = async () => {
    try {
      const { ok, data } = await apiFetch("/group/join", {
        method: "POST",
        json: { group_id: parseInt(groupId) },
      });

      if (ok) {
        toast.success(data.message || "Request sent successfully");
        loadGroup();
      } else {
        toast.error(data.error || "Failed to join group");
      }
    } catch (err) {
      console.error("Error joining group:", err);
      toast.error("Failed to join group");
    }
  };

  const handleLeaveGroup = async () => {
    if (!confirm("Are you sure you want to leave this group?")) return;

    try {
      const { ok, data } = await apiFetch("/group/leave", {
        method: "POST",
        json: { group_id: parseInt(groupId) },
      });

      if (ok) {
        toast.success("Left group successfully");
        router.push("/groups");
      } else {
        toast.error(data.error || "Failed to leave group");
      }
    } catch (err) {
      console.error("Error leaving group:", err);
      toast.error("Failed to leave group");
    }
  };

  const handleRequestAction = async (requestId, action) => {
    try {
      const { ok, data } = await apiFetch("/group/request", {
        method: "POST",
        json: { request_id: requestId, action },
      });

      if (ok) {
        toast.success(action === "accept" ? "Request accepted" : "Request declined");
        loadGroup();
      } else {
        toast.error(data.error || "Failed to process request");
      }
    } catch (err) {
      console.error("Error processing request:", err);
      toast.error("Failed to process request");
    }
  };

  const handleInvitation = async (action) => {
    try {
      const { ok, data } = await apiFetch("/group/invitation", {
        method: "POST",
        json: { group_id: parseInt(groupId), action },
      });

      if (ok) {
        toast.success(data.message || `Invitation ${action}d`);
        if (action === "accept") {
          loadGroup();
        } else {
          router.push("/groups");
        }
      } else {
        toast.error(data.error || "Failed to respond to invitation");
      }
    } catch (err) {
      console.error("Error responding to invitation:", err);
      toast.error("Failed to respond to invitation");
    }
  };

  if (authLoading || loading) {
    return (
      <div className='group-loading'>
        <div className='group-spinner' />
      </div>
    );
  }

  if (error) {
    return (
      <div className='group-error'>
        <h2 className='group-error-title'>Error</h2>
        <p className='group-error-msg'>{error}</p>
        <button
          onClick={() => router.push("/groups")}
          className='group-back-btn'>
          Back to Groups
        </button>
      </div>
    );
  }

  if (!group) {
    return (
      <div className='group-error'>
        <h2 className='group-error-title'>Group not found</h2>
        <button
          onClick={() => router.push("/groups")}
          className='group-back-btn'>
          Back to Groups
        </button>
      </div>
    );
  }

  const isMember = group.is_member || false;
  const showChat = isMember && !chatUnavailable;

  return (
    <div className={`group-page ${showChat ? "group-page--split" : ""}`}>
      <button
        onClick={() => router.push("/groups")}
        className='group-back-link'>
        ← Back to Groups
      </button>

      {showChat && (
        <div className='group-tabs' role='tablist' aria-label='Group view'>
          <button
            type='button'
            role='tab'
            aria-selected={mobileTab === "posts"}
            className={`group-tab ${mobileTab === "posts" ? "group-tab--active" : ""}`}
            onClick={() => setMobileTab("posts")}>
            Posts
          </button>
          <button
            type='button'
            role='tab'
            aria-selected={mobileTab === "chat"}
            className={`group-tab ${mobileTab === "chat" ? "group-tab--active" : ""}`}
            onClick={() => setMobileTab("chat")}>
            Chat
          </button>
        </div>
      )}

      <div className='group-layout'>
        <div
          className={`group-card group-main ${
            showChat && mobileTab !== "posts" ? "group-panel--mobile-hidden" : ""
          }`}>
          {/* Header */}
          <div className='group-header'>
            <div className='group-header-left'>
              <h1 className='group-title'>{group.title}</h1>
              <p className='group-description'>
                {group.description || "No description"}
              </p>
              <div className='group-meta'>
                <span>👥 {group.member_count || 0} members</span>
                <span>
                  Created {new Date(group.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>

            <div className='group-header-actions'>
              {!isMember && !group.has_pending_request && !group.is_invited && (
                <button onClick={handleJoinGroup} className='group-join-btn'>
                  Join Group
                </button>
              )}
              {group.has_pending_request && (
                <span className='group-status group-status--pending'>
                  Request Pending
                </span>
              )}
              {group.is_invited && (
                <>
                  <span className='group-status group-status--invited'>
                    You&apos;re invited
                  </span>
                  <button
                    onClick={() => handleInvitation("accept")}
                    className='group-accept-btn'>
                    Accept
                  </button>
                  <button
                    onClick={() => handleInvitation("decline")}
                    className='group-decline-btn'>
                    Decline
                  </button>
                </>
              )}
              {isMember && (
                <GroupInviteButton
                  groupId={groupId}
                  members={members}
                  onInvited={handleInvited}
                />
              )}
              {isMember && (
                <button onClick={handleLeaveGroup} className='group-leave-btn'>
                  Leave
                </button>
              )}
            </div>
          </div>

          {/* Events - members only */}
          {isMember && <GroupEvents groupId={groupId} />}

          {/* Posts - only group members can create or see group posts */}
          {isMember && (
            <div className='group-section'>
              <h3 className='group-section-title'>Posts</h3>
              <div className='group-section-body'>
                <PostForm groupId={groupId} onPostCreated={handlePostCreated} />

                {postsLoading ? (
                  <p className='group-empty'>Loading posts...</p>
                ) : posts.length === 0 ? (
                  <p className='group-empty'>
                    No posts yet - be the first to post!
                  </p>
                ) : (
                  posts.map((post) => (
                    <PostCard
                      key={post.id}
                      post={post}
                      onDelete={handlePostDeleted}
                    />
                  ))
                )}
              </div>
            </div>
          )}

          {/* Pending join requests (creator only) */}
          {group.creator_id === user?.id && pendingRequests.length > 0 && (
            <div className='group-section'>
              <h3 className='group-section-title'>Pending Join Requests</h3>
              <div className='group-section-body'>
                {pendingRequests.map((req) => (
                  <div key={req.id} className='group-member-row'>
                    <div>
                      <p className='group-member-name'>
                        {req.user.first_name} {req.user.last_name}
                      </p>
                      <p className='group-member-nick'>
                        @{req.user.nickname || req.user.first_name}
                      </p>
                    </div>
                    <div className='group-member-actions'>
                      <button
                        onClick={() => handleRequestAction(req.id, "accept")}
                        className='group-accept-btn'>
                        Accept
                      </button>
                      <button
                        onClick={() => handleRequestAction(req.id, "decline")}
                        className='group-decline-btn'>
                        Decline
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Members */}
          <div className='group-section'>
            <h3 className='group-section-title'>
              Members ({members.filter((m) => m.status !== "invited").length})
            </h3>
            <div className='group-section-body'>
              {members.length === 0 ? (
                <p className='group-empty'>No members yet</p>
              ) : (
                members.map((member) => (
                  <div key={member.user.id} className='group-member-row'>
                    <div
                      className='group-member-info'
                      onClick={() => router.push(`/profile/${member.user.id}`)}>
                      <div className='group-avatar'>
                        {member.user.first_name?.[0]}
                        {member.user.last_name?.[0]}
                      </div>
                      <div>
                        <p className='group-member-name'>
                          {member.user.first_name} {member.user.last_name}
                        </p>
                        {member.user.nickname && (
                          <p className='group-member-nick'>
                            @{member.user.nickname}
                          </p>
                        )}
                      </div>
                    </div>
                    <div className='group-member-badges'>
                      {member.user.id === group.creator_id && (
                        <span className='group-badge group-badge--creator'>
                          Creator
                        </span>
                      )}
                      {member.status === "invited" && (
                        <span className='group-badge group-badge--invited'>
                          Invited
                        </span>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>

        {showChat && (
          <aside
            className={`group-chat-panel ${
              mobileTab !== "chat" ? "group-panel--mobile-hidden" : ""
            }`}>
            <GroupChat
              groupId={groupId}
              members={members}
              onUnavailable={handleChatUnavailable}
            />
          </aside>
        )}
      </div>
    </div>
  );
}
