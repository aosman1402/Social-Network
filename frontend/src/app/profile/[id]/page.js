"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { toast } from "@/context/ToastContext";

export default function ProfileDetailPage() {
  const params = useParams();
  const router = useRouter();
  const userId = params.id;

  const [profile, setProfile] = useState(null);
  const [userPosts, setUserPosts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadingPosts, setLoadingPosts] = useState(false);
  const [error, setError] = useState(null);
  const [currentUser, setCurrentUser] = useState(null);
  const [activeTab, setActiveTab] = useState("posts");
  const [followers, setFollowers] = useState([]);
  const [following, setFollowing] = useState([]);
  const [isOwnProfile, setIsOwnProfile] = useState(false);

  // ✅ Format date function
  const formatDate = (dateString) => {
    if (!dateString) return "Recently joined";
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return "Recently joined";
      return date.toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    } catch {
      return "Recently joined";
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
    const fetchProfile = async () => {
      try {
        setLoading(true);

        // 1. Check authentication and get current user
        const authRes = await fetch("http://localhost:8081/api/check-auth", {
          credentials: "include",
        });
        const authData = await authRes.json();

        if (!authData.authenticated) {
          router.push("/login");
          return;
        }
        setCurrentUser(authData);

        // Check if this is the user's own profile
        setIsOwnProfile(authData.id === parseInt(userId));

        // 2. Fetch profile data
        const profileRes = await fetch(
          `http://localhost:8081/api/user/profile?id=${userId}`,
          {
            credentials: "include",
          },
        );

        if (!profileRes.ok) {
          if (profileRes.status === 404) {
            setError("User not found");
          } else if (profileRes.status === 403) {
            setError("This profile is private");
          } else {
            setError("Failed to load profile");
          }
          return;
        }

        const profileData = await profileRes.json();
        setProfile(profileData);

        // 3. Fetch user posts
        await fetchUserPosts(userId);

        // 4. Fetch followers and following
        await fetchFollowers(userId);
        await fetchFollowing(userId);
      } catch (err) {
        console.error("Error fetching profile:", err);
        setError("Network error");
      } finally {
        setLoading(false);
      }
    };

    if (userId) {
      fetchProfile();
    }
  }, [userId, router]);

  // ✅ Fetch user posts
  const fetchUserPosts = async (userId) => {
    setLoadingPosts(true);
    try {
      const response = await fetch(
        `http://localhost:8081/api/user/posts?id=${userId}`,
        { credentials: "include" },
      );
      if (response.ok) {
        const data = await response.json();
        setUserPosts(Array.isArray(data) ? data : []);
      } else {
        setUserPosts([]);
      }
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
      const response = await fetch(
        `http://localhost:8081/api/user/followers?id=${userId}`,
        { credentials: "include" },
      );
      if (response.ok) {
        const data = await response.json();
        setFollowers(Array.isArray(data) ? data : []);
      }
    } catch (error) {
      console.error("Error fetching followers:", error);
    }
  };

  // ✅ Fetch following
  const fetchFollowing = async (userId) => {
    try {
      const response = await fetch(
        `http://localhost:8081/api/user/following?id=${userId}`,
        { credentials: "include" },
      );
      if (response.ok) {
        const data = await response.json();
        setFollowing(Array.isArray(data) ? data : []);
      }
    } catch (error) {
      console.error("Error fetching following:", error);
    }
  };

  // ✅ Handle follow/unfollow
  const handleFollowToggle = async () => {
    if (!profile) return;

    try {
      const url = profile.is_following
        ? "http://localhost:8081/api/user/unfollow"
        : "http://localhost:8081/api/user/follow";

      const response = await fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ user_id: parseInt(userId) }),
      });

      const data = await response.json();

      if (response.ok) {
        // Update profile state
        setProfile({
          ...profile,
          is_following: !profile.is_following,
          followers_count: profile.is_following
            ? profile.followers_count - 1
            : profile.followers_count + 1,
        });

        // Update followers list
        if (!profile.is_following) {
          // Followed - add user to followers list if they have public profile
          setFollowers([...followers, { user: currentUser }]);
        } else {
          // Unfollowed - remove user from followers list
          setFollowers(followers.filter((f) => f.user.id !== currentUser.id));
        }

        toast.success(
          data.message || (profile.is_following ? "Unfollowed" : "Followed"),
        );
      } else {
        toast.error(data.error || "Failed to update follow status");
      }
    } catch (error) {
      console.error("Error toggling follow:", error);
      toast.error("Failed to update follow status");
    }
  };

  // ✅ Loading state
  if (loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <div className='animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600'></div>
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
            <div className='w-24 h-24 rounded-full border-4 border-white bg-blue-500 flex items-center justify-center text-white text-3xl font-bold shadow-lg'>
              {profile.first_name?.[0]}
              {profile.last_name?.[0]}
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
              </div>
            </div>

            {/* Follow / Edit Button */}
            <div className='mt-4 md:mt-0'>
              {isOwnProfile ? (
                <button
                  onClick={() => router.push("/settings")}
                  className='px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 transition'>
                  Edit Profile
                </button>
              ) : (
                <button
                  onClick={handleFollowToggle}
                  className={`px-4 py-2 rounded-lg font-medium transition ${
                    profile.is_following
                      ? "bg-gray-200 text-gray-700 hover:bg-gray-300"
                      : "bg-blue-500 text-white hover:bg-blue-600"
                  }`}>
                  {profile.is_following ? "Following" : "Follow"}
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
        </div>

        <div className='p-4'>
          {/* Posts Tab */}
          {activeTab === "posts" && (
            <div className='space-y-4'>
              {loadingPosts ? (
                <div className='flex justify-center items-center py-8'>
                  <div className='animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600'></div>
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
        </div>
      </div>
    </div>
  );
}
