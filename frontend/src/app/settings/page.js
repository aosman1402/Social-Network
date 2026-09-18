"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/context/AuthContext";
import SettingsProfileTab from "@/components/SettingsProfileTab";
import SettingsPasswordTab from "@/components/SettingsPasswordTab";
import SettingsNotificationsTab from "@/components/SettingsNotificationsTab";

export default function SettingsPage() {
  const router = useRouter();
  const { user, setUser, loading: authLoading } = useAuth();
  const [activeTab, setActiveTab] = useState("profile");
  const [isLoading, setIsLoading] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (authLoading) return;
    if (!user) router.push("/login");
  }, [user, authLoading, router]);

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

            {activeTab === "profile" && (
              <SettingsProfileTab
                user={user}
                setUser={setUser}
                isLoading={isLoading}
                setIsLoading={setIsLoading}
                setMessage={setMessage}
              />
            )}

            {activeTab === "password" && (
              <SettingsPasswordTab
                isLoading={isLoading}
                setIsLoading={setIsLoading}
                setMessage={setMessage}
                onChanged={() => router.push("/login")}
              />
            )}

            {activeTab === "notifications" && (
              <SettingsNotificationsTab
                isLoading={isLoading}
                setIsLoading={setIsLoading}
                setMessage={setMessage}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
