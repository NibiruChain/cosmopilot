import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Cosmopilot",
  description: "Cosmopilot Documentation",
  head: [
    ['link', { rel: "icon", type: "image/png", sizes: "96x96", href: "/favicon-96x96.png" }],
    ['link', { rel: "icon", type: "image/svg+xml", href: "/favicon.svg" }],
    ['link', { rel: "shortcut icon", href: "/favicon.ico" }],
    ['link', { rel: "apple-touch-icon", sizes: "180x180", href: "/apple-touch-icon.png" }],
    ['meta', { name: "apple-mobile-web-app-title", content: "Cosmopilot" }],
    ['link', { rel: "manifest", href: "/site.webmanifest" }],
  ],
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    logo: {
      src: 'logo.png',
      alt: 'Cosmopilot'
    },
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Getting Started', link: '/getting-started/01-prerequisites' }
    ],
    sidebar: [
      {
        text: 'Getting Started',
        base: '/getting-started/',
        items: [
          { text: 'Prerequisites', link: '01-prerequisites' },
          { text: 'Installation', link: '02-installation' },
          { text: 'Configuration', link: '03-configuration' }
        ]
      },
      {
        text: 'Usage',
        base: '/usage/',
        items: [
          { text: 'Deploying a Node', link: '01-deploy-node' },
        ]
      },
      {
        text: 'Reference',
        items: [
          { text: 'Custom Resource Definitions', link: '/reference/crds/crds' },
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/NibiruChain/cosmopilot' }
    ],
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024-present NIBIRU'
    }
  }
})
