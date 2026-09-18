"use client";

import "./globals.css";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { AuthProvider, useAuth } from "@/context/AuthContext";
import { ToastProvider } from "@/context/ToastContext";
import { WebSocketProvider } from "@/context/WebSocketContext";
import NotificationBell from "@/components/NotificationBell";

function Navbar() {
  const { user, loading, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  const handleLogout = async () => {
    const result = await logout();
    if (result.success) {
      router.push("/login");
    }
  };

  if (loading) {
    return (
      <nav className='bg-white shadow-md p-4 sticky top-0 z-50'>
        <div className='max-w-7xl mx-auto flex justify-between items-center'>
          <Link href='/' className='text-2xl font-bold text-blue-600'>
            Social Network
          </Link>
          <span className='text-gray-400 text-sm'>Loading...</span>
        </div>
      </nav>
    );
  }

  return (
    <nav className='bg-white shadow-md p-4 sticky top-0 z-50'>
      <div className='max-w-7xl mx-auto flex justify-between items-center'>
        <Link
          href='/'
          className='text-2xl font-bold text-blue-600 hover:text-blue-800 transition'>
          Social Network
        </Link>

        <div className='flex items-center space-x-4'>
          {user ? (
            // ✅ LOGGED IN
            <>
              <Link href='/' className='text-gray-600 hover:text-blue-600'>
                Home
              </Link>
              <Link
                href={`/profile/${user.id}`}
                className='text-gray-600 hover:text-blue-600'>
                Profile
              </Link>
              <Link
                href='/groups'
                className='text-gray-600 hover:text-blue-600'>
                Groups
              </Link>
              <Link href='/chat' className='text-gray-600 hover:text-blue-600'>
                Chat
              </Link>
              <NotificationBell /> {/* ✅ Added Notification Bell */}
              <Link
                href='/settings'
                className='text-gray-600 hover:text-blue-600'>
                Settings
              </Link>
              <div className='w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center text-white text-sm font-bold'>
                {user.first_name?.[0] || "U"}
              </div>
              <button
                onClick={handleLogout}
                className='text-red-600 hover:text-red-800 font-medium text-sm transition'>
                Logout
              </button>
            </>
          ) : (
            <Link href='/login' className='text-gray-600 hover:text-blue-600'>
              Login
            </Link>
          )}
        </div>
      </div>
    </nav>
  );
}

export default function RootLayout({ children }) {
  const pathname = usePathname();
  const isAuthPage = pathname === "/login" || pathname === "/register";

  return (
    <html lang='en'>
      <body>
        <ToastProvider>
          <AuthProvider>
            <WebSocketProvider>
              {!isAuthPage && <Navbar />}
              <main className={!isAuthPage ? "max-w-7xl mx-auto p-4" : ""}>
                {children}
              </main>
            </WebSocketProvider>
          </AuthProvider>
        </ToastProvider>
      </body>
    </html>
  );
}
