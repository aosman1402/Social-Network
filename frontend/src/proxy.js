import { NextResponse } from "next/server";

/**
 * Proxy (formerly "middleware") for handling authentication and route protection
 * Runs on every request before the page renders
 */
export function proxy(request) {
  // Get the session token from cookies
  const token = request.cookies.get("session_token");
  const { pathname } = request.nextUrl;

  // Define protected routes (authentication required)
  const protectedRoutes = [
    "/profile",
    "/groups",
    "/chat",
    "/notifications",
    "/settings",
    "/create",
    "/search",
  ];
  // The home feed ("/") is protected too, but matched exactly - a prefix
  // match on "/" would cover every route.
  const isProtectedRoute =
    pathname === "/" ||
    protectedRoutes.some(
      (route) => pathname === route || pathname.startsWith(route + "/"),
    );

  // No "already logged in -> leave /login" redirect here: this only sees
  // that a cookie exists, not whether its session is still valid, so an
  // expired cookie would bounce between /login and / forever.

  // If user is not authenticated and tries to access protected route, redirect to login
  if (isProtectedRoute && !token) {
    const loginUrl = new URL("/login", request.url);
    // Add the original URL as a redirect parameter
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  // Allow the request to proceed
  return NextResponse.next();
}

/**
 * Configuration for which paths the middleware should run on
 * Matches all routes except static files, api routes, and Next.js internal paths
 */
export const config = {
  matcher: [
    /*
     * Match all request paths except:
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - public folder
     * - api routes (handled separately)
     */
    "/((?!api|_next/static|_next/image|favicon.ico|public/|.*\\.(?:jpg|jpeg|gif|png|svg|ico|webp)$).*)",
  ],
};
