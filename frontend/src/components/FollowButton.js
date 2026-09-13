"use client";

import { useState, useEffect } from "react";
import { toast } from "@/context/ToastContext";

export default function FollowButton({
  userId,
  isFollowing: initialIsFollowing,
  onFollowChange,
}) {
  const [isFollowing, setIsFollowing] = useState(initialIsFollowing || false);
  const [isPending, setIsPending] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setIsFollowing(initialIsFollowing || false);
  }, [initialIsFollowing]);

  const handleFollow = async () => {
    if (loading) return;
    setLoading(true);

    try {
      const response = await fetch("http://localhost:8081/api/user/follow", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ user_id: userId }),
      });

      const data = await response.json();

      if (response.ok) {
        if (data.status === "pending") {
          setIsPending(true);
          toast.success("Follow request sent!");
        } else {
          setIsFollowing(true);
          setIsPending(false);
          toast.success("Now following this user!");
        }
        if (onFollowChange) onFollowChange(true);
      } else {
        toast.error(data.error || "Failed to follow user");
      }
    } catch (error) {
      console.error("Error following user:", error);
      toast.error("Failed to follow user");
    } finally {
      setLoading(false);
    }
  };

  const handleUnfollow = async () => {
    if (loading) return;
    setLoading(true);

    try {
      const response = await fetch("http://localhost:8081/api/user/unfollow", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ user_id: userId }),
      });

      if (response.ok) {
        setIsFollowing(false);
        setIsPending(false);
        toast.success("Unfollowed successfully!");
        if (onFollowChange) onFollowChange(false);
      } else {
        const data = await response.json();
        toast.error(data.error || "Failed to unfollow user");
      }
    } catch (error) {
      console.error("Error unfollowing user:", error);
      toast.error("Failed to unfollow user");
    } finally {
      setLoading(false);
    }
  };

  if (isPending) {
    return (
      <button
        onClick={handleUnfollow}
        disabled={loading}
        title='Cancel follow request'
        className='px-4 py-2 bg-yellow-100 text-yellow-800 rounded-lg hover:bg-red-100 hover:text-red-700 transition'>
        <span className='flex items-center space-x-2'>
          <span>⏳</span>
          <span>{loading ? "Cancelling..." : "Pending · Cancel"}</span>
        </span>
      </button>
    );
  }

  return (
    <button
      onClick={isFollowing ? handleUnfollow : handleFollow}
      disabled={loading}
      className={`px-4 py-2 rounded-lg font-medium transition flex items-center space-x-2 ${
        isFollowing
          ? "bg-gray-200 text-gray-700 hover:bg-gray-300"
          : "bg-blue-500 text-white hover:bg-blue-600"
      } ${loading ? "opacity-50 cursor-not-allowed" : ""}`}>
      {loading ? (
        <span>Loading...</span>
      ) : isFollowing ? (
        <>
          <span>✓</span>
          <span>Following</span>
        </>
      ) : (
        <>
          <span>+</span>
          <span>Follow</span>
        </>
      )}
    </button>
  );
}
