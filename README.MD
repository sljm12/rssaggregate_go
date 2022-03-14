# RSS Aggregator in GO

This is a rss aggregator written in GO

## Building:

go build 

## Configuration

Config is in the config.json file

Basically the config is a array of objects. Each object has the following format

```
{
    "name": "Title",
    "sites": [
        "https://site1rss",
        "https://site2rss",
        "https://site3rss"
    ],
    "outputFile": "outputfile.html"
}
```

Each title must be unique
Each site in the sites section must be a valid RSS feed

## Running

./rssaggregate

## Docker
You can build a Docker container using the included Dockerfile.
```
docker build -t <image_name>:<tag>
```

The container is built using alpine as a base and runs as a user with the name appuser.

You will need to attach a volume to the container to see the results.

```
docker run -v <src dir>:/home/appuser/data <image_name>:<tag> /home/appuser/rssaggregate
```