# Doss docs site

The documentation site for [Doss](https://github.com/Kordi-AI/doss), built with
[Fern](https://buildwithfern.com/docs). This lives on the `doss-docs` branch
only; `main` stays pure Go.

## Local

```sh
npm install
npm run check
npm run docs:dev
```

`fern docs dev` requires Node.js 22 or newer and `pnpm` on PATH. `fern check`
works as the lightweight validation step for review.

## Deploy (Fern)

Publishing uses the `FERN_TOKEN` repository secret and runs from GitHub Actions
on pushes to `doss-docs`:

```sh
npm run docs:preview
npm run docs:publish
```

Content lives in `fern/docs/pages/*.mdx`; navigation and agent-readable output
are configured in `fern/docs.yml`.
