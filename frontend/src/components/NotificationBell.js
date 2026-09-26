"use client";

import { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { toast } from "@/context/ToastContext";
import { useAuth } from "@/context/AuthContext";
import { useWebSocket } from "@/context/WebSocketContext";
import { apiFetch } from "@/lib/api";
import { getNotificationIcon, getNotificationTarget } from "@/utils/helpers";

function BellIcon({ size = 20 }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox='0 0 24 24'
      fill='currentColor'
      aria-hidden='true'>
      <path d='M12 22c1.1 0 2-.9 2-2h-4c0 1.1.89 2 2 2zm6-6v-5c0-3.07-1.64-5.64-4.5-6.32V4c0-.83-.67-1.5-1.5-1.5s-1.5.67-1.5 1.5v.68C7.63 5.36 6 7.92 6 11v5l-2 2v1h16v-1l-2-2z' />
    </svg>
  );
}

export default function NotificationBell() {
  const router = useRouter();
  const { user } = useAuth();
  const { subscribe, isConnected } = useWebSocket();
  const [unreadCount, setUnreadCount] = useState(0);
  const [showDropdown, setShowDropdown] = useState(false);
  const [notifications, setNotifications] = useState([]);
  const [loading, setLoading] = useState(false);
  const dropdownRef = useRef(null);
  // Tracks which notification IDs we've already counted, so a duplicate
  // live push (the GET-on-mount fetch can already include a row that then
  // also arrives over the socket) doesn't inflate unreadCount - it's a
  // separately-tracked counter, not derived from `notifications`, so
  // deduping the list alone wouldn't keep it in sync.
  const seenNotificationIdsRef = useRef(new Set());

  useEffect(() => {
    fetchUnreadCount();
    fetchNotifications();

    // Click outside to close dropdown
    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  useEffect(() => {
    return subscribe("notification", (notification) => {
      if (seenNotificationIdsRef.current.has(notification.id)) return;
      seenNotificationIdsRef.current.add(notification.id);

      setNotifications((prev) => [notification, ...prev]);
      setUnreadCount((prev) => prev + 1);
      toast.success(notification.content);
    });
  }, [subscribe]);

  // Notifications were marked read somewhere else - the notifications
  // page, another tab, or the server itself (e.g. answering a group
  // invitation). Mirror it in the list and resync the count from the
  // server rather than adjusting it locally, so it can't drift.
  useEffect(() => {
    return subscribe("notifications_read", ({ notification_ids, all }) => {
      const ids = new Set(notification_ids || []);
      setNotifications((prev) =>
        prev.map((n) => (all || ids.has(n.id) ? { ...n, is_read: true } : n)),
      );
      fetchUnreadCount();
    });
  }, [subscribe]);

  // Anything sent while the socket was down never arrived live - reload
  // on every reconnect (not the first connect, which the mount fetch
  // above already covers).
  const wasConnectedRef = useRef(false);
  useEffect(() => {
    if (!isConnected) return;
    if (wasConnectedRef.current) {
      fetchUnreadCount();
      fetchNotifications();
    }
    wasConnectedRef.current = true;
  }, [isConnected]);

  // A cancelled follow request stays in the list (there's still a record
  // it happened) but is no longer actionable - flag it so it renders
  // struck through instead of being removed.
  useEffect(() => {
    return subscribe("follow_request_cancelled", ({ notification_id }) => {
      setNotifications((prev) =>
        prev.map((n) =>
          n.id === notification_id ? { ...n, is_cancelled: true } : n,
        ),
      );
    });
  }, [subscribe]);

  const fetchUnreadCount = async () => {
    try {
      const { ok, data } = await apiFetch("/notifications/unread");
      if (ok) {
        setUnreadCount(data.unread_count || 0);
      }
    } catch (error) {
      console.error("Error fetching unread count:", error);
    }
  };

  const fetchNotifications = async () => {
    setLoading(true);
    try {
      const { ok, data } = await apiFetch("/notifications");
      if (ok) {
        const list = data || [];
        list.forEach((n) => seenNotificationIdsRef.current.add(n.id));
        setNotifications(list);
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
        setUnreadCount((prev) => Math.max(0, prev - 1));
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
        setUnreadCount(0);
        toast.success("All notifications marked as read");
      }
    } catch (error) {
      console.error("Error marking all as read:", error);
      toast.error("Failed to mark all as read");
    }
  };

  const handleNotificationClick = (notification) => {
    if (!notification.is_read) {
      markAsRead(notification.id);
    }

    router.push(getNotificationTarget(notification, user?.id));
    setShowDropdown(false);
  };

  return (
    <div className='relative' ref={dropdownRef}>
      <button
        onClick={() => setShowDropdown(!showDropdown)}
        aria-label={unreadCount > 0 ? `Notifications (${unreadCount} unread)` : "Notifications"}
        className='navbar-bell relative p-2 text-gray-600 hover:text-blue-600 transition'>
        <BellIcon size={20} />
        {unreadCount > 0 && (
          <span className='absolute top-0 right-0 bg-red-500 text-white text-xs font-bold rounded-full w-5 h-5 flex items-center justify-center'>
            {unreadCount > 9 ? "9+" : unreadCount}
          </span>
        )}
      </button>

      {showDropdown && (
        <div className='absolute right-0 mt-2 w-96 bg-white rounded-lg shadow-xl border max-h-[500px] flex flex-col z-50'>
          <div className='p-3 border-b flex items-center justify-between'>
            <h3 className='font-semibold'>Notifications</h3>
            {unreadCount > 0 && (
              <button
                onClick={markAllAsRead}
                className='text-xs text-blue-500 hover:text-blue-700'>
                Mark all as read
              </button>
            )}
            <button
              onClick={() => {
                setShowDropdown(false);
                router.push("/notifications");
              }}
              className='text-xs text-gray-400 hover:text-gray-600'>
              View all
            </button>
          </div>

          <div className='overflow-y-auto flex-1'>
            {loading ? (
              <div className='p-4 text-center text-gray-400'>Loading...</div>
            ) : notifications.length === 0 ? (
              <div className='p-8 text-center text-gray-400'>
                <p className='text-2xl mb-2'>🔔</p>
                <p>No notifications yet</p>
              </div>
            ) : (
              notifications.slice(0, 10).map((notification) => (
                <div
                  key={notification.id}
                  onClick={() => handleNotificationClick(notification)}
                  className={`p-3 hover:bg-gray-50 cursor-pointer transition border-b ${
                    !notification.is_read ? "bg-blue-50" : ""
                  }`}>
                  <div className='flex items-start space-x-3'>
                    <div className='text-2xl'>
                      {getNotificationIcon(notification.type)}
                    </div>
                    <div className='flex-1 min-w-0'>
                      <p
                        className={`text-sm ${!notification.is_read ? "font-semibold" : ""} ${
                          notification.is_cancelled
                            ? "line-through text-gray-400"
                            : ""
                        }`}>
                        {notification.content}
                      </p>
                      <p className='text-xs text-gray-400 mt-1'>
                        {new Date(notification.created_at).toLocaleDateString()}{" "}
                        at{" "}
                        {new Date(notification.created_at).toLocaleTimeString(
                          [],
                          {
                            hour: "2-digit",
                            minute: "2-digit",
                          },
                        )}
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
      )}
    </div>
  );
}
