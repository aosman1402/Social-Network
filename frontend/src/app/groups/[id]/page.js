"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import toast from "react-hot-toast";
import GroupChat from "@/components/GroupChat";

export default function GroupDetailPage() {
  const params = useParams();
  const router = useRouter();
  const groupId = params.id;
  const [group, setGroup] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [members, setMembers] = useState([]);
  const [pendingRequests, setPendingRequests] = useState([]);
  const [showInviteModal, setShowInviteModal] = useState(false);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviting, setInviting] = useState(false);
  const [activeTab, setActiveTab] = useState("posts");
  const [currentUser, setCurrentUser] = useState(null);
  const [isMember, setIsMember] = useState(false);
  const [isCreator, setIsCreator] = useState(false);
  const [events, setEvents] = useState([]);
  const [showEventModal, setShowEventModal] = useState(false);
  const [eventForm, setEventForm] = useState({
    title: "",
    description: "",
    event_time: "",
  });
  const [creatingEvent, setCreatingEvent] = useState(false);

  // Get current user
  useEffect(() => {
    const getCurrentUser = async () => {
      try {
        const response = await fetch("http://localhost:8081/api/check-auth", {
          credentials: "include",
        });
        if (response.ok) {
          const data = await response.json();
          setCurrentUser(data);
        }
      } catch (error) {
        console.error("Error getting current user:", error);
      }
    };
    getCurrentUser();
  }, []);

  // Fetch group details
  useEffect(() => {
    const fetchGroup = async () => {
      try {
        setLoading(true);
        setError(null);

        const authRes = await fetch("http://localhost:8081/api/check-auth", {
          credentials: "include",
        });
        if (!authRes.ok) {
          router.push("/login");
          return;
        }

        const response = await fetch(
          `http://localhost:8081/api/group/detail?id=${groupId}`,
          {
            credentials: "include",
          },
        );

        if (!response.ok) {
          if (response.status === 404) {
            setError("Group not found");
          } else {
            setError("Failed to load group");
          }
          return;
        }

        const data = await response.json();
        setGroup(data);
        setMembers(data.members || []);
        setPendingRequests(data.pending_requests || []);
        setIsMember(data.is_member || false);
        setIsCreator(data.creator_id === currentUser?.id);

        // Fetch events if member
        if (data.is_member) {
          await fetchEvents();
        }
      } catch (error) {
        console.error("Error fetching group:", error);
        setError("Network error");
      } finally {
        setLoading(false);
      }
    };

    if (groupId) {
      fetchGroup();
    }
  }, [groupId, router, currentUser?.id]);

  const fetchEvents = async () => {
    try {
      const response = await fetch(
        `http://localhost:8081/api/group/events?group_id=${groupId}`,
        {
          credentials: "include",
        },
      );
      if (response.ok) {
        const data = await response.json();
        setEvents(data);
      }
    } catch (error) {
      console.error("Error fetching events:", error);
    }
  };

  const handleJoinGroup = async () => {
    try {
      const response = await fetch("http://localhost:8081/api/group/join", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ group_id: parseInt(groupId) }),
      });

      const data = await response.json();

      if (response.ok) {
        toast.success(data.message || "Request sent successfully");
        // Refresh group data
        window.location.reload();
      } else {
        toast.error(data.error || "Failed to join group");
      }
    } catch (error) {
      console.error("Error joining group:", error);
      toast.error("Failed to join group");
    }
  };

  const handleLeaveGroup = async () => {
    if (!confirm("Are you sure you want to leave this group?")) return;

    try {
      const response = await fetch("http://localhost:8081/api/group/leave", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
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
    } catch (error) {
      console.error("Error leaving group:", error);
      toast.error("Failed to leave group");
    }
  };

  const handleInvite = async (e) => {
    e.preventDefault();
    if (!inviteEmail.trim()) {
      toast.error("Please enter an email");
      return;
    }

    setInviting(true);
    try {
      // First find user by email
      const userRes = await fetch(
        `http://localhost:8081/api/user/search?email=${inviteEmail}`,
        {
          credentials: "include",
        },
      );

      if (!userRes.ok) {
        toast.error("User not found");
        return;
      }

      const userData = await userRes.json();
      if (!userData.id) {
        toast.error("User not found");
        return;
      }

      const response = await fetch("http://localhost:8081/api/group/invite", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          user_id: userData.id,
          group_id: parseInt(groupId),
        }),
      });

      const data = await response.json();

      if (response.ok) {
        toast.success("Invitation sent successfully");
        setInviteEmail("");
        setShowInviteModal(false);
      } else {
        toast.error(data.error || "Failed to send invitation");
      }
    } catch (error) {
      console.error("Error sending invitation:", error);
      toast.error("Failed to send invitation");
    } finally {
      setInviting(false);
    }
  };

  const handleRequestAction = async (requestId, action) => {
    try {
      const response = await fetch("http://localhost:8081/api/group/request", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          request_id: requestId,
          action: action,
        }),
      });

      if (response.ok) {
        toast.success(`Request ${action}ed`);
        window.location.reload();
      } else {
        const data = await response.json();
        toast.error(data.error || "Failed to process request");
      }
    } catch (error) {
      console.error("Error processing request:", error);
      toast.error("Failed to process request");
    }
  };

  const handleCreateEvent = async (e) => {
    e.preventDefault();
    if (!eventForm.title.trim() || !eventForm.event_time) {
      toast.error("Title and time are required");
      return;
    }

    setCreatingEvent(true);
    try {
      const response = await fetch(
        "http://localhost:8081/api/group/event/create",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({
            group_id: parseInt(groupId),
            title: eventForm.title,
            description: eventForm.description,
            event_time: new Date(eventForm.event_time).toISOString(),
          }),
        },
      );

      const data = await response.json();

      if (response.ok) {
        toast.success("Event created successfully!");
        setShowEventModal(false);
        setEventForm({ title: "", description: "", event_time: "" });
        await fetchEvents();
      } else {
        toast.error(data.error || "Failed to create event");
      }
    } catch (error) {
      console.error("Error creating event:", error);
      toast.error("Failed to create event");
    } finally {
      setCreatingEvent(false);
    }
  };

  const handleEventResponse = async (eventId, response) => {
    try {
      const res = await fetch("http://localhost:8081/api/group/event/respond", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          event_id: eventId,
          response: response,
        }),
      });

      if (res.ok) {
        toast.success(`You are ${response}`);
        await fetchEvents();
      } else {
        const data = await res.json();
        toast.error(data.error || "Failed to respond to event");
      }
    } catch (error) {
      console.error("Error responding to event:", error);
      toast.error("Failed to respond to event");
    }
  };

  const formatDate = (dateString) => {
    if (!dateString) return "";
    try {
      const date = new Date(dateString);
      return date.toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return "";
    }
  };

  if (loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600'></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className='max-w-4xl mx-auto py-12 px-4 text-center'>
        <h2 className='text-2xl font-bold text-red-600'>Error</h2>
        <p className='text-gray-600 mt-2'>{error}</p>
        <button
          onClick={() => router.push("/groups")}
          className='mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600'>
          Back to Groups
        </button>
      </div>
    );
  }

  if (!group) {
    return (
      <div className='max-w-4xl mx-auto py-12 px-4 text-center'>
        <h2 className='text-2xl font-bold text-gray-600'>Group not found</h2>
        <button
          onClick={() => router.push("/groups")}
          className='mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600'>
          Back to Groups
        </button>
      </div>
    );
  }

  return (
    <div className='max-w-4xl mx-auto py-4 px-4'>
      {/* Back Button */}
      <button
        onClick={() => router.push("/groups")}
        className='text-blue-500 hover:text-blue-700 mb-4 flex items-center'>
        ← Back to Groups
      </button>

      {/* Group Header */}
      <div className='bg-white rounded-lg shadow'>
        <div className='p-6 border-b'>
          <div className='flex items-start justify-between'>
            <div>
              <h1 className='text-2xl font-bold'>{group.title}</h1>
              <p className='text-gray-600 mt-1'>
                {group.description || "No description"}
              </p>
              <div className='flex items-center space-x-4 mt-2'>
                <span className='text-sm text-gray-500'>
                  👥 {group.member_count || 0} members
                </span>
                <span className='text-sm text-gray-500'>
                  Created {new Date(group.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>
            <div className='flex space-x-2'>
              {!isMember && !group.has_pending_request && !group.is_invited && (
                <button
                  onClick={handleJoinGroup}
                  className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition'>
                  Join Group
                </button>
              )}
              {group.has_pending_request && (
                <span className='px-4 py-2 bg-yellow-100 text-yellow-700 rounded-lg'>
                  Request Pending
                </span>
              )}
              {group.is_invited && (
                <span className='px-4 py-2 bg-green-100 text-green-700 rounded-lg'>
                  Invited
                </span>
              )}
              {isMember && (
                <>
                  <button
                    onClick={() => setShowInviteModal(true)}
                    className='px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition'>
                    + Invite
                  </button>
                  {!isCreator && (
                    <button
                      onClick={handleLeaveGroup}
                      className='px-4 py-2 bg-red-500 text-white rounded-lg hover:bg-red-600 transition'>
                      Leave
                    </button>
                  )}
                </>
              )}
            </div>
          </div>
        </div>

        {/* Pending Requests (only for creator) */}
        {isCreator && pendingRequests.length > 0 && (
          <div className='p-4 border-b'>
            <h3 className='font-semibold mb-2'>Pending Join Requests</h3>
            <div className='space-y-2'>
              {pendingRequests.map((req) => (
                <div
                  key={req.id}
                  className='flex items-center justify-between bg-gray-50 p-2 rounded'>
                  <div>
                    <p className='font-medium'>
                      {req.user.first_name} {req.user.last_name}
                    </p>
                    <p className='text-sm text-gray-500'>
                      @{req.user.nickname || req.user.first_name}
                    </p>
                  </div>
                  <div className='flex space-x-2'>
                    <button
                      onClick={() => handleRequestAction(req.id, "accept")}
                      className='px-3 py-1 bg-green-500 text-white rounded hover:bg-green-600 text-sm'>
                      Accept
                    </button>
                    <button
                      onClick={() => handleRequestAction(req.id, "decline")}
                      className='px-3 py-1 bg-red-500 text-white rounded hover:bg-red-600 text-sm'>
                      Decline
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Tabs */}
        <div className='flex border-b overflow-x-auto'>
          <button
            onClick={() => setActiveTab("posts")}
            className={`px-4 py-3 text-sm font-medium transition whitespace-nowrap ${
              activeTab === "posts"
                ? "text-blue-600 border-b-2 border-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}>
            Posts
          </button>
          <button
            onClick={() => setActiveTab("events")}
            className={`px-4 py-3 text-sm font-medium transition whitespace-nowrap ${
              activeTab === "events"
                ? "text-blue-600 border-b-2 border-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}>
            Events
          </button>
          <button
            onClick={() => setActiveTab("members")}
            className={`px-4 py-3 text-sm font-medium transition whitespace-nowrap ${
              activeTab === "members"
                ? "text-blue-600 border-b-2 border-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}>
            Members
          </button>
          <button
            onClick={() => setActiveTab("chat")}
            className={`px-4 py-3 text-sm font-medium transition whitespace-nowrap ${
              activeTab === "chat"
                ? "text-blue-600 border-b-2 border-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}>
            💬 Chat
          </button>
        </div>

        {/* Tab Content */}
        <div className='p-4'>
          {/* Posts Tab */}
          {activeTab === "posts" && (
            <div>
              {isMember ? (
                <div className='text-center text-gray-400 py-8'>
                  <p>Group posts will appear here</p>
                  <p className='text-sm mt-2'>Create a post in the group!</p>
                </div>
              ) : (
                <div className='text-center py-8 text-gray-500'>
                  <p>Join the group to see posts</p>
                </div>
              )}
            </div>
          )}

          {/* Events Tab */}
          {activeTab === "events" && (
            <div>
              {isMember ? (
                <>
                  <button
                    onClick={() => setShowEventModal(true)}
                    className='mb-4 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition'>
                    + Create Event
                  </button>
                  <div className='space-y-3'>
                    {events.length === 0 ? (
                      <div className='text-center text-gray-400 py-8'>
                        <p>No events yet</p>
                        <p className='text-sm mt-2'>Create the first event!</p>
                      </div>
                    ) : (
                      events.map((event) => (
                        <div
                          key={event.id}
                          className='bg-gray-50 rounded-lg p-4 border'>
                          <div className='flex items-start justify-between'>
                            <div>
                              <h4 className='font-semibold'>{event.title}</h4>
                              <p className='text-sm text-gray-600'>
                                {event.description}
                              </p>
                              <p className='text-sm text-gray-500 mt-1'>
                                📅 {formatDate(event.event_time)}
                              </p>
                              <p className='text-xs text-gray-400 mt-1'>
                                Created by {event.creator?.first_name}{" "}
                                {event.creator?.last_name}
                              </p>
                              <div className='flex items-center space-x-4 mt-2'>
                                <span className='text-sm text-gray-500'>
                                  ✅ Going:{" "}
                                  {event.responses?.filter(
                                    (r) => r.response === "going",
                                  ).length || 0}
                                </span>
                                <span className='text-sm text-gray-500'>
                                  ❌ Not going:{" "}
                                  {event.responses?.filter(
                                    (r) => r.response === "not_going",
                                  ).length || 0}
                                </span>
                              </div>
                            </div>
                            <div className='flex space-x-2'>
                              <button
                                onClick={() =>
                                  handleEventResponse(event.id, "going")
                                }
                                className={`px-3 py-1 text-sm rounded ${
                                  event.user_response === "going"
                                    ? "bg-green-500 text-white"
                                    : "bg-gray-200 text-gray-600 hover:bg-gray-300"
                                }`}>
                                Going
                              </button>
                              <button
                                onClick={() =>
                                  handleEventResponse(event.id, "not_going")
                                }
                                className={`px-3 py-1 text-sm rounded ${
                                  event.user_response === "not_going"
                                    ? "bg-red-500 text-white"
                                    : "bg-gray-200 text-gray-600 hover:bg-gray-300"
                                }`}>
                                Not Going
                              </button>
                            </div>
                          </div>
                        </div>
                      ))
                    )}
                  </div>
                </>
              ) : (
                <div className='text-center py-8 text-gray-500'>
                  <p>Join the group to see events</p>
                </div>
              )}
            </div>
          )}

          {/* Members Tab */}
          {activeTab === "members" && (
            <div className='space-y-2'>
              {members.length === 0 ? (
                <p className='text-center text-gray-400 py-4'>No members yet</p>
              ) : (
                members.map((member) => (
                  <div
                    key={member.user.id}
                    className='flex items-center justify-between bg-gray-50 p-2 rounded'>
                    <div
                      className='flex items-center space-x-3 cursor-pointer'
                      onClick={() => router.push(`/profile/${member.user.id}`)}>
                      <div className='w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-sm font-bold'>
                        {member.user.first_name?.[0]}
                        {member.user.last_name?.[0]}
                      </div>
                      <div>
                        <p className='font-medium'>
                          {member.user.first_name} {member.user.last_name}
                        </p>
                        {member.user.nickname && (
                          <p className='text-sm text-gray-500'>
                            @{member.user.nickname}
                          </p>
                        )}
                      </div>
                    </div>
                    {member.user.id === group.creator_id && (
                      <span className='text-xs bg-yellow-100 text-yellow-700 px-2 py-1 rounded'>
                        Creator
                      </span>
                    )}
                  </div>
                ))
              )}
            </div>
          )}

          {/* Chat Tab */}
          {activeTab === "chat" && (
            <div>
              {isMember ? (
                <GroupChat groupId={groupId} />
              ) : (
                <div className='text-center py-8 text-gray-500'>
                  <p>Join the group to chat!</p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Invite Modal */}
      {showInviteModal && (
        <div className='fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4'>
          <div className='bg-white rounded-lg max-w-md w-full p-6'>
            <h2 className='text-xl font-bold mb-4'>Invite to Group</h2>
            <form onSubmit={handleInvite} className='space-y-4'>
              <input
                type='email'
                placeholder="Enter user's email"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                className='w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                required
              />
              <div className='flex justify-end space-x-2'>
                <button
                  type='button'
                  onClick={() => setShowInviteModal(false)}
                  className='px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg'>
                  Cancel
                </button>
                <button
                  type='submit'
                  disabled={inviting}
                  className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
                  {inviting ? "Sending..." : "Send Invite"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Event Modal */}
      {showEventModal && (
        <div className='fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4'>
          <div className='bg-white rounded-lg max-w-md w-full p-6'>
            <h2 className='text-xl font-bold mb-4'>Create Event</h2>
            <form onSubmit={handleCreateEvent} className='space-y-4'>
              <input
                type='text'
                placeholder='Event Title *'
                value={eventForm.title}
                onChange={(e) =>
                  setEventForm({ ...eventForm, title: e.target.value })
                }
                className='w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                required
              />
              <textarea
                placeholder='Description'
                value={eventForm.description}
                onChange={(e) =>
                  setEventForm({ ...eventForm, description: e.target.value })
                }
                rows='3'
                className='w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none'
              />
              <input
                type='datetime-local'
                value={eventForm.event_time}
                onChange={(e) =>
                  setEventForm({ ...eventForm, event_time: e.target.value })
                }
                className='w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                required
              />
              <div className='flex justify-end space-x-2'>
                <button
                  type='button'
                  onClick={() => setShowEventModal(false)}
                  className='px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg'>
                  Cancel
                </button>
                <button
                  type='submit'
                  disabled={creatingEvent}
                  className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
                  {creatingEvent ? "Creating..." : "Create Event"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
