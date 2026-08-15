FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json ./
COPY .npmrc ./
RUN npm install

COPY . .
RUN npm run generate

FROM nginx:alpine

COPY --from=builder /app/.output/public /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
