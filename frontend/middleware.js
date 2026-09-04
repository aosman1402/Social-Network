import { NextResponse } from "next/server";

/**
 * Middleware for handling authentication and route protection
 * Runs on every request before the page renders
 */
export function middleware(request) {
  // Get the session token from cookies
  const token = request.cookies.get("session_token");
  const { pathname } = request.nextUrl;

  // Define public routes (no authentication required)
  const publicRoutes = ["/login", "/register"];
  const isPublicRoute = publicRoutes.some(
    (route) => pathname === route || pathname.startsWith(route + "/"),
  );

  // Define API routes that should be excluded from middleware
  const apiRoutes = ["/api/auth", "/api/register", "/api/login"];
  const isApiRoute = apiRoutes.some((route) => pathname.startsWith(route));

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
  const isProtectedRoute = protectedRoutes.some(
    (route) => pathname === route || pathname.startsWith(route + "/"),
  );

  // Handle authentication redirects
  // If user is authenticated and tries to access login/register, redirect to home
  if (isPublicRoute && token) {
    return NextResponse.redirect(new URL("/", request.url));
  }

  // If user is not authenticated and tries to access protected route, redirect to login
  if (isProtectedRoute && !token) {
    const loginUrl = new URL("/login", request.url);
    // Add the original URL as a redirect parameter
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  // Add the user token to request headers for API routes
  if (token && isApiRoute) {
    const requestHeaders = new Headers(request.headers);
    requestHeaders.set("Authorization", `Bearer ${token.value}`);

    return NextResponse.next({
      request: {
        headers: requestHeaders,
      },
    });
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
