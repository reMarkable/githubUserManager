FROM node:20.18.1-alpine@sha256:b5b9467fe7b33aad47f1ec3f6e0646a658f85f05c18d4243024212a91f3b7554

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install --production
COPY . .

CMD ["npm", "start"]
