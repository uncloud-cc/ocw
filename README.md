# Open Container Workflow (ocw)

Container-native workflows for local development & CI/CD.

## Example
```javascript
// server.js
require('http').createServer((_, res) => {
  res.end(`Hello world!`);
}).listen(8888);
```

```yaml
# hello.yaml
name: Hello World
sequence:
  - id: server
    image: node:25-alpine
    cmd: node server.js

    # Make it a hot-reloading dev server
    background: true
    watch: true
    expose: 8888
```

Run the workflow with `ocw hello.yaml` and play around with `server.js` to see hot-reloading in action ✨

## Install
```bash
go install github.com/uncloud-cc/ocw/cmd/ocw@latest
```

Check if it worked by running `ocw --help`

## Getting started

- Go through the [basics tutorial](./tutorials/basics.md)
- Check out advanced features in the [advanced tutorial](./tutorials/advanced.md)
- Or check out the [examples](./examples/)

## Feedback

[Join the community](https://github.com/uncloud-cc/ocw/discussions) to ask questions, get help and share feedback.

## License

MIT License - see [LICENSE](LICENSE) file.
