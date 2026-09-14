"use client";

import { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { toast } from "@/context/ToastContext";
import { useAuth } from "@/context/AuthContext";

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
  const [unreadCount, setUnreadCount] = useState(0);
  const [showDropdown, setShowDropdown] = useState(false);
  const [notifications, setNotifications] = useState([]);
  const [loading, setLoading] = useState(false);
  const dropdownRef = useRef(null);
  const wsRef = useRef(null);
  const reconnectTimerRef = useRef(null);
  const reconnectDelayRef = useRef(1000);
  const unmountingRef = useRef(false);

  useEffect(() => {
    fetchUnreadCount();
    fetchNotifications();
    connectWebSocket();

    // Click outside to close dropdown
    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      unmountingRef.current = true;
      clearTimeout(reconnectTimerRef.current);
      const ws = wsRef.current;
      if (ws) {
        if (ws.readyState === WebSocket.CONNECTING) {
          // Closing mid-handshake logs a "closed before connection
          // established" warning; wait for it to open, then close it.
          ws.addEventListener("open", () => ws.close(), { once: true });
        } else {
          ws.close();
        }
      }
    };
  }, []);

  const connectWebSocket = () => {
    const ws = new WebSocket("ws://localhost:8081/ws");
    wsRef.current = ws;

    ws.onopen = () => {
      reconnectDelayRef.current = 1000; // back to the short delay once healthy
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.type === "notification") {
          const notification = JSON.parse(data.data);
          setNotifications((prev) => [notification, ...prev]);
          setUnreadCount((prev) => prev + 1);
          toast.success(notification.content);
        }
      } catch (error) {
        console.error("Error parsing notification:", error);
      }
    };

    ws.onclose = () => {
      if (unmountingRef.current) return;
      // Reconnect with exponential backoff (capped at 30s) instead of
      // leaving notifications silently offline until the next page visit.
      const maxReconnectDelay = 30000;
      reconnectTimerRef.current = setTimeout(() => {
        reconnectDelayRef.current = Math.min(
          reconnectDelayRef.current * 2,
          maxReconnectDelay,
        );
        connectWebSocket();
      }, reconnectDelayRef.current);
    };

    ws.onerror = (error) => {
      console.error("Notification WebSocket error:", error);
    };
  };

  const fetchUnreadCount = async () => {
    try {
      const response = await fetch(
        "http://localhost:8081/api/notifications/unread",
        {
          credentials: "include",
        },
      );
      if (response.ok) {
        const data = await response.json();
        setUnreadCount(data.unread_count || 0);
      }
    } catch (error) {
      console.error("Error fetching unread count:", error);
    }
  };

  const fetchNotifications = async () => {
    setLoading(true);
    try {
      const response = await fetch("http://localhost:8081/api/notifications", {
        credentials: "include",
      });
      if (response.ok) {
        const data = await response.json();
        setNotifications(data || []);
      }
    } catch (error) {
      console.error("Error fetching notifications:", error);
    } finally {
      setLoading(false);
    }
  };

  const markAsRead = async (id) => {
    try {
      const response = await fetch(
        "http://localhost:8081/api/notifications/read",
        {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({ notification_id: id }),
        },
      );
      if (response.ok) {
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
      const response = await fetch(
        "http://localhost:8081/api/notifications/read-all",
        {
          method: "PUT",
          credentials: "include",
        },
      );
      if (response.ok) {
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

    // Navigate based on notification type
    switch (notification.type) {
      case "follow_request":
        // related_id here is the sender (the person who wants to follow
        // you) - the recipient needs to land on their OWN profile's
        // Requests tab to accept/decline, not on the sender's profile.
        router.push(`/profile/${user?.id}?tab=requests`);
        break;
      case "follow_accepted":
      case "new_follower":
        router.push(`/profile/${notification.related_id}`);
        break;
      case "group_invitation":
      case "group_request_accepted":
        router.push(`/groups/${notification.related_id}`);
        break;
      case "event_created":
        router.push(`/groups/${notification.related_id}`);
        break;
      case "comment":
        router.push(`/`);
        break;
      default:
        router.push("/notifications");
    }
    setShowDropdown(false);
  };

  const getNotificationIcon = (type) => {
    switch (type) {
      case "follow_request":
      case "follow_accepted":
      case "new_follower":
        return "👤";
      case "group_invitation":
        return "👥";
      case "group_request":
        return "📩";
      case "group_request_accepted":
        return "✅";
      case "event_created":
        return "📅";
      case "comment":
        return "💬";
      default:
        return "🔔";
    }
  };

  return (
    <div className='relative' ref={dropdownRef}>
      <button
        onClick={() => setShowDropdown(!showDropdown)}
        className='relative p-2 text-gray-600 hover:text-blue-600 transition'>
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
                        className={`text-sm ${!notification.is_read ? "font-semibold" : ""}`}>
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
