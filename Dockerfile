FROM node:24-bookworm-slim AS build
WORKDIR /app
RUN npm install --global pnpm@10.17.1
COPY . .
RUN pnpm install --frozen-lockfile
ARG PUBLIC_API_BASE_URL
ENV PUBLIC_API_BASE_URL=$PUBLIC_API_BASE_URL
RUN NODE_OPTIONS=--max-old-space-size=512 pnpm build

FROM nginxinc/nginx-unprivileged:stable-alpine AS runtime
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build /usr/share/nginx/html
USER 101:101
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
