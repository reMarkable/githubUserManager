FROM node:20.18.1-alpine@sha256:426f843809ae05f324883afceebaa2b9cab9cb697097dbb1a2a7a41c5701de72

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install --production
COPY . .

CMD ["npm", "start"]
