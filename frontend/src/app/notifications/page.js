"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { formatDistanceToNow } from "@/lib/formatDistanceToNow";
import { toast } from "@/context/ToastContext";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";
import { apiFetch } from "@/lib/api";
import LoadingSpinner from "@/components/LoadingSpinner";
import { getNotificationIcon, getNotificationTarget } from "@/utils/helpers";

export default function NotificationsPage() {
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const { subscribe } = useWebSocket();
  const [notifications, setNotifications] = useState([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (authLoading) return;
    if (!user) {
      router.push("/login");
      return;
    }
    fetchNotifications();
  }, [user, authLoading, router]);

  useEffect(() => {
    return subscribe("notification", (notification) => {
      setNotifications((prev) => [notification, ...prev]);
    });
  }, [subscribe]);

  const fetchNotifications = async () => {
    try {
      const { ok, data } = await apiFetch("/notifications");
      if (ok) {
        setNotifications(data);
      }
    } catch (error) {
      console.error("Error fetching notifications:", error);
    } finally {
      setLoading(false);
    }
  };

  const markAsRead = async (id) => {
    try {
      const { ok } = await apiFetch("/notifications/read", {
        method: "PUT",
        json: { notification_id: id },
      });
      if (ok) {
        setNotifications((prev) =>
          prev.map((n) => (n.id === id ? { ...n, is_read: true } : n)),
        );
      }
    } catch (error) {
      console.error("Error marking notification as read:", error);
    }
  };

  const markAllAsRead = async () => {
    try {
      const { ok } = await apiFetch("/notifications/read-all", {
        method: "PUT",
      });
      if (ok) {
        setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
        toast.success("All notifications marked as read");
      }
    } catch (error) {
      console.error("Error marking all as read:", error);
      toast.error("Failed to mark all as read");
    }
  };

  const unreadCount = notifications.filter((n) => !n.is_read).length;

  if (loading) {
    return (
      <div className='flex justify-center items-center h-64'>
        <LoadingSpinner size='large' />
      </div>
    );
  }

  return (
    <div className='max-w-3xl mx-auto py-4 px-4'>
      <div className='bg-white rounded-lg shadow'>
        <div className='p-4 border-b flex items-center justify-between'>
          <div className='flex items-center space-x-3'>
            <h1 className='text-2xl font-bold'>Notifications</h1>
            {unreadCount > 0 && (
              <span className='bg-blue-500 text-white text-xs font-bold px-2 py-1 rounded-full'>
                {unreadCount} new
              </span>
            )}
          </div>
          {unreadCount > 0 && (
            <button
              onClick={markAllAsRead}
              className='text-sm text-blue-500 hover:text-blue-700'>
              Mark all as read
            </button>
          )}
        </div>

        <div className='divide-y'>
          {notifications.length === 0 ? (
            <div className='p-8 text-center text-gray-500'>
              <p className='text-4xl mb-2'>🔔</p>
              <p>No notifications yet</p>
            </div>
          ) : (
            notifications.map((notification) => (
              <div
                key={notification.id}
                className={`p-4 hover:bg-gray-50 cursor-pointer transition ${
                  !notification.is_read ? "bg-blue-50" : ""
                }`}
                onClick={() => {
                  if (!notification.is_read) {
                    markAsRead(notification.id);
                  }
                  router.push(getNotificationTarget(notification, user?.id));
                }}>
                <div className='flex items-start space-x-3'>
                  <div className='text-2xl'>
                    {getNotificationIcon(notification.type)}
                  </div>
                  <div className='flex-1'>
                    <p className={!notification.is_read ? "font-semibold" : ""}>
                      {notification.content}
                    </p>
                    <p className='text-sm text-gray-400 mt-1'>
                      {formatDistanceToNow(new Date(notification.created_at), {
                        addSuffix: true,
                      })}
                    </p>
                  </div>
                  {!notification.is_read && (
                    <div className='w-2 h-2 mt-2 bg-blue-500 rounded-full flex-shrink-0'></div>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
