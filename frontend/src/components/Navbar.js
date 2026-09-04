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
  FaSignOutAlt,
} from "react-icons/fa";
import toast from "react-hot-toast";

export default function Navbar() {
  const { user, logout } = useAuth();
  const pathname = usePathname();

  const handleLogout = async () => {
    try {
      await logout();
      toast.success("Logged out successfully");
    } catch (error) {
      toast.error("Logout failed");
    }
  };

  if (!user) return null;

  const isActive = (path) => {
    return pathname === path
      ? "text-blue-600"
      : "text-gray-600 hover:text-blue-600";
  };

  return (
    <nav className='bg-white shadow-md fixed top-0 left-0 right-0 z-50 h-16'>
      <div className='max-w-7xl mx-auto px-4 h-full flex items-center justify-between'>
        <Link href='/' className='text-2xl font-bold text-blue-600'>
          Social
        </Link>

        <div className='flex items-center space-x-4'>
          <Link href='/' className={isActive("/")}>
            <FaHome size={20} />
          </Link>
          <Link href={`/profile/${user?.id}`} className={isActive("/profile")}>
            <FaUser size={20} />
          </Link>
          <Link href='/groups' className={isActive("/groups")}>
            <FaUsers size={20} />
          </Link>
          <Link href='/chat' className={isActive("/chat")}>
            <FaComment size={20} />
          </Link>
          <Link href='/notifications' className={isActive("/notifications")}>
            <FaBell size={20} />
          </Link>
          <button
            onClick={handleLogout}
            className='text-gray-600 hover:text-red-600'>
            <FaSignOutAlt size={20} />
          </button>
        </div>
      </div>
    </nav>
  );
}
