"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { toast } from "@/context/ToastContext";

export default function GroupDetailPage() {
  const params = useParams();
  const router = useRouter();
  const groupId = params.id;

  const [group, setGroup] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [members, setMembers] = useState([]);
  const [pendingRequests, setPendingRequests] = useState([]);

  useEffect(() => {
    const fetchGroup = async () => {
      try {
        const authRes = await fetch("http://localhost:8081/api/check-auth", {
          credentials: "include",
        });
        const authData = await authRes.json();
        if (!authData.authenticated) {
          router.push("/login");
          return;
        }

        const response = await fetch(
          `http://localhost:8081/api/group/detail?id=${groupId}`,
          { credentials: "include" },
        );

        if (!response.ok) {
          if (response.status === 404) setError("Group not found");
          else setError("Failed to load group");
          return;
        }

        const data = await response.json();
        setGroup(data);
        setMembers(data.members || []);
        setPendingRequests(data.pending_requests || []);
      } catch (err) {
        console.error("Error fetching group:", err);
        setError("Network error");
      } finally {
        setLoading(false);
      }
    };

    if (groupId) fetchGroup();
  }, [groupId, router]);

  const handleJoinGroup = async () => {
    try {
      const response = await fetch("http://localhost:8081/api/group/join", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ group_id: parseInt(groupId) }),
      });
      const data = await response.json();

      if (response.ok) {
        toast.success(data.message || "Request sent successfully");
        window.location.reload();
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
      const response = await fetch("http://localhost:8081/api/group/leave", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ group_id: parseInt(groupId) }),
      });

      if (response.ok) {
        toast.success("Left group successfully");
        router.push("/groups");
      } else {
        const data = await response.json();
        toast.error(data.error || "Failed to leave group");
      }
    } catch (err) {
      console.error("Error leaving group:", err);
      toast.error("Failed to leave group");
    }
  };

  const handleRequestAction = async (requestId, action) => {
    try {
      const response = await fetch("http://localhost:8081/api/group/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ request_id: requestId, action }),
      });

      if (response.ok) {
        toast.success(`Request ${action}ed`);
        window.location.reload();
      } else {
        const data = await response.json();
        toast.error(data.error || "Failed to process request");
      }
    } catch (err) {
      console.error("Error processing request:", err);
      toast.error("Failed to process request");
    }
  };

  if (loading) {
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

  return (
    <div className='group-page'>
      <button
        onClick={() => router.push("/groups")}
        className='group-back-link'>
        ← Back to Groups
      </button>

      <div className='group-card'>
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
              <span className='group-status group-status--invited'>
                Invited
              </span>
            )}
            {isMember && (
              <button onClick={handleLeaveGroup} className='group-leave-btn'>
                Leave
              </button>
            )}
          </div>
        </div>

        {/* Pending join requests (creator only) */}
        {group.creator_id ===
          parseInt(
            typeof window !== "undefined"
              ? localStorage.getItem("userId") || "0"
              : "0",
          ) &&
          pendingRequests.length > 0 && (
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
          <h3 className='group-section-title'>Members ({members.length})</h3>
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
    </div>
  );
}
