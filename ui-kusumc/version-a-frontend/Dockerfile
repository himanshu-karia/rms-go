FROM node:20-alpine AS builder

WORKDIR /app

# Install pnpm
RUN npm install -g pnpm

# Copy workspace config (no root package.json in repo)
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY frontend/package.json ./frontend/

# Install dependencies for the whole workspace (dev deps included)
ENV NODE_ENV=development
RUN pnpm install --frozen-lockfile

# Copy source
COPY frontend ./frontend

# Build
WORKDIR /app/frontend
# Assuming relative API paths, so no VITE_API_URL needed at build time
RUN pnpm build

# --- Production Stage ---
FROM nginx:alpine
ENV NODE_ENV=production

# Copy built assets
COPY --from=builder /app/frontend/dist /usr/share/nginx/html

# Copy custom nginx config for SPA (Single Page App) routing
# We need to handle client-side routing (return index.html for 404s)
RUN echo 'server { \
    listen 80; \
    location / { \
        root /usr/share/nginx/html; \
        index index.html index.htm; \
        try_files $uri $uri/ /index.html; \
    } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
