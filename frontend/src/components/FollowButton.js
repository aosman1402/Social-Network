"use client";

import { useState, useEffect } from "react";
import toast from "react-hot-toast";

export default function FollowButton({
  userId,
  isFollowing: initialIsFollowing,
  onFollowChange,
}) {
  const [isFollowing, setIsFollowing] = useState(initialIsFollowing || false);
  const [isPending, setIsPending] = useState(false);
  const [loading, setLoading] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

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
      setShowConfirm(false);
    }
  };

  const handleUnfollowClick = () => {
    setShowConfirm(true);
  };

  const handleCancelUnfollow = () => {
    setShowConfirm(false);
  };

  if (isPending) {
    return (
      <button
        disabled
        className='px-4 py-2 bg-yellow-500 text-white rounded-lg cursor-not-allowed opacity-70'>
        <span className='flex items-center space-x-2'>
          <span>⏳</span>
          <span>Pending...</span>
        </span>
      </button>
    );
  }

  if (showConfirm) {
    return (
      <div className='flex items-center space-x-2'>
        <span className='text-sm text-gray-600'>Unfollow?</span>
        <button
          onClick={handleUnfollow}
          disabled={loading}
          className='px-3 py-1 bg-red-500 text-white rounded hover:bg-red-600 text-sm'>
          Yes
        </button>
        <button
          onClick={handleCancelUnfollow}
          className='px-3 py-1 bg-gray-300 text-gray-700 rounded hover:bg-gray-400 text-sm'>
          No
        </button>
      </div>
    );
  }

  return (
    <button
      onClick={isFollowing ? handleUnfollowClick : handleFollow}
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
