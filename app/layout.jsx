import { Head } from 'nextra/components'
import { getPageMap } from 'nextra/page-map'
import { Footer, Layout, Navbar } from 'nextra-theme-docs'
import 'nextra-theme-docs/style.css'

export const metadata = {
  title: {
    default: 'Doss docs',
    template: '%s - Doss'
  },
  description:
    'Doss is a user-owned, cross-platform vault for long-term personal preferences, synced across devices with owner-controlled public disclosure.'
}

const navbar = (
  <Navbar
    logo={
      <span>
        <b>Doss</b>&nbsp;docs
      </span>
    }
    projectLink="https://github.com/Kordi-AI/doss"
  />
)

const footer = <Footer>Doss - your agent's memory, your rules.</Footer>

export default async function RootLayout({ children }) {
  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <Head
        color={{
          hue: { dark: 36, light: 28 },
          saturation: { dark: 18, light: 22 },
          lightness: { dark: 64, light: 42 }
        }}
      >
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      </Head>
      <body>
        <Layout
          navbar={navbar}
          pageMap={await getPageMap()}
          docsRepositoryBase="https://github.com/Kordi-AI/doss/tree/doss-docs"
          footer={footer}
        >
          {children}
        </Layout>
      </body>
    </html>
  )
}
