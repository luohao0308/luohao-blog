# Nuxt 生产镜像：构建期装全量依赖跑 pnpm build，运行期只带 .output。
FROM node:24-alpine AS build
WORKDIR /app
RUN npm install -g pnpm@11
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM node:24-alpine
WORKDIR /app
COPY --from=build /app/.output ./.output
ENV NODE_ENV=production
# backendBase 由 compose 注入（compose 网络内的 backend 服务名）。
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
