FROM node:24.22.2-alpine3.24 AS base
WORKDIR /app
RUN chown node:node /app
USER node

COPY --chown=node:node package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY --chown=node:node . .

FROM base AS development
EXPOSE 5173
CMD ["npm", "run", "dev"]

FROM base AS build
RUN npm run build

FROM nginx/nginx-unprivileged:1.30.6-alpine AS production
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080

