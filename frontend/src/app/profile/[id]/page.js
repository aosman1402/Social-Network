"use client";

import { useState, useEffect, useRef } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { toast } from "@/context/ToastContext";
import { useWebSocket } from "@/context/WebSocketContext";
import FollowRequests from "@/components/FollowRequests";
import Avatar from "@/components/Avatar";
import { apiFetch } from "@/lib/api";
import LoadingSpinner from "@/components/LoadingSpinner";
import { useRequireAuth } from "@/hooks/useRequireAuth";

const VALID_TABS = ["posts", "followers", "following", "requests"];

export default function ProfileDetailPage() {
  const params = useParams();
  const router = useRouter();
  const searchParams = useSearchParams();
  const { subscribe } = useWebSocket();
  const userId = params.id;
  const { user: currentUser, loading: authLoading } = useRequireAuth();
  const isOwnProfile = currentUser?.id === parseInt(userId);

  // A notification (e.g. "X requested to follow you") can deep-link here
  // with ?tab=requests so the recipient lands straight on the tab that
  // handles it, instead of just the default Posts tab.
  const initialTab = VALID_TABS.includes(searchParams.get("tab"))
    ? searchParams.get("tab")
    : "posts";

  const [profile, setProfile] = useState(null);
  const [userPosts, setUserPosts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadingPosts, setLoadingPosts] = useState(false);
  const [error, setError] = useState(null);
  const [activeTab, setActiveTab] = useState(initialTab);
  const [followers, setFollowers] = useState([]);
  const [following, setFollowing] = useState([]);
  const [pendingRequestsCount, setPendingRequestsCount] = useState(0);
  const redirectTimeoutRef = useRef(null);

  // Clears the pending redirect if the page unmounts (e.g. the user
  // navigates away manually) - otherwise the stale timer would still fire
  // later and yank them back to Home from wherever they'd gone instead.
  useEffect(() => {
    return () => clearTimeout(redirectTimeoutRef.current);
  }, []);

  // Redirects Home 5s after a follow/follow-request action - the toast
  // already shown by the caller covers the "what happened" message.
  const scheduleHomeRedirect = () => {
    clearTimeout(redirectTimeoutRef.current);
    redirectTimeoutRef.current = setTimeout(() => {
      router.push("/");
    }, 5000);
  };

  // A notification can deep-link here with a different ?tab= while this
  // page is already mounted (e.g. still on your own profile when a follow
  // request arrives) - the App Router reuses the existing page instance in
  // that case rather than remounting it, so activeTab has to be re-synced
  // from the URL explicitly instead of relying on the initialTab useState
  // above, which only ever runs once.
  useEffect(() => {
    const tab = searchParams.get("tab");
    if (VALID_TABS.includes(tab)) {
      setActiveTab(tab);
    }
  }, [searchParams]);

  // Bump the "Requests (N)" tab label live when a follow request comes in,
  // even while sitting on a different tab - FollowRequests (which owns the
  // authoritative count once it's mounted) only exists while the Requests
  // tab itself is open, so it can't pick this up on its own.
  useEffect(() => {
    if (!isOwnProfile) return;
    return subscribe("notification", (notification) => {
      if (notification.type === "follow_request") {
        setPendingRequestsCount((prev) => prev + 1);
      }
    });
  }, [isOwnProfile, subscribe]);

  // Mirror the drop when a still-pending request gets cancelled, for the
  // same reason as above - FollowRequests can only correct the count
  // itself while the Requests tab is the one open.
  useEffect(() => {
    if (!isOwnProfile) return;
    return subscribe("follow_request_cancelled", () => {
      setPendingRequestsCount((prev) => Math.max(0, prev - 1));
    });
  }, [isOwnProfile, subscribe]);

  // ✅ Format date function
  const formatDate = (dateString, fallback = "Recently joined") => {
    if (!dateString) return fallback;
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return fallback;
      return date.toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    } catch {
      return fallback;
    }
  };

  // ✅ Format post time
  const formatPostTime = (dateString) => {
    if (!dateString) return "";
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return "";
      return date.toLocaleDateString("en-US", {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return "";
    }
  };

  // ✅ Fetch profile data
  useEffect(() => {
    if (!currentUser || !userId) return;

    const fetchProfile = async () => {
      try {
        setLoading(true);

        // 1. Fetch profile data
        const {
          ok: profileOk,
          status: profileStatus,
          data: profileData,
        } = await apiFetch(`/user/profile?id=${userId}`);

        if (!profileOk) {
          if (profileStatus === 404) {
            setError("User not found");
          } else if (profileStatus === 403) {
            // Private account, not (yet) an accepted follower - the body
            // still carries name/avatar/follow-status so this renders as
            // an identity card with a follow-request button, not a dead end.
            if (profileData) {
              setProfile(profileData);
            } else {
              setError("This profile is private");
            }
          } else {
            setError("Failed to load profile");
          }
          return;
        }

        setProfile(profileData);

        // 3. Fetch user posts
        await fetchUserPosts(userId);

        // 4. Fetch followers and following
        await fetchFollowers(userId);
        await fetchFollowing(userId);

        // 5. On your own profile, get the pending-requests count up front
        // so the "Requests" tab shows it without having to click into it
        // first - FollowRequests itself keeps it live after that.
        if (isOwnProfile) {
          try {
            const { ok: reqOk, data: reqData } = await apiFetch(
              "/user/follow-requests",
            );
            if (reqOk) {
              setPendingRequestsCount(
                Array.isArray(reqData) ? reqData.length : 0,
              );
            }
          } catch (err) {
            console.error("Error fetching follow requests count:", err);
          }
        }
      } catch (err) {
        console.error("Error fetching profile:", err);
        setError("Network error");
      } finally {
        setLoading(false);
      }
    };

    fetchProfile();
  }, [userId, currentUser, isOwnProfile]);

  // ✅ Fetch user posts
  const fetchUserPosts = async (userId) => {
    setLoadingPosts(true);
    try {
      const { ok, data } = await apiFetch(`/user/posts?id=${userId}`);
      setUserPosts(ok && Array.isArray(data) ? data : []);
    } catch (error) {
      console.error("Error fetching user posts:", error);
      setUserPosts([]);
    } finally {
      setLoadingPosts(false);
    }
  };

  // ✅ Fetch followers
  const fetchFollowers = async (userId) => {
    try {
      const { ok, data } = await apiFetch(`/user/followers?id=${userId}`);
      if (ok) {
        setFollowers(Array.isArray(data) ? data : []);
      }
    } catch (error) {
      console.error("Error fetching followers:", error);
    }
  };

  // ✅ Fetch following
  const fetchFollowing = async (userId) => {
    try {
      const { ok, data } = await apiFetch(`/user/following?id=${userId}`);
      if (ok) {
        setFollowing(Array.isArray(data) ? data : []);
      }
    } catch (error) {
      console.error("Error fetching following:", error);
    }
  };

  // ✅ Handle follow/unfollow - also covers cancelling a still-pending
  // request to a private account, and unfollows directly (no separate
  // confirm step) since the backend already applies it immediately.
  const handleFollowToggle = async () => {
    if (!profile) return;

    const wasFollowingOrPending = profile.is_following || profile.is_pending;

    // Only confirm an actual unfollow - cancelling a still-pending request
    // isn't undoing a real relationship, so it doesn't need one.
    if (
      profile.is_following &&
      !confirm(
        `Unfollow ${profile.first_name} ${profile.last_name}? They'll be notified.`,
      )
    ) {
      return;
    }

    try {
      const path = wasFollowingOrPending ? "/user/unfollow" : "/user/follow";

      const { ok, data } = await apiFetch(path, {
        method: "POST",
        json: { user_id: parseInt(userId) },
      });

      if (ok) {
        if (wasFollowingOrPending) {
          // Unfollowed (or cancelled a pending request).
          setProfile({
            ...profile,
            is_following: false,
            is_pending: false,
            // Only an accepted follow ever counted towards followers_count.
            followers_count: profile.is_following
              ? profile.followers_count - 1
              : profile.followers_count,
          });
          setFollowers(
            followers.filter((f) => f.user.id !== currentUser.id),
          );
          toast.success(data.message || "Unfollowed");
        } else if (data.status === "pending") {
          // Private account - request sent, not yet accepted. Don't count
          // it as a follower yet.
          setProfile({ ...profile, is_pending: true });
          toast.success(data.message || "Follow request sent");
          scheduleHomeRedirect();
        } else {
          setProfile({
            ...profile,
            is_following: true,
            followers_count: profile.followers_count + 1,
          });
          setFollowers([...followers, { user: currentUser }]);
          toast.success(data.message || "Followed");
          scheduleHomeRedirect();
        }
      } else {
        toast.error(data.error || "Failed to update follow status");
      }
    } catch (error) {
      console.error("Error toggling follow:", error);
      toast.error("Failed to update follow status");
    }
  };

  // ✅ One-click public/private toggle, right on the profile page - the
  // account was previously only switchable from a Settings form that never
  // actually called the backend. UpdateProfile overwrites first/last
  // name + about_me + is_public together, so the current values have to be
  // sent along even though only is_public is changing here.
  const [togglingPrivacy, setTogglingPrivacy] = useState(false);
  const handleTogglePrivacy = async () => {
    if (!profile || togglingPrivacy) return;
    const newIsPublic = !profile.is_public;

    const confirmMessage = newIsPublic
      ? "Switching your profile to public will automatically accept all of your pending follow requests and notify those users. Continue?"
      : "Switching your profile to private means new followers will need your approval, and only accepted followers will be able to see your posts and profile. Continue?";
    if (!confirm(confirmMessage)) return;

    setTogglingPrivacy(true);

    try {
      const { ok, data } = await apiFetch("/user/update", {
        method: "PUT",
        json: {
          first_name: profile.first_name,
          last_name: profile.last_name,
          about_me: profile.about_me || "",
          is_public: newIsPublic,
        },
      });

      if (ok) {
        setProfile({ ...profile, is_public: newIsPublic });
        // Both directions now earn a self-targeted live notification from
        // the backend - NotificationBell already toasts that, so showing
        // one here too would duplicate it.

        // A privacy flip changes what's visible throughout the page
        // (followers/following eligibility, post visibility, etc.) - a
        // full reload is the simplest way to make sure everything reflects
        // it consistently, rather than patching every derived bit of state
        // by hand. Delayed briefly so the toast from the live notification
        // above has a moment to actually appear first.
        setTimeout(() => window.location.reload(), 600);
      } else {
        toast.error(data.error || "Failed to update privacy");
      }
    } catch (error) {
      console.error("Error toggling privacy:", error);
      toast.error("Failed to update privacy");
    } finally {
      setTogglingPrivacy(false);
    }
  };

  // ✅ Called by FollowRequests after accepting/declining an incoming
  // request, so the followers list/count update in place instead of a
  // full page reload.
  const handleFollowRequestHandled = (action, follower) => {
    if (action !== "accept" || !follower) return;
    setProfile((prev) =>
      prev ? { ...prev, followers_count: prev.followers_count + 1 } : prev,
    );
    setFollowers((prev) => [...prev, { user: follower }]);
  };

  // ✅ Loading state
  if (authLoading || loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <LoadingSpinner size='large' />
      </div>
    );
  }

  // ✅ Error state
  if (error) {
    return (
      <div className='max-w-3xl mx-auto py-12 px-4 text-center'>
        <h2 className='text-2xl font-bold text-red-600'>Error</h2>
        <p className='text-gray-600 mt-2'>{error}</p>
        <button
          onClick={() => router.push("/")}
          className='mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600'>
          Go Home
        </button>
      </div>
    );
  }

  // ✅ Private account, not (yet) an accepted follower - show who it is
  // and a way to request access, instead of a dead-end error page.
  if (profile?.private) {
    return (
      <div className='max-w-md mx-auto py-12 px-4 text-center'>
        <div className='flex justify-center mb-4'>
          <Avatar user={profile} size='2xl' />
        </div>
        <h2 className='text-2xl font-bold'>
          {profile.first_name} {profile.last_name}
        </h2>
        {profile.nickname && (
          <p className='text-gray-500'>@{profile.nickname}</p>
        )}
        <p className='text-gray-600 mt-3'>
          🔒 This profile is private. Follow to see their posts and info.
        </p>
        <div className='mt-4 flex justify-center gap-2'>
          <button
            onClick={handleFollowToggle}
            className={`px-4 py-2 rounded-lg font-medium transition ${
              profile.is_pending
                ? "bg-yellow-100 text-yellow-800 hover:bg-red-100 hover:text-red-700"
                : "bg-blue-500 text-white hover:bg-blue-600"
            }`}>
            {profile.is_pending ? "Cancel Request" : "Follow Request"}
          </button>
          <button
            onClick={() => router.push("/")}
            className='px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition'>
            Go Home
          </button>
        </div>
      </div>
    );
  }

  // ✅ Not found state
  if (!profile) {
    return (
      <div className='max-w-3xl mx-auto py-12 px-4 text-center'>
        <h2 className='text-2xl font-bold text-gray-600'>User not found</h2>
        <button
          onClick={() => router.push("/")}
          className='mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600'>
          Go Home
        </button>
      </div>
    );
  }

  const postsCount = userPosts.length || profile.posts_count || 0;

  return (
    <div className='max-w-3xl mx-auto py-4 px-4'>
      {/* Profile Header */}
      <div className='bg-white rounded-lg shadow'>
        {/* Cover Image */}
        <div className='h-24 bg-gradient-to-r from-blue-400 to-purple-500 rounded-t-lg'></div>

        {/* Profile Info */}
        <div className='px-6 pb-6'>
          <div className='flex flex-col md:flex-row items-start md:items-end -mt-12 mb-4'>
            {/* Avatar */}
            <div className='rounded-full border-4 border-white shadow-lg'>
              <Avatar user={profile} size='2xl' />
            </div>

            {/* User Info */}
            <div className='flex-1 mt-4 md:mt-0 md:ml-4'>
              <div className='flex flex-wrap items-center gap-2'>
                <h1 className='text-2xl font-bold'>
                  {profile.first_name} {profile.last_name}
                </h1>
                {profile.is_public ? (
                  <span className='text-xs bg-green-100 text-green-700 px-2 py-0.5 rounded-full'>
                    Public
                  </span>
                ) : (
                  <span className='text-xs bg-yellow-100 text-yellow-700 px-2 py-0.5 rounded-full'>
                    Private
                  </span>
                )}
              </div>

              {profile.nickname && (
                <p className='text-gray-500'>@{profile.nickname}</p>
              )}

              {profile.about_me && (
                <p className='text-gray-600 mt-1'>{profile.about_me}</p>
              )}

              <div className='flex flex-wrap items-center gap-4 mt-2'>
                <span className='text-sm text-gray-500'>
                  📧 {profile.email}
                </span>
                <span className='text-sm text-gray-500'>
                  📅 Joined {formatDate(profile.created_at)}
                </span>
                {profile.date_of_birth && (
                  <span className='text-sm text-gray-500'>
                    🎂 Born {formatDate(profile.date_of_birth, "Unknown")}
                  </span>
                )}
              </div>
            </div>

            {/* Follow / Edit Button */}
            <div className='mt-4 md:mt-0 flex items-center gap-2'>
              {isOwnProfile ? (
                <>
                  <button
                    onClick={() => router.push("/settings")}
                    className='px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition'>
                    Edit Profile
                  </button>
                  <button
                    onClick={handleTogglePrivacy}
                    disabled={togglingPrivacy}
                    title={
                      profile.is_public
                        ? "Switch to a private profile"
                        : "Switch to a public profile"
                    }
                    className={`px-4 py-2 rounded-lg font-medium transition disabled:opacity-50 ${
                      profile.is_public
                        ? "bg-green-100 text-green-700 hover:bg-yellow-100 hover:text-yellow-800"
                        : "bg-yellow-100 text-yellow-800 hover:bg-green-100 hover:text-green-700"
                    }`}>
                    {togglingPrivacy
                      ? "Saving..."
                      : profile.is_public
                        ? "🌍 Public"
                        : "🔒 Private"}
                  </button>
                </>
              ) : (
                <button
                  onClick={handleFollowToggle}
                  className={`px-4 py-2 rounded-lg font-medium transition ${
                    profile.is_following
                      ? "bg-gray-200 text-gray-700 hover:bg-gray-300"
                      : profile.is_pending
                        ? "bg-yellow-100 text-yellow-800 hover:bg-red-100 hover:text-red-700"
                        : "bg-blue-500 text-white hover:bg-blue-600"
                  }`}>
                  {profile.is_following
                    ? "Following"
                    : profile.is_pending
                      ? "Cancel Request"
                      : "Follow"}
                </button>
              )}
            </div>
          </div>

          {/* Stats */}
          <div className='flex items-center gap-6 mt-4 pt-4 border-t'>
            <button
              onClick={() => setActiveTab("posts")}
              className={`text-center hover:text-blue-500 transition ${activeTab === "posts" ? "text-blue-500" : ""}`}>
              <p className='text-xl font-bold'>{postsCount}</p>
              <p className='text-sm text-gray-500'>Posts</p>
            </button>
            <button
              onClick={() => setActiveTab("followers")}
              className={`text-center hover:text-blue-500 transition ${activeTab === "followers" ? "text-blue-500" : ""}`}>
              <p className='text-xl font-bold'>
                {profile.followers_count || 0}
              </p>
              <p className='text-sm text-gray-500'>Followers</p>
            </button>
            <button
              onClick={() => setActiveTab("following")}
              className={`text-center hover:text-blue-500 transition ${activeTab === "following" ? "text-blue-500" : ""}`}>
              <p className='text-xl font-bold'>
                {profile.following_count || 0}
              </p>
              <p className='text-sm text-gray-500'>Following</p>
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className='mt-4 bg-white rounded-lg shadow'>
        <div className='flex border-b'>
          <button
            onClick={() => setActiveTab("posts")}
            className={`flex-1 py-3 text-sm font-medium transition ${
              activeTab === "posts"
                ? "text-blue-600 border-b-2 border-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}>
            Posts
          </button>
          <button
            onClick={() => setActiveTab("followers")}
            className={`flex-1 py-3 text-sm font-medium transition ${
              activeTab === "followers"
                ? "text-blue-600 border-b-2 border-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}>
            Followers ({profile.followers_count || 0})
          </button>
          <button
            onClick={() => setActiveTab("following")}
            className={`flex-1 py-3 text-sm font-medium transition ${
              activeTab === "following"
                ? "text-blue-600 border-b-2 border-blue-600"
                : "text-gray-500 hover:text-gray-700"
            }`}>
            Following ({profile.following_count || 0})
          </button>
          {isOwnProfile && (
            <button
              onClick={() => setActiveTab("requests")}
              className={`flex-1 py-3 text-sm font-medium transition ${
                activeTab === "requests"
                  ? "text-blue-600 border-b-2 border-blue-600"
                  : "text-gray-500 hover:text-gray-700"
              }`}>
              Requests ({pendingRequestsCount})
            </button>
          )}
        </div>

        <div className='p-4'>
          {/* Posts Tab */}
          {activeTab === "posts" && (
            <div className='space-y-4'>
              {loadingPosts ? (
                <div className='flex justify-center items-center py-8'>
                  <LoadingSpinner size='medium' />
                </div>
              ) : userPosts.length === 0 ? (
                <div className='text-center text-gray-400 py-8'>
                  <p>No posts yet</p>
                  {isOwnProfile && (
                    <p className='text-sm mt-2'>Share your first post!</p>
                  )}
                </div>
              ) : (
                userPosts.map((post) => (
                  <div
                    key={post.id}
                    className='bg-gray-50 rounded-lg p-4 border'>
                    <p className='text-gray-800 whitespace-pre-wrap'>
                      {post.content}
                    </p>

                    {post.image_path && (
                      <img
                        src={`http://localhost:8081${post.image_path}`}
                        alt='Post image'
                        className='mt-2 rounded-lg max-h-60 object-cover'
                      />
                    )}

                    <div className='flex items-center justify-between mt-2'>
                      <p className='text-xs text-gray-400'>
                        {formatPostTime(post.created_at)}
                      </p>
                      {post.privacy_level && (
                        <span className='text-xs bg-gray-200 px-2 py-0.5 rounded'>
                          {post.privacy_level === "public"
                            ? "🌍"
                            : post.privacy_level === "almost_private"
                              ? "👥"
                              : "🔒"}
                        </span>
                      )}
                    </div>

                    <div className='flex items-center gap-4 mt-2 pt-2 border-t'>
                      <span className='text-sm text-gray-500'>
                        💬 {post.comment_count || 0} comments
                      </span>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}

          {/* Followers Tab */}
          {activeTab === "followers" && (
            <div className='space-y-3'>
              {followers.length === 0 ? (
                <p className='text-center text-gray-400 py-4'>
                  No followers yet
                </p>
              ) : (
                followers.map((item) => (
                  <div
                    key={item.user.id}
                    className='flex items-center justify-between p-2 hover:bg-gray-50 rounded cursor-pointer'
                    onClick={() => router.push(`/profile/${item.user.id}`)}>
                    <div className='flex items-center gap-3'>
                      <div className='w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold'>
                        {item.user.first_name?.[0]}
                        {item.user.last_name?.[0]}
                      </div>
                      <div>
                        <p className='font-medium'>
                          {item.user.first_name} {item.user.last_name}
                        </p>
                        {item.user.nickname && (
                          <p className='text-sm text-gray-500'>
                            @{item.user.nickname}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}

          {/* Following Tab */}
          {activeTab === "following" && (
            <div className='space-y-3'>
              {following.length === 0 ? (
                <p className='text-center text-gray-400 py-4'>
                  Not following anyone yet
                </p>
              ) : (
                following.map((item) => (
                  <div
                    key={item.user.id}
                    className='flex items-center justify-between p-2 hover:bg-gray-50 rounded cursor-pointer'
                    onClick={() => router.push(`/profile/${item.user.id}`)}>
                    <div className='flex items-center gap-3'>
                      <div className='w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold'>
                        {item.user.first_name?.[0]}
                        {item.user.last_name?.[0]}
                      </div>
                      <div>
                        <p className='font-medium'>
                          {item.user.first_name} {item.user.last_name}
                        </p>
                        {item.user.nickname && (
                          <p className='text-sm text-gray-500'>
                            @{item.user.nickname}
                          </p>
                        )}
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}

          {/* Requests Tab (own profile only) */}
          {isOwnProfile && activeTab === "requests" && (
            <FollowRequests
              onRequestHandled={handleFollowRequestHandled}
              onCountChange={setPendingRequestsCount}
            />
          )}
        </div>
      </div>
    </div>
  );
}
