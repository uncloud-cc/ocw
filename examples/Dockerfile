# Sample Dockerfile for OCW examples
# A simple nginx-based application

FROM nginx:alpine

# Copy custom HTML
COPY index.html /usr/share/nginx/html/index.html

# Expose port
EXPOSE 80

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
