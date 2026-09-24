"use client";

import { useState } from "react";
import InfoTooltip from "@/components/InfoTooltip";

const NOTIFICATION_TYPES = [
  {
    key: "email",
    label: "Email Notifications",
    info: "Get an email when something needs your attention, like a follow request or a new message.",
  },
  {
    key: "push",
    label: "Push Notifications",
    info: "Show real-time alerts in your browser while you're on the site.",
  },
  {
    key: "follow",
    label: "Follow Notifications",
    info: "Notify you when someone follows you or sends a follow request.",
  },
  {
    key: "messages",
    label: "Message Notifications",
    info: "Notify you when you receive a new direct message.",
  },
  {
    key: "groups",
    label: "Group Notifications",
    info: "Notify you about group invites, join requests, and activity in your groups.",
  },
];

// The "Notifications" tab of the Settings page.
export default function SettingsNotificationsTab({
  isLoading,
  setIsLoading,
  setMessage,
}) {
  const [notifications, setNotifications] = useState({
    email: true,
    push: true,
    follow: true,
    messages: true,
    groups: true,
  });

  const handleNotificationChange = (e) => {
    const { name, checked } = e.target;
    setNotifications({ ...notifications, [name]: checked });
  };

  const handleNotificationSubmit = (e) => {
    e.preventDefault();
    setIsLoading(true);
    setMessage("");

    // Simulate API call
    setTimeout(() => {
      setMessage("Notification settings updated!");
      setIsLoading(false);
    }, 1000);
  };

  return (
    <form onSubmit={handleNotificationSubmit} className='space-y-4'>
      <h2 className='text-lg font-semibold'>Notification Preferences</h2>
      <p className='text-sm text-gray-500'>
        Choose what notifications you want to receive
      </p>

      <div className='space-y-3'>
        {NOTIFICATION_TYPES.map(({ key, label, info }) => (
          <div key={key} className='flex items-center justify-between'>
            <span className='settings-notif-label'>
              <label htmlFor={`settings-notif-${key}`} className='text-sm text-gray-700'>
                {label}
              </label>
              <InfoTooltip text={info} />
            </span>
            <input
              id={`settings-notif-${key}`}
              type='checkbox'
              name={key}
              checked={notifications[key]}
              onChange={handleNotificationChange}
              className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
            />
          </div>
        ))}
      </div>

      <button
        type='submit'
        disabled={isLoading}
        className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
        {isLoading ? "Saving..." : "Save Settings"}
      </button>
    </form>
  );
}
