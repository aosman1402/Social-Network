"use client";

import Link from "next/link";
import { useAuth } from "@/context/AuthContext";
import { usePathname } from "next/navigation";
import {
  FaHome,
  FaUser,
  FaUsers,
  FaComment,
  FaBell,
  FaCog,
} from "react-icons/fa";

export default function Sidebar() {
  const { user } = useAuth();
  const pathname = usePathname();

  if (!user) return null;

  const isActive = (path) => {
    return pathname === path || pathname?.startsWith(path + "/")
      ? "bg-blue-50 text-blue-600 border-r-4 border-blue-600"
      : "hover:bg-gray-100 text-gray-700";
  };

  const navItems = [
    { path: "/", icon: FaHome, label: "Home" },
    { path: `/profile/${user?.id}`, icon: FaUser, label: "Profile" },
    { path: "/groups", icon: FaUsers, label: "Groups" },
    { path: "/chat", icon: FaComment, label: "Messages" },
    { path: "/notifications", icon: FaBell, label: "Notifications" },
    { path: "/settings", icon: FaCog, label: "Settings" },
  ];

  return (
    <aside className='w-64 bg-white shadow-md h-screen fixed left-0 top-16 overflow-y-auto'>
      <div className='p-4'>
        <div className='flex items-center space-x-3 mb-6'>
          <div className='w-10 h-10 rounded-full bg-blue-500 flex items-center justify-center text-white font-bold'>
            {user?.first_name?.[0]}
            {user?.last_name?.[0]}
          </div>
          <div>
            <p className='font-semibold'>
              {user?.first_name} {user?.last_name}
            </p>
            <p className='text-sm text-gray-500'>
              @{user?.nickname || user?.first_name}
            </p>
          </div>
        </div>

        <nav className='space-y-2'>
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <Link
                key={item.path}
                href={item.path}
                className={`flex items-center space-x-3 px-3 py-2 rounded-lg transition ${isActive(item.path)}`}>
                <Icon size={18} />
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>
      </div>
    </aside>
  );
}
