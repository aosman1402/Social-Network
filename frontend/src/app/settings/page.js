"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import { toast } from "@/context/ToastContext";

export default function SettingsPage() {
  const router = useRouter();
  const { user, setUser } = useAuth();
  const [activeTab, setActiveTab] = useState("profile");
  const [isLoading, setIsLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [uploadingAvatar, setUploadingAvatar] = useState(false);

  // Profile settings
  const [profile, setProfile] = useState({
    firstName: "John",
    lastName: "Doe",
    email: "john@example.com",
    bio: "Software developer and tech enthusiast",
    location: "San Francisco, CA",
    isPublic: true,
  });

  // Password settings
  const [password, setPassword] = useState({
    current: "",
    new: "",
    confirm: "",
  });

  // Notification settings
  const [notifications, setNotifications] = useState({
    email: true,
    push: true,
    follow: true,
    messages: true,
    groups: true,
  });

  const handleAvatarChange = async (e) => {
    const file = e.target.files[0];
    if (!file) return;

    const validTypes = ["image/jpeg", "image/png", "image/gif"];
    if (!validTypes.includes(file.type)) {
      toast.error("Please upload a valid image (JPEG, PNG, or GIF)");
      e.target.value = "";
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      toast.error("Image size must be less than 5MB");
      e.target.value = "";
      return;
    }

    setUploadingAvatar(true);
    try {
      const formData = new FormData();
      formData.append("avatar", file);

      const response = await fetch("http://localhost:8081/api/user/avatar", {
        method: "POST",
        credentials: "include",
        body: formData,
      });

      const data = await response.json();

      if (!response.ok) {
        toast.error(data.error || "Failed to update avatar");
        return;
      }

      setUser((prev) => (prev ? { ...prev, avatar_path: data.avatar_path } : prev));
      toast.success("Profile picture updated!");
    } catch (error) {
      console.error("Error uploading avatar:", error);
      toast.error("Failed to update avatar");
    } finally {
      setUploadingAvatar(false);
      e.target.value = "";
    }
  };

  const handleProfileChange = (e) => {
    const { name, value, type, checked } = e.target;
    setProfile({
      ...profile,
      [name]: type === "checkbox" ? checked : value,
    });
  };

  const handlePasswordChange = (e) => {
    const { name, value } = e.target;
    setPassword({ ...password, [name]: value });
  };

  const handleNotificationChange = (e) => {
    const { name, checked } = e.target;
    setNotifications({ ...notifications, [name]: checked });
  };

  const handleProfileSubmit = (e) => {
    e.preventDefault();
    setIsLoading(true);
    setMessage("");

    // Simulate API call
    setTimeout(() => {
      setMessage("Profile updated successfully!");
      setIsLoading(false);
    }, 1000);
  };

  const handlePasswordSubmit = async (e) => {
    e.preventDefault();
    if (password.new !== password.confirm) {
      setMessage("Passwords do not match");
      return;
    }
    if (password.new.length < 8) {
      setMessage("New password must be at least 8 characters");
      return;
    }

    setIsLoading(true);
    setMessage("");

    try {
      const response = await fetch("http://localhost:8081/api/user/password", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          current_password: password.current,
          new_password: password.new,
        }),
      });

      const data = await response.json();

      if (!response.ok) {
        setMessage(data.error || "Failed to change password");
        return;
      }

      setPassword({ current: "", new: "", confirm: "" });
      // Changing the password invalidates every session, including this
      // one - the user has to log in again with the new password.
      router.push("/login");
    } catch (error) {
      console.error("Error changing password:", error);
      setMessage("Failed to change password");
    } finally {
      setIsLoading(false);
    }
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
    <div className='max-w-4xl mx-auto py-4 px-4'>
      <div className='bg-white rounded-lg shadow'>
        {/* Header */}
        <div className='p-6 border-b'>
          <h1 className='text-2xl font-bold text-gray-800'>Settings</h1>
          <p className='text-gray-500 text-sm'>
            Manage your account preferences
          </p>
        </div>

        <div className='flex flex-col md:flex-row'>
          {/* Sidebar */}
          <div className='w-full md:w-48 border-r p-4 space-y-2'>
            <button
              onClick={() => setActiveTab("profile")}
              className={`w-full text-left px-3 py-2 rounded-lg transition ${
                activeTab === "profile"
                  ? "bg-blue-50 text-blue-600"
                  : "hover:bg-gray-50 text-gray-700"
              }`}>
              Profile
            </button>
            <button
              onClick={() => setActiveTab("password")}
              className={`w-full text-left px-3 py-2 rounded-lg transition ${
                activeTab === "password"
                  ? "bg-blue-50 text-blue-600"
                  : "hover:bg-gray-50 text-gray-700"
              }`}>
              Password
            </button>
            <button
              onClick={() => setActiveTab("notifications")}
              className={`w-full text-left px-3 py-2 rounded-lg transition ${
                activeTab === "notifications"
                  ? "bg-blue-50 text-blue-600"
                  : "hover:bg-gray-50 text-gray-700"
              }`}>
              Notifications
            </button>
          </div>

          {/* Content */}
          <div className='flex-1 p-6'>
            {message && (
              <div
                className={`p-3 rounded-lg mb-4 ${
                  message.includes("successfully")
                    ? "bg-green-50 text-green-700"
                    : "bg-red-50 text-red-700"
                }`}>
                {message}
              </div>
            )}

            {/* Profile Tab */}
            {activeTab === "profile" && (
              <form onSubmit={handleProfileSubmit} className='space-y-4'>
                <h2 className='text-lg font-semibold'>Profile Information</h2>

                <div className='flex items-center gap-4'>
                  {user?.avatar_path ? (
                    <img
                      src={`http://localhost:8081${user.avatar_path}`}
                      alt='Profile picture'
                      className='w-16 h-16 rounded-full object-cover border'
                    />
                  ) : (
                    <div className='w-16 h-16 rounded-full bg-blue-500 flex items-center justify-center text-white text-xl font-bold'>
                      {user?.first_name?.[0] || "U"}
                    </div>
                  )}
                  <label className='cursor-pointer px-3 py-2 border border-gray-300 rounded-lg text-sm text-gray-700 hover:bg-gray-50 transition'>
                    {uploadingAvatar ? "Uploading..." : "Change Photo"}
                    <input
                      type='file'
                      accept='image/jpeg,image/png,image/gif'
                      onChange={handleAvatarChange}
                      className='hidden'
                      disabled={uploadingAvatar}
                    />
                  </label>
                </div>

                <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                  <div>
                    <label className='block text-sm font-medium text-gray-700 mb-1'>
                      First Name
                    </label>
                    <input
                      type='text'
                      name='firstName'
                      value={profile.firstName}
                      onChange={handleProfileChange}
                      className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                    />
                  </div>
                  <div>
                    <label className='block text-sm font-medium text-gray-700 mb-1'>
                      Last Name
                    </label>
                    <input
                      type='text'
                      name='lastName'
                      value={profile.lastName}
                      onChange={handleProfileChange}
                      className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                    />
                  </div>
                </div>

                <div>
                  <label className='block text-sm font-medium text-gray-700 mb-1'>
                    Email
                  </label>
                  <input
                    type='email'
                    name='email'
                    value={profile.email}
                    onChange={handleProfileChange}
                    className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                  />
                </div>

                <div>
                  <label className='block text-sm font-medium text-gray-700 mb-1'>
                    Bio
                  </label>
                  <textarea
                    name='bio'
                    value={profile.bio}
                    onChange={handleProfileChange}
                    rows='3'
                    className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none'
                  />
                </div>

                <div>
                  <label className='block text-sm font-medium text-gray-700 mb-1'>
                    Location
                  </label>
                  <input
                    type='text'
                    name='location'
                    value={profile.location}
                    onChange={handleProfileChange}
                    className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                  />
                </div>

                <div className='flex items-center space-x-2'>
                  <input
                    type='checkbox'
                    name='isPublic'
                    checked={profile.isPublic}
                    onChange={handleProfileChange}
                    className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                  />
                  <label className='text-sm text-gray-700'>
                    Make my profile public
                  </label>
                </div>

                <button
                  type='submit'
                  disabled={isLoading}
                  className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
                  {isLoading ? "Saving..." : "Save Profile"}
                </button>
              </form>
            )}

            {/* Password Tab */}
            {activeTab === "password" && (
              <form onSubmit={handlePasswordSubmit} className='space-y-4'>
                <h2 className='text-lg font-semibold'>Change Password</h2>
                <p className='text-sm text-gray-500'>
                  Password must be at least 8 characters long
                </p>

                <div>
                  <label className='block text-sm font-medium text-gray-700 mb-1'>
                    Current Password
                  </label>
                  <input
                    type='password'
                    name='current'
                    value={password.current}
                    onChange={handlePasswordChange}
                    className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                    required
                  />
                </div>

                <div>
                  <label className='block text-sm font-medium text-gray-700 mb-1'>
                    New Password
                  </label>
                  <input
                    type='password'
                    name='new'
                    value={password.new}
                    onChange={handlePasswordChange}
                    className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                    required
                  />
                </div>

                <div>
                  <label className='block text-sm font-medium text-gray-700 mb-1'>
                    Confirm New Password
                  </label>
                  <input
                    type='password'
                    name='confirm'
                    value={password.confirm}
                    onChange={handlePasswordChange}
                    className='w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500'
                    required
                  />
                </div>

                <button
                  type='submit'
                  disabled={isLoading}
                  className='px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition disabled:opacity-50'>
                  {isLoading ? "Changing..." : "Change Password"}
                </button>
              </form>
            )}

            {/* Notifications Tab */}
            {activeTab === "notifications" && (
              <form onSubmit={handleNotificationSubmit} className='space-y-4'>
                <h2 className='text-lg font-semibold'>
                  Notification Preferences
                </h2>
                <p className='text-sm text-gray-500'>
                  Choose what notifications you want to receive
                </p>

                <div className='space-y-3'>
                  <div className='flex items-center justify-between'>
                    <label className='text-sm text-gray-700'>
                      Email Notifications
                    </label>
                    <input
                      type='checkbox'
                      name='email'
                      checked={notifications.email}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label className='text-sm text-gray-700'>
                      Push Notifications
                    </label>
                    <input
                      type='checkbox'
                      name='push'
                      checked={notifications.push}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label className='text-sm text-gray-700'>
                      Follow Notifications
                    </label>
                    <input
                      type='checkbox'
                      name='follow'
                      checked={notifications.follow}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label className='text-sm text-gray-700'>
                      Message Notifications
                    </label>
                    <input
                      type='checkbox'
                      name='messages'
                      checked={notifications.messages}
                      onChange={handleNotificationChange}
                      className='w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500'
                    />
                  </div>
                  <div className='flex items-center justify-between'>
                    <label className='text-sm text-gray-700'>
                      Group Notifications
                    </label>
                    <input
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
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
