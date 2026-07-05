export default {
  logo: (
    <span>
      <b>Doss</b>&nbsp;docs
    </span>
  ),
  project: { link: 'https://github.com/Kordi-AI/doss' },
  docsRepositoryBase: 'https://github.com/Kordi-AI/doss/tree/doss-docs',
  footer: {
    text: 'Doss — your agent’s memory, your rules.',
  },
  useNextSeoProps() {
    return { titleTemplate: '%s – Doss' }
  },
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta
        name="description"
        content="Doss — a synced memory folder your agents read and write as plain files, with rules for what may leave."
      />
    </>
  ),
}
