FROM node:22.16.0-alpine@sha256:41e4389f3d988d2ed55392df4db1420ad048ae53324a8e2b7c6d19508288107e

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install --production
COPY . .

CMD ["npm", "start"]
