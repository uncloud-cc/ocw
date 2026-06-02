FROM alpine
COPY greeting.tmpl /app/
WORKDIR /app
CMD ["sh", "-c", "sed \"s/__NAME__/$NAME/g\" /app/greeting.tmpl && echo"]