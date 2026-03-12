FROM nginx:alpine

# Demonstrate using RUN with /workflow mount
# This approach accesses files via the /workflow mount instead of COPY

# Copy website.html using RUN and the /workflow mount
RUN cp /workflow/Dockerfiles/website.html /usr/share/nginx/html/index.html

# We can also access other files from the workflow root
RUN echo "Build completed at $(date)" >> /usr/share/nginx/html/build-info.txt
RUN echo "Workflow files available:" >> /usr/share/nginx/html/build-info.txt
RUN ls -la /workflow/ >> /usr/share/nginx/html/build-info.txt
