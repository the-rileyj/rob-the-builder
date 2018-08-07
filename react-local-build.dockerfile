FROM node:8.11.3-alpine

WORKDIR /app

COPY tslint.json .
COPY tsconfig.test.json .
COPY tsconfig.prod.json .
COPY tsconfig.json .
COPY package-lock.json .
COPY images.d.ts .
COPY package.json .

RUN npm install
ADD ./src ./src
ADD ./public ./public

ENTRYPOINT ["npm", "run", "build"]
