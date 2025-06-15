# Build stage
FROM node:20-alpine as builder

WORKDIR /app

# Copy package files
COPY ui/package*.json ./

# Install dependencies
RUN npm install

# Copy source code
COPY ui/ .

# Build the app
RUN npm run build

# Production stage
FROM nginx:alpine

COPY --from=builder /app/build /usr/share/nginx/html
COPY ui/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"] 