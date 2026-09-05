"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { toast } from "@/context/ToastContext";

export default function FollowRequests() {
  const router = useRouter();
  const [requests, setRequests] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchRequests();
  }, []);

  const fetchRequests = async () => {
    try {
      const response = await fetch(
        "http://localhost:8081/api/user/follow-requests",
        {
          credentials: "include",
        },
      );
      if (response.ok) {
        const data = await response.json();
        setRequests(data);
      }
    } catch (error) {
      console.error("Error fetching follow requests:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleRequest = async (followerId, action) => {
    try {
      const response = await fetch(
        "http://localhost:8081/api/user/follow-request",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({ follower_id: followerId, action }),
        },
      );

      if (response.ok) {
        setRequests(requests.filter((r) => r.follower.id !== followerId));
        toast.success(`Follow request ${action}ed`);
        // Refresh the page to update follower count
        window.location.reload();
      } else {
        const data = await response.json();
        toast.error(data.error || "Failed to process request");
      }
    } catch (error) {
      console.error("Error processing follow request:", error);
      toast.error("Failed to process request");
    }
  };

  if (loading) {
    return (
      <div className='text-center py-4'>
        <div className='animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mx-auto'></div>
      </div>
    );
  }

  if (requests.length === 0) {
    return (
      <div className='bg-white rounded-lg shadow p-4 text-center text-gray-500'>
        <p>No pending follow requests</p>
      </div>
    );
  }

  return (
    <div className='bg-white rounded-lg shadow divide-y'>
      <div className='p-4 border-b'>
        <h3 className='font-semibold'>Follow Requests</h3>
      </div>
      {requests.map((request) => (
        <div
          key={request.id}
          className='p-4 flex items-center justify-between hover:bg-gray-50'>
          <div
            className='flex items-center space-x-3 cursor-pointer'
            onClick={() => router.push(`/profile/${request.follower.id}`)}>
            <div className='w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold'>
              {request.follower.first_name?.[0]}
              {request.follower.last_name?.[0]}
            </div>
            <div>
              <p className='font-medium'>
                {request.follower.first_name} {request.follower.last_name}
              </p>
              {request.follower.nickname && (
                <p className='text-sm text-gray-500'>
                  @{request.follower.nickname}
                </p>
              )}
            </div>
          </div>
          <div className='flex space-x-2'>
            <button
              onClick={() => handleRequest(request.follower.id, "accept")}
              className='p-2 bg-green-500 text-white rounded-full hover:bg-green-600 transition'
              title='Accept'>
              ✓
            </button>
            <button
              onClick={() => handleRequest(request.follower.id, "decline")}
              className='p-2 bg-red-500 text-white rounded-full hover:bg-red-600 transition'
              title='Decline'>
              ✕
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
