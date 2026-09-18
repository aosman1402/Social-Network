"use client";

import { useState } from "react";

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
        <div className='flex items-center justify-between'>
          <label htmlFor='settings-notif-email' className='text-sm text-gray-700'>
            Email Notifications
          </label>
          <input
            id='settings-notif-email'
            type='checkbox'
            name='email'
            checked={notifications.email}
            onChange={handleNotificationChange}
            className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
          />
        </div>
        <div className='flex items-center justify-between'>
          <label htmlFor='settings-notif-push' className='text-sm text-gray-700'>
            Push Notifications
          </label>
          <input
            id='settings-notif-push'
            type='checkbox'
            name='push'
            checked={notifications.push}
            onChange={handleNotificationChange}
            className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
          />
        </div>
        <div className='flex items-center justify-between'>
          <label htmlFor='settings-notif-follow' className='text-sm text-gray-700'>
            Follow Notifications
          </label>
          <input
            id='settings-notif-follow'
            type='checkbox'
            name='follow'
            checked={notifications.follow}
            onChange={handleNotificationChange}
            className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
          />
        </div>
        <div className='flex items-center justify-between'>
          <label htmlFor='settings-notif-messages' className='text-sm text-gray-700'>
            Message Notifications
          </label>
          <input
            id='settings-notif-messages'
            type='checkbox'
            name='messages'
            checked={notifications.messages}
            onChange={handleNotificationChange}
            className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
          />
        </div>
        <div className='flex items-center justify-between'>
          <label htmlFor='settings-notif-groups' className='text-sm text-gray-700'>
            Group Notifications
          </label>
          <input
            id='settings-notif-groups'
            type='checkbox'
            name='groups'
            checked={notifications.groups}
            onChange={handleNotificationChange}
            className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
          />
        </div>
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
