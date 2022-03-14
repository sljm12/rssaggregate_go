FROM alpine

RUN apk add --no-cache gcompat

RUN addgroup -S appgroup && adduser -S appuser -G appgroup 

USER appuser

WORKDIR /home/appuser	

COPY rssaggregate /home/appuser

COPY *.csv /home/appuser

COPY *.json /home/appuser

COPY cities500.txt /home/appuser