"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { toast } from "@/context/ToastContext";
import { useWebSocket } from "@/context/WebSocketContext";
import { apiFetch } from "@/lib/api";
import LoadingSpinner from "@/components/LoadingSpinner";

export default function FollowRequests({ onRequestHandled, onCountChange }) {
  const router = useRouter();
  const { subscribe } = useWebSocket();
  const [requests, setRequests] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchRequests();
  }, []);

  // Refetch on a live "follow_request" notification instead of relying on
  // navigation to re-trigger a fetch - clicking the notification when
  // you're already sitting on this tab doesn't produce a navigation event
  // at all (same URL), so the list would otherwise never learn about the
  // new pending request without a manual page reload.
  useEffect(() => {
    return subscribe("notification", (notification) => {
      if (notification.type === "follow_request") {
        fetchRequests();
      }
    });
  }, [subscribe]);

  // A sender cancelling their still-pending request no longer belongs in
  // this list - refetch so it disappears (and the "Requests (N)" count
  // drops via onCountChange below) without needing a page reload.
  useEffect(() => {
    return subscribe("follow_request_cancelled", () => {
      fetchRequests();
    });
  }, [subscribe]);

  // Report the current pending count up to the parent whenever it changes
  // (initial load, and after each accept/decline) so a tab label showing
  // "Requests (N)" elsewhere stays in sync without a page reload.
  useEffect(() => {
    if (onCountChange) onCountChange(requests.length);
  }, [requests, onCountChange]);

  const fetchRequests = async () => {
    try {
      const { ok, data } = await apiFetch("/user/follow-requests");
      if (ok) {
        setRequests(Array.isArray(data) ? data : []);
      }
    } catch (error) {
      console.error("Error fetching follow requests:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleRequest = async (followerId, action) => {
    try {
      const { ok, data } = await apiFetch("/user/follow-request", {
        method: "POST",
        json: { follower_id: followerId, action },
      });

      if (ok) {
        const request = requests.find((r) => r.follower.id === followerId);
        setRequests(requests.filter((r) => r.follower.id !== followerId));
        toast.success(`Follow request ${action}ed`);
        // Let the parent update follower count/list in place instead of a
        // full page reload.
        if (onRequestHandled) onRequestHandled(action, request?.follower);
      } else {
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
        <LoadingSpinner size='medium' />
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
