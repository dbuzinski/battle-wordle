# Build stage
FROM node:20-alpine as builder

WORKDIR /app

# Copy package files
COPY ui/package*.json ./

# Install dependencies
RUN npm install

# Copy source code
COPY ui/ .

# Build the app with environment variables
ARG VITE_WS_URL
ARG VITE_API_URL
ENV VITE_WS_URL=$VITE_WS_URL
ENV VITE_API_URL=$VITE_API_URL

RUN npm run build

# Production stage
FROM node:20-alpine

WORKDIR /app

# Copy built files from builder
COPY --from=builder /app/build ./build
COPY --from=builder /app/package*.json ./

# Install production dependencies only
RUN npm install --production

# Expose the port the app runs on
EXPOSE 3000

# Start the app
CMD ["node", "build"] 