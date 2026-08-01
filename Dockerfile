FROM node:24.18.1-alpine@sha256:f70403e87646dc51b45295f4b8b70cdad0b63d2297c4c9899119b03f7af7a6b3

WORKDIR /app
RUN corepack enable && echo yo
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --production --frozen-lockfile
COPY . .

CMD ["pnpm", "start"]
