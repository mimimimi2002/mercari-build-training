FROM node:20-alpine
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

RUN addgroup -S mercari && adduser -S trainee -G mercari
RUN chown -R trainee:mercari /app
USER trainee

CMD ["npm", "start", "--", "--host"]
