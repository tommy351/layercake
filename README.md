# layercake

## Usage

### Run with Docker

```sh
docker run -it --rm \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $(pwd):/src \
  -w /src \
  tommy351/layercake build
```

## License

MIT
