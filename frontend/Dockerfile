# --- Dependencies stage ---------------------------------------------------
FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# --- Build stage -----------------------------------------------------------
FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# next.config.js's rewrites() runs during `next build` and its result is
# frozen into .next/routes-manifest.json — setting this env var at
# container *runtime* is too late, it must be a build arg.
ARG BACKEND_INTERNAL_URL=http://localhost:8081
ENV BACKEND_INTERNAL_URL=$BACKEND_INTERNAL_URL
ENV NEXT_TELEMETRY_DISABLED=1
ENV NEXT_OUTPUT=standalone
RUN npm run build

# --- Runtime stage -----------------------------------------------------------
FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# .next/standalone already contains a minimal node_modules and a
# server.js entrypoint (see next.config.js `output: "standalone"`).
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
# Static files served as-is (e.g. the login page video) - the standalone
# build doesn't include public/, so it has to be copied in separately.
COPY --from=builder /app/public ./public

USER appuser

EXPOSE 3000
CMD ["node", "server.js"]
