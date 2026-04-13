# Open Container Workflow (ocw)

Container-native CI/CD workflows that actually run locally (and vice-versa).

## Why?

Github Actions, Gitlab piplines, CircleCI jobs - they all lack one thing: It's ~~hard~~ impossible to fully run them locally.

ocw uses the power of containers (run anywhere) to allow you to build local development workflows that run inside containers. Once you want to run these workflows in CI/CD, simply use our Github Action.

👉🏻 Checkout our [Github Actions example](./tutorials/advanced.md#use-in-github-actions) or see the Github Action README for details

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
