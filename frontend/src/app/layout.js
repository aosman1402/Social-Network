"use client";

import "./globals.css";
import { usePathname } from "next/navigation";
import { AuthProvider } from "@/context/AuthContext";
import { ToastProvider } from "@/context/ToastContext";
import { WebSocketProvider } from "@/context/WebSocketContext";
import Navbar from "@/components/Navbar";

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
