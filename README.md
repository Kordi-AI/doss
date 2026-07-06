# Doss docs site

The documentation site for [Doss](https://github.com/Kordi-AI/doss), built with
[Nextra](https://nextra.site). This lives on the `doss-docs` branch only — `main`
stays pure Go.

## Local

```sh
npm install
npm run dev     # http://localhost:3000
npm run build   # static export → ./out
```

## Deploy (Vercel)

Import the repo on Vercel and set the **Production Branch** to `doss-docs`
(Settings → Git). Framework preset: Next.js. It auto-builds on every push to
this branch.

Content lives in `content/*.mdx`; the sidebar order is `content/_meta.js`.
Nextra 4 uses the App Router shell under `app/`.
