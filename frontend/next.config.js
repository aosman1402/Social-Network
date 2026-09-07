// next.config.js

// Where the Next.js *server* (not the browser) reaches the backend when
// proxying relative /api/* requests. In local dev without Docker this is
// localhost; inside docker-compose it's overridden to the backend
// service's hostname, since "localhost" inside the frontend container
// would otherwise mean the frontend container itself.
const BACKEND_INTERNAL_URL =
  process.env.BACKEND_INTERNAL_URL || "http://localhost:8081";

module.exports = {
  // Produces a self-contained .next/standalone build (server + only the
  // node_modules it actually needs), which keeps the Docker runtime image
  // small and dependency-free.
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${BACKEND_INTERNAL_URL}/api/:path*`,
      },
    ];
  },
};
